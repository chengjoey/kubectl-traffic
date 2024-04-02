# kubectl-traffic: kubernetes pod http traffic monitoring

kubectl-traffic is a non-intrusive kubernetes pod http traffic monitoring tool. It can monitor the pod's http requests and responses, sort the number of http requests, and calculate the average time consumption. 
open 5557 port to collect http prometheus metrics.

## Usage
```shell
## download latest release
wget https://github.com/chengjoey/kubectl-traffic/releases
#or somewhere on $PATH and give it executable permissions.
mv kubectl-traffic /usr/local/bin/
```
or
```shell
make build-traffic
#or somewhere on $PATH and give it executable permissions.
mv kubectl-traffic /usr/local/bin/
```
inject the ebpf ebpemeral container into the target namespace and pod:
```shell
kubectl traffic --ns=${namespace} --pod=${pod}
```
then you could see the http log in the pod's log:
```shell
kubecl logs ${pod} -n ${namespace} -c ebpf-agent -f
```
or get the http metrics:
```shell
curl http://pod_ip:5557/metrics
```

## Commands
Flag | Description
--- | ---
`-h, --help` | Display help and usage
`--ns` | The target namespace
`--pod` | The target pod
`--image` | The ebpf container image

## Build ebpf image
If you want to compile the ebpf image of the ebpemeral container yourself, you can use the following command:
```shell
make image REGISTRY=${your_registry}
```
then modify the image flag of kubectl-traffic to your registry address.

## Support version
linux/amd64, kernel version >= 4.9, kubernetes version >= 1.25