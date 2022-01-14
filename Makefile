SHELL := /usr/bin/env bash

# Verbosity
AT_ = @
AT = $(AT_$(V))
# /Verbosity

GIT_HASH := $(shell git rev-parse --short=7 HEAD)
IMAGETAG ?= ${GIT_HASH}

BASE_IMG ?= ocm-agent
IMAGE_REGISTRY ?= quay.io
IMAGE_REPOSITORY ?= app-sre
IMG ?= $(IMAGE_REGISTRY)/$(IMAGE_REPOSITORY)/${BASE_IMG}

BINARY_FILE ?= build/_output/ocm-agent

GO_SOURCES := $(find $(CURDIR) -type f -name "*.go" -print)
EXTRA_DEPS := $(find $(CURDIR)/build -type f -print) Makefile

# Containers may default GOFLAGS=-mod=vendor which would break us since
# we're using modules.
unexport GOFLAGS
GOOS?=linux
GOARCH?=amd64
GOENV=GOOS=${GOOS} GOARCH=${GOARCH} CGO_ENABLED=0 GOFLAGS=

GOBUILDFLAGS=-gcflags="all=-trimpath=${GOPATH}" -asmflags="all=-trimpath=${GOPATH}"

CONTAINER_ENGINE ?= $(shell command -v podman 2>/dev/null || command -v docker 2>/dev/null)
SRC_CONTAINER_TRANSPORT ?= $(if $(findstring podman,$(CONTAINER_ENGINE)),container-storage,docker-daemon)

#eg, -v
TESTOPTS ?=

DOC_BINARY := hack/documentation/document.go
# ex -hideRules
DOCFLAGS ?=

default: all

all: test build-image

.PHONY: test
go-test: vet $(GO_SOURCES)
	$(AT)go test $(TESTOPTS) $(shell go list -mod=readonly -e ./...)

.PHONY: clean
clean:
	$(AT)rm -f $(BINARY_FILE) coverage.txt

.PHONY: serve
serve:
	$(AT)go run ./cmd/main.go -port 8888

.PHONY: vet
vet:
	$(AT)gofmt -s -l $(shell go list -f '{{ .Dir }}' ./... ) | grep ".*\.go"; if [ "$$?" = "0" ]; then gofmt -s -d $(shell go list -f '{{ .Dir }}' ./... ); exit 1; fi
	$(AT)go vet ./cmd/...

.PHONY: build
build: $(BINARY_FILE)

$(BINARY_FILE): test $(GO_SOURCES)
	mkdir -p $(shell dirname $(BINARY_FILE))
	$(GOENV) go build $(GOBUILDFLAGS) -o $(BINARY_FILE) ./cmd/ocm-agent

.PHONY: build-base
build-base: build-image
.PHONY: build-image
build-image: clean $(GO_SOURCES) $(EXTRA_DEPS)
	$(CONTAINER_ENGINE) build -t $(IMG):$(IMAGETAG) -f $(join $(CURDIR),/build/Dockerfile) . && \
	$(CONTAINER_ENGINE) tag $(IMG):$(IMAGETAG) $(IMG):latest

.PHONY: build-push
build-push:
	build/build_push.sh $(IMG):$(IMAGETAG)

### Imported
.PHONY: skopeo-push
skopeo-push:
	@if [[ -z $$QUAY_USER || -z $$QUAY_TOKEN ]]; then \
		echo "You must set QUAY_USER and QUAY_TOKEN environment variables" ;\
		echo "ex: make QUAY_USER=value QUAY_TOKEN=value $@" ;\
		exit 1 ;\
	fi
	# QUAY_USER and QUAY_TOKEN are supplied as env vars
	skopeo copy --dest-creds "${QUAY_USER}:${QUAY_TOKEN}" \
		"${SRC_CONTAINER_TRANSPORT}:${IMG}:${IMAGETAG}" \
		"docker://${IMG}:latest"
	skopeo copy --dest-creds "${QUAY_USER}:${QUAY_TOKEN}" \
		"${SRC_CONTAINER_TRANSPORT}:${IMG}:${IMAGETAG}" \
		"docker://${IMG}:${IMAGETAG}"


.PHONY: push-base
push-base: build/Dockerfile
	$(CONTAINER_ENGINE) push $(IMG):$(IMAGETAG)
	$(CONTAINER_ENGINE) push $(IMG):latest

.PHONY: test
test: go-test

.PHONY: coverage
coverage:
	hack/codecov.sh

.PHONY: docs
docs:
	@# Ensure that the output from the test is hidden so this can be
	@# make docs > docs.json
	@# To hide the rules: make DOCFLAGS=-hideRules docs
	@$(MAKE test)
	@go run $(DOC_BINARY) $(DOCFLAGS)
