<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [OCM Agent Operator Testing](#ocm-agent-operator-testing)
  - [Unit Tests](#unit-tests)
    - [Prerequisites](#prerequisites)
    - [Bootstrapping the tests](#bootstrapping-the-tests)
    - [How to run the tests](#how-to-run-the-tests)
  - [Functional Tests](#functional-tests)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# OCM Agent Operator Testing

## Unit Tests

Tests are playing a primary role and we take them seriously.
It is expected from PRs to add, modify or delete tests on case by case scenario.
To contribute you need to be familiar with:

* [Ginkgo](https://github.com/onsi/ginkgo) - BDD Testing Framework for Go
* [Gomega](https://onsi.github.io/gomega/) - Matcher/assertion library

### Prerequisites

Make sure that all the necessary dependencies are already in place. The `ginkgo` and `mockgen` binaries that are required for testing will be installed as part of tool dependencies. Run `make ensure-mockgen` to have the `mockgen` binary locally.

```bash
# Installing ginkgo v2 binary locally
go install github.com/onsi/ginkgo/v2/ginkgo

# Ensuring mockgen binary locally
make ensure-mockgen
```

### Bootstrapping the tests

If there are new package(s) added to the repository then the Ginkgo test files can be bootstrapped using the below commands:

```shell
$ PACKAGE="newpackage"
$ cd pkg/${PACKAGE}
$ ginkgo bootstrap
$ ginkgo generate ${PACKAGE}.go

find .
./newpackage.go
./newpackage_suite_test.go
./newpackage_test.go
```

### How to run the tests

* You can run the tests using `make test` or `go test ./...`

OR

* Can also run the tests using the `ginkgo` binary

```shell
ginkgo -v pkg/...
```

## Functional Tests

For functional testing, can refer to [README.md](../test/README.md) file. In short, following commands need to be run on the staging cluster:

```bash
export CLUSTERNAME="my-staging-cluster"

# Change directory to the test directory
cd test

# Build and run the ocm-agent binary locally in one terminal session
./build-and-run.sh ${CLUSTERNAME}

# Open another terminal and run below command from test/ directory
./test-alerts.sh ${CLUSTERNAME}
```

The above will validate if the basic functionality of `ocm-agent` to startup, authenticate with OCM and send servicelog is successful or not.
