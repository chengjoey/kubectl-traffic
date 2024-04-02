ARCH := amd64
OS := linux
GOARCH := $(ARCH)
PROGRAM = ebpf-agent
KERNEL_VERSION ?= 5.15.0-87-generic
EBPF_DEVEL_VERSION ?= v0.2
PROJ_PATH := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
CGO_EXTLDFLAGS_STATIC = '-w -extldflags "-static"'
GOPROXY ?= https://goproxy.cn,direct
REGISTRY ?= registry.cn-hongkong.aliyuncs.com/joeycheng/library
IMAGE_TAG = $(shell date '+%Y%m%d%H%M%S')

build-ebpf: clean
	docker run --rm \
    	-v ${PROJ_PATH}:/build \
    	-e KERNEL_VERSION=${KERNEL_VERSION} \
    	registry.erda.cloud/erda/ebpf-devel:${EBPF_DEVEL_VERSION} \
    	sh -c " \
    		cd build && \
    		bash -x build/compile_ebpf.sh \
    	"

build-ebpf-agent: build-ebpf
	CC=$(CLANG) \
		CGO_ENABLED=0 \
		CGO_CFLAGS=$(CGO_CFLAGS_STATIC) \
		GOPROXY=$(GOPROXY) \
		CGO_LDFLAGS=$(CGO_LDFLAGS_STATIC) \
                GOARCH=$(GOARCH) \
                GOOS=$(OS) \
                go build \
                -tags netgo -ldflags $(CGO_EXTLDFLAGS_STATIC) \
                -o $(PROGRAM) ./cmd/agent/*.go

image: build-ebpf-agent
	docker buildx build --platform=linux/amd64 --build-arg "KERNEL_VERSION=${KERNEL_VERSION}" -t $(REGISTRY):ebpf-$(IMAGE_TAG) \
 		-f build/agent/Dockerfile . --push

build-traffic:
	GOARCH=$(GOARCH) \
		GOOS=$(OS) \
		GOPROXY=$(GOPROXY) \
		go build -o kubectl-traffic ./cmd/traffic/*.go && chmod +x kubectl-traffic

clean:
	rm -rf ebpf-agent
	rm -rf target