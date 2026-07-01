export KONFLUX_BUILDS=true
SHELL := /usr/bin/env bash

IMAGE_NAME=ocm-agent

include boilerplate/generated-includes.mk

# Verbosity
AT_ = @
AT = $(AT_$(V))
# /Verbosity

BINARY_FILE ?= build/_output/ocm-agent

GO_SOURCES := $(find $(CURDIR) -type f -name "*.go" -print)
EXTRA_DEPS := $(find $(CURDIR)/build -type f -print) Makefile

# Containers may default GOFLAGS=-mod=vendor which would break us since
# we're using modules.
unexport GOFLAGS
GOOS?=linux
GOARCH?=amd64
GOENV=GOOS=${GOOS} GOARCH=${GOARCH} CGO_ENABLED=1 GOEXPERIMENT=boringcrypto GOFLAGS=

GOBUILDFLAGS=-gcflags="all=-trimpath=${GOPATH}" -asmflags="all=-trimpath=${GOPATH}" -tags="fips_enabled"

#eg, -v
TESTOPTS ?=

DOC_BINARY := hack/documentation/document.go
# ex -hideRules
DOCFLAGS ?=

all: test osd-container-image-build

.PHONY: test
go-test: vet $(GO_SOURCES)
	$(AT)go test $(TESTOPTS) $(shell go list -mod=readonly -e ./...)

.PHONY: clean
clean:
	$(AT)rm -f $(BINARY_FILE) coverage.txt

.PHONY: serve
serve:
	$(AT)go run ./cmd/ocm-agent/main.go serve --access-token "$TOKEN" --services "$SERVICE" --ocm-url "https://sample.example.com"

.PHONY: vet
vet:
	$(AT)gofmt -s -l $(shell go list -f '{{ .Dir }}' ./... ) | grep ".*\.go"; if [ "$$?" = "0" ]; then gofmt -s -d $(shell go list -f '{{ .Dir }}' ./... ); exit 1; fi
	$(AT)go vet ./cmd/...

.PHONY: build
build: $(BINARY_FILE)

$(BINARY_FILE): test $(GO_SOURCES)
	mkdir -p $(shell dirname $(BINARY_FILE))
	$(GOENV) go build $(GOBUILDFLAGS) -o $(BINARY_FILE) ./cmd/ocm-agent

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

# Installed using instructions from: https://golangci-lint.run/usage/install/#linux-and-windows
getlint:
	@mkdir -p $(GOPATH)/bin
	@ls $(GOPATH)/bin/golangci-lint 1>/dev/null || (echo "Installing golangci-lint..." && curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(GOPATH)/bin v1.64.2)

.PHONY: lint
lint: getlint
	@mkdir -p /tmp/go-cache
	@export GOCACHE=/tmp/go-cache && $(GOPATH)/bin/golangci-lint run --timeout=5m

mockgen: ensure-mockgen
	go generate $(GOBUILDFLAGS) ./...

ensure-mockgen:
	go install go.uber.org/mock/mockgen@v0.6.0

.PHONY: boilerplate-update
boilerplate-update: ## Update boilerplate version
	@boilerplate/update
