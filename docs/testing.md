# OCM Agent Operator Testing

Tests are playing a primary role and we take them seriously.
It is expected from PRs to add, modify or delete tests on case by case scenario.
To contribute you need to be familiar with:

* [Ginkgo](https://github.com/onsi/ginkgo) - BDD Testing Framework for Go
* [Gomega](https://onsi.github.io/gomega/) - Matcher/assertion library

## Prerequisites

Make sure that all the necessary dependencies are already in place. The `ginkgo` and `mockgen` binaries that are required for testing will be installed as part of tool dependencies.

## Bootstrapping the tests

```shell
$ cd pkg/samplepackage
$ ginkgo bootstrap
$ ginkgo generate samplepackage.go

find .
./samplepackage.go
./samplepackage_suite_test.go
./samplepackage_test.go
```

## How to run the tests

* You can run the tests using `make test` or `go test ./...`
