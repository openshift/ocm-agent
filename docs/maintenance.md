<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [OCM Agent Maintenance](#ocm-agent-maintenance)
  - [Updating versions](#updating-versions)
    - [Updating Golang version](#updating-golang-version)
    - [Updating go.mod dependencies](#updating-gomod-dependencies)
    - [Updating the base image version](#updating-the-base-image-version)
    - [Rebuilding the controller-runtime mocks for tests](#rebuilding-the-controller-runtime-mocks-for-tests)
  - [Validating version changes](#validating-version-changes)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# OCM Agent Maintenance

Unlike the maintenance tasks for SREP managed operators, the maintenance tasks for ocm-agent are a bit different since it's repository is not based on [boilerplate](https://github.com/openshift/boilerplate). This document covers some bits regarding how to update some versions/dependencies.

## Updating versions

### Updating Golang version

To update the Golang minor version (example 1.17 to 1.19), there are two main files to update:

1. [openshift-ocm-agent-master.yaml](https://github.com/openshift/release/blob/master/ci-operator/config/openshift/ocm-agent/openshift-ocm-agent-master.yaml) file in [openshift/release](https://github.com/openshift/release) repository.
2. [Dockerfile](https://github.com/openshift/ocm-agent/blob/master/build/Dockerfile) file in [openshift/ocm-agent](https://github.com/openshift/ocm-agent) repository.

Example Jira with relevant PRs - [OSD-15608](https://issues.redhat.com/browse/OSD-15608)

### Updating go.mod dependencies

The easiest and the cleanest way to update the go.mod dependencies is to do the following:

1. Fork openshift/ocm-agent repo and checkout a new branch
2. Edit go.mod file and just keep following content in the file with the desired/minimum Golang version.

    ```go
    module github.com/openshift/ocm-agent

    go 1.19
    ```

3. Run command `go mod tidy` to fetch the required latest dependencies.

    NOTE: This command might not always work and there "could" be some dependency failures which need to be solved manually.

### Updating the base image version

In order to align to latest base image, can refer to [ubi8/ubi-micro:latest](https://catalog.redhat.com/software/containers/ubi8/ubi-micro/5ff3f50a831939b08d1b832a?tag=latest) image from the software catalog and update the [Dockerfile](https://github.com/openshift/ocm-agent/blob/master/build/Dockerfile) file in [openshift/ocm-agent](https://github.com/openshift/ocm-agent) repository.

### Rebuilding the controller-runtime mocks for tests

If there is an update in the controller-runtime version, then it's possible that the mock clients might have different method signatures so might need to be recreated as well. Following steps can be performed to recreate the mocks:

Prerequisite is to have the `mockgen` binary present. Refer [testing.md](testing.md) file to review the steps to have this binary present.

```bash
# Go to the path where the mock client is present
cd $(dirname $(find pkg/ -name cr-client.go;))
```

Next, follow the steps mentioned in [README.md](../pkg/util/test/generated/mocks/client/README.md) to rebuild the mock clients.

## Validating version changes

Majority of the checks and tests are covered by the PR tests as such but for local validation can run the following commands to validate the changes.

1. `go mod tidy` - Validate that the go.mod dependencies are the latest and compatible with required Go version.
2. `make test` - Run the unit tests locally to validate the changes. Can also refer to [testing.md](testing.md).
3. `make lint` - Run the lint tests.
4. `make build` - Create the ocm-agent binary locally and test the startup is fine and test the binary in default mode as well as fleet mode.
5. Lastly, make sure that [functional tests](testing.md#functional-tests) is successful as well.
