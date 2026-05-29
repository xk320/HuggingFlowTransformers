VERSION ?= 1.7.5-beta
COMMIT ?= local
BUILD_TIME ?= $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
BINARY := HuggingFlowTransformers
GATEWAY_BINARY := hft-gateway
IMAGE ?= huggingflowtransformers-runtime
TARGET_OS := linux
TARGET_ARCH := amd64
TARGET_LABEL := x86_64

.PHONY: test prepare build package docker clean

test:
	go test ./...

prepare:
	./scripts/prepare-engine.sh

build: prepare
	mkdir -p dist
	rm -f dist/$(BINARY)-$(TARGET_OS)-*-v$(VERSION) dist/$(BINARY)-$(TARGET_OS)-*-v$(VERSION).tar.gz dist/$(GATEWAY_BINARY)-$(TARGET_OS)-*-v$(VERSION)
	CGO_ENABLED=0 GOOS=$(TARGET_OS) GOARCH=$(TARGET_ARCH) go build \
		-ldflags "-s -w -X main.hftVersion=$(VERSION) -X main.buildCommit=$(COMMIT) -X main.buildTime=$(BUILD_TIME)" \
		-o dist/$(BINARY)-$(TARGET_OS)-$(TARGET_LABEL)-v$(VERSION) \
		./cmd/HuggingFlowTransformers
	CGO_ENABLED=0 GOOS=$(TARGET_OS) GOARCH=$(TARGET_ARCH) go build \
		-ldflags "-s -w -X main.gatewayVersion=$(VERSION)" \
		-o dist/$(GATEWAY_BINARY)-$(TARGET_OS)-$(TARGET_LABEL)-v$(VERSION) \
		./cmd/hft-gateway

package: build
	cd dist && tar -czf $(BINARY)-$(TARGET_OS)-$(TARGET_LABEL)-v$(VERSION).tar.gz $(BINARY)-$(TARGET_OS)-$(TARGET_LABEL)-v$(VERSION) $(GATEWAY_BINARY)-$(TARGET_OS)-$(TARGET_LABEL)-v$(VERSION)

docker: build
	docker build \
		--build-arg VERSION=$(VERSION) \
		-t $(IMAGE):$(VERSION) \
		-t $(IMAGE):latest \
		.

clean:
	rm -rf dist
