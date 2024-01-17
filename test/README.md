# Testing OCM Agent

This directory comprises of scripts and templates required to perform integration testing between OCM Agent and OCM.

## Procedure

1. First step is to build and run ocm-agent locally. Run the following script in a terminal:

```bash
./build-and-run.sh ${STAGING_CLUSTER_NAME}
```

The ocm-agent will continue running and while the local testing process, can watch the logs for any errors/information etc.

2. Second step is to take a new terminal session and run the set of integration tests defined. For now, the integration tests for alerts can be run via following script:

```bash
./test-alerts.sh ${STAGING_CLUSTER_NAME}

# For fleet mode, run
./test-fleet-mode-alerts.sh ${STAGING_CLUSTER_NAME}
```

Each test run will show PASSED or FAILED result as part of the integration test. New tests can be added in same script to test integration between ocm-agent and OCM (Service Log service).

That's it !

## Directory Layout

Considering `ocm-agent/test/` directory:

- `*.sh` scripts = These scripts under test/ directory are meant to be run directly for testing purpose. Examples:
  - `build-and-run.sh`
  - `test-alerts.sh`
  - `test-fleet-mode-alerts.sh`
- `util/` = Scripts under this directory are meant to be helper or util scripts to aid in easier adding of more tests.
- `template-alert.json` and `template-alert-fleet-notification.json`= These template alert json files have environment variables that can be replaced by the `util/create-alert.sh` script. It is used to create other json files for alerts that are used for testing.
- `manifests/` = This directory can have any of the yaml or json files used for testing purposes which are typically Kubernetes/Openshift resources.
