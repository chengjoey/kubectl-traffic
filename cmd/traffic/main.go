package main

import (
	"context"
	"fmt"
	"k8s.io/klog/v2"
	"os"

	"github.com/alecthomas/kingpin/v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	flagPod     = kingpin.Flag("pod", "pod").Short('p').PlaceHolder("NAME").String()
	flagNs      = kingpin.Flag("ns", "namespace").Short('n').PlaceHolder("NAME").Default("default").String()
	flagContext = kingpin.Flag("context", "kubernetes context").PlaceHolder("CONTEXT-NAME").String()
	flagImage   = kingpin.Flag("image", "ebpf agent image").PlaceHolder("IMAGE").Default("registry.cn-hongkong.aliyuncs.com/joeycheng/library:ebpf-20240402172538").String()
)

func main() {
	if os.Args[len(os.Args)-1] == "" {
		os.Args = os.Args[0 : len(os.Args)-1]
	}

	kingpin.Command("run", "Run ebpf agent").Default()
	kingpin.Command("version", "Display current version")
	kingpin.CommandLine.HelpFlag.Short('h')
	kingpin.CommandLine.Help = "Add an Ephemera Container to the corresponding pod to view http traffic information"
	kingpin.CommandLine.DefaultEnvars()
	cmd := kingpin.Parse()

	if cmd == "version" {
		fmt.Printf("main\n")
		return
	}

	if flagPod == nil || *flagPod == "" {
		kingpin.Fatalf("Please specify target pod")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cs := createKubeClient(ctx)
	targetPod, err := cs.CoreV1().Pods(*flagNs).Get(ctx, *flagPod, metav1.GetOptions{})
	if err != nil {
		kingpin.Fatalf("Error getting pod: %v", err)
	}
	targetPod.Spec.EphemeralContainers = append(targetPod.Spec.EphemeralContainers, makeEphemeralContainer(*flagImage))
	if _, err := cs.CoreV1().Pods(*flagNs).UpdateEphemeralContainers(ctx, targetPod.Name, targetPod, metav1.UpdateOptions{}); err != nil {
		kingpin.Fatalf("Error updating pod: %v", err)
	}

	klog.Infof("Ephemeral container added to pod %s/%s", *flagNs, targetPod.Name)
	klog.Infof("Use kubectl logs %s -n %s -c ebpf-agent to view http traffic logs", targetPod.Name, *flagNs)
	klog.Infof("Use curl http://%s:5557/metrics to view prometheus metrics", targetPod.Status.PodIP)
}

func makeEphemeralContainer(image string) corev1.EphemeralContainer {
	return corev1.EphemeralContainer{
		EphemeralContainerCommon: corev1.EphemeralContainerCommon{
			Name:  "ebpf-agent",
			Image: image,
			SecurityContext: &corev1.SecurityContext{
				Privileged: &[]bool{true}[0],
			},
			Env: []corev1.EnvVar{
				{
					Name: "POD_IP",
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							APIVersion: "v1",
							FieldPath:  "status.podIP",
						},
					},
				},
			},
		},
	}
}

func createKubeClient(ctx context.Context) kubernetes.Interface {

	config, err := rest.InClusterConfig()
	switch {
	case err == nil:
		cs, err := kubernetes.NewForConfig(config)
		kingpin.FatalIfError(err, "Error configuring kubernetes connection")
		return cs
	case config != nil:
		kingpin.Fatalf("Error configuring in-cluster config: %v", err)
	}

	overrides := &clientcmd.ConfigOverrides{}
	if flagContext != nil {
		overrides.CurrentContext = *flagContext
	}

	cc := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(), overrides)

	rc, err := cc.ClientConfig()
	kingpin.FatalIfError(err, "Error determining client config")

	cs, err := kubernetes.NewForConfig(rc)
	kingpin.FatalIfError(err, "Error building kubernetes config")

	_, err = cs.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil && !apierrors.IsForbidden(err) {
		kingpin.FatalIfError(err, "Can't connnect to kubernetes")
	}

	return cs
}
