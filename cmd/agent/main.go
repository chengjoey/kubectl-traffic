package main

import (
	"context"
	"fmt"
	"github.com/chengjoey/kubectl-traffic/pkg/traffic/http"
	"github.com/chengjoey/kubectl-traffic/pkg/trie"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
	nethttp "net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func watchSignals(ctx context.Context, cancel context.CancelFunc) <-chan struct{} {
	donech := make(chan struct{})
	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch, syscall.SIGINT, syscall.SIGHUP)
	go func() {
		defer close(donech)
		defer signal.Stop(sigch)
		select {
		case <-ctx.Done():
		case <-sigch:
			cancel()
		}
	}()
	return donech
}

func getSelfLink() (index int, ip string, err error) {
	ip = os.Getenv("POD_IP")
	if ip == "" {
		return 0, "", fmt.Errorf("env POD_IP is empty")

	}
	netLinks, err := netlink.LinkList()
	if err != nil {
		return 0, "", err
	}
	for _, link := range netLinks {
		if link.Type() == "veth" {
			attrs := link.Attrs()
			return attrs.Index, ip, nil
		}
	}
	return 0, "", fmt.Errorf("veth not found")
}

func main() {
	logger := logrus.New()

	// ebpf agent
	ifIndex, ip, err := getSelfLink()
	if err != nil {
		logger.Fatalf("failed to get self link: %v", err)
	}
	ch := make(chan *http.HttpPackage, 100)
	agent := http.New(*logger, ifIndex, ip, ch)
	if err := agent.Load(); err != nil {
		logger.Fatalf("failed to load agent: %v", err)
	}

	// trie
	t := trie.NewTrie()

	// metrics
	registry := prometheus.NewRegistry()
	registry.MustRegister(t)
	nethttp.HandleFunc("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}).ServeHTTP)
	go func() {
		logger.Fatal(nethttp.ListenAndServe(":5557", nil))
	}()

	ctx, cancel := context.WithCancel(context.Background())
	doneCh := watchSignals(ctx, cancel)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case p := <-ch:
			t.Insert(p)
		case <-ticker.C:
			fmt.Printf("TOP10: \n%s", t.OutputTopN(10))
		case <-doneCh:
			agent.Close()
			cancel()
			break
		}
	}
}
