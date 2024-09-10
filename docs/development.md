# # OCM Agent Development

## Development Environment Setup

### golang

A recent Go distribution (>=1.22) with enabled Go modules.

```shell
$ go version
go version go1.22.7 linux/amd64
```

## Makefile

All available standardized commands for the `Makefile` are available via:

```shell
$ make
Usage: make <OPTIONS> ... <TARGETS>

Available targets are:

build                            Build binary
go-test                          runs go test across operator
serve                            Start OCM Agent Server
build-image                      Build image locally
getlint                          Get lint package
```

## Dependencies

The module dependencies that are required locally to be present are all part of [go.mod](https://github.com/openshift/ocm-agent/blob/master/go.mod) file.

---

### NOTE

If any of the dependencies are failing to install due to checksum mismatch, try setting `GOPROXY` env variable using `export GOPROXY="https://proxy.golang.org"`.

---

## Linting

To run lint locally, call `make lint`

```shell
make lint
```

## Testing

To run unit tests locally, call `make test`

```shell
make go-test
```

## Building

To run build locally, call `make build`

```shell
make build
```

To build image, call `make build-image`

```shell
make build-image 
```

## Run locally

To run OCM Agent locally, call `make serve`

```shell
make serve
```
