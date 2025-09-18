# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Development Commands

### Build
- `make build` - Build the OCM agent binary to `build/_output/ocm-agent`
- `make build-image` - Build container image with tag based on git hash
- `make clean` - Remove build artifacts

### Testing
- `make test` - Run all Go tests (alias for `make go-test`)
- `make go-test` - Run Go unit tests with vet checks
- `make coverage` - Generate test coverage report using `hack/codecov.sh`

### Linting and Code Quality
- `make lint` - Run golangci-lint (installs automatically if not present)
- `make vet` - Run gofmt and go vet checks
- `make mockgen` - Generate mocks for testing (installs mockgen if needed)

### Running Locally
- `make serve` - Run OCM agent locally with sample configuration
  - Requires `$TOKEN`, `$SERVICE` environment variables
  - Uses sample OCM URL for testing

### Container Operations
- `make build-push` - Build and push container image
- `make skopeo-push` - Push image using skopeo (requires QUAY_USER and QUAY_TOKEN)

## Project Architecture

### Core Components

**Main Entry Point**: `cmd/ocm-agent/main.go` - Simple CLI wrapper around the root command

**CLI Framework**: `pkg/cli/`
- Uses Cobra for command structure
- Main command is `serve` which starts the HTTP server
- Configuration handled via Viper with support for config files, env vars, and flags

**HTTP Server**: `pkg/cli/serve/serve.go`
- Gorilla Mux router for handling webhook endpoints
- Supports both traditional OSD/ROSA mode and Fleet (HyperShift) mode
- Prometheus metrics endpoint at `/metrics`

**Request Handlers**: `pkg/handlers/`
- `webhookreceiver.go` - Main AlertManager webhook receiver
- `webhookrhobsreceiver.go` - RHOBS (Red Hat Observability Service) webhook receiver
- `cluster.go` - Cluster management operations
- `upgrade_policies.go` - Cluster upgrade policy management
- `readyz.go` / `livez.go` - Health check endpoints

**OCM Integration**: `pkg/ocm/`
- Wrapper around OCM SDK for API communication
- Handles authentication (token-based or client credentials)
- Connection management and retry logic

**Kubernetes Client**: `pkg/k8s/`
- Kubernetes client-go wrapper for cluster operations
- Used for reading cluster metadata and applying configurations

### Key Packages

- `pkg/config/` - Configuration management using Viper
- `pkg/metrics/` - Prometheus metrics collection
- `pkg/logging/` - Structured logging with logrus
- `pkg/httpchecker/` - HTTP health checking utilities with retry logic
- `pkg/consts/` - Application constants

### Operational Modes

**Traditional Mode**: Runs on OSD/ROSA clusters to forward alerts to OCM Service Log
**Fleet Mode**: Runs in HyperShift management clusters to handle fleet-wide notifications

### Testing Structure

- Unit tests alongside source files (`*_test.go`)
- Ginkgo/Gomega BDD testing framework
- Mock generation using `golang/mock`
- E2E tests in `test/e2e/` directory
- Test utilities and scripts in `test/` directory

### Container Build

Uses multi-stage Dockerfile in `build/Dockerfile` with FIPS-enabled Go build for security compliance.

## Development Notes

This is a Kubernetes-native Go service that acts as a bridge between cluster monitoring (AlertManager/RHOBS) and OpenShift Cluster Manager services. The agent receives webhook notifications about cluster events and forwards them to appropriate OCM services for SRE visibility and automation.