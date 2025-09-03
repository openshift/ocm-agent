## Locally running e2e test suite
When updating your operator it's beneficial to add e2e tests for new functionality AND ensure existing functionality is not breaking using e2e tests. 
To do this, following steps are recommended

1. Run "make e2e-binary-build"  to make sure e2e tests build 
2. Deploy your new version of operator in a test cluster
3. Run "go install github.com/onsi/ginkgo/ginkgo@latest"
4. Get kubeadmin credentials from your cluster using 

ocm get /api/clusters_mgmt/v1/clusters/(cluster-id)/credentials | jq -r .kubeconfig > /(path-to)/kubeconfig

5. Run test suite using 
 
DISABLE_JUNIT_REPORT=true KUBECONFIG=/(path-to)/kubeconfig  ./(path-to)/bin/ginkgo  --tags=osde2e -v test/e2e

## ocm-agent e2e test for local test
When running tests locally against a remote cluster, you will need to use `oc port-forward` to make the ocm-agent available to your local test environment.

First, find the name of the `ocm-agent` pod:
```bash
oc get pods -n openshift-ocm-agent-operator -l app=ocm-agent
```

Use the pod name to set up port forwarding:
```bash
oc -n openshift-ocm-agent-operator port-forward <ocm-agent-pod-name> 8081:8081
```

Run the tests with the `OCM_AGENT_URL` environment variable set:
```bash
export OCM_TOKEN=$(ocm token)
OCM_AGENT_URL=http://localhost:8081 DISABLE_JUNIT_REPORT=true KUBECONFIG=/(path-to)/kubeconfig ./bin/ginkgo --tags=osde2e -v test/e2e
```

                   ┌──────────────────────┐
                   │ Kubernetes Cluster   │
┌─────────────────┐│    ┌───────────────┐ │
│   Test Runner   ││    │  ocm-agent    │ │
│ (local machine) ││    │     Pod       │ │
│                 ││    │   (port 8081) │ │
└─────────────────┘│    └───────────────┘ │
           │       │              │       │
           │  oc port-forward     │       │
           │       │              │       │
           └──────────────────────┘       │
                   │                      │
                   └──────────────────────┘

## ocm-agent e2e image test for personal cluster

The e2e image can be executed in existing cluster. This will be similar to CI environment except provisioning a new cluster and ocm connection setup.
Several environment varibles should be set before run the e2e image test.
```
export TEST_IMAGE="YOUR TEST IMAGE in quay.io, eg quay.io/tkong-ocm/ocm-agent-e2e"
export IMAGE_TAG="tag of the image, eg latest"
export OCM_E2E_TOKEN=$(ocm token)
export AWS_ACCESS_KEY_ID="aws access key id"
export AWS_SECRET_ACCESS_KEY="aws access key"
export REGION="Same region as testing cluster"
export CLUSTER_ID="Testing cluster ID"
export OSD_ENV="stage or int"
envsubst < ./test/e2e/e2e-image-job.yaml | oc apply --as backplane-cluster-admin -f -
```

### Debugging ocm-agent e2e image test
The workflow for e2e test image is

```mermaid
flowchart LR
subgraph N[osde2e-executor-* ns]
C[ocm-agent e2e test job (executor-*)] --> D[ocm-agent e2e test pod (executor-*-*)]

A[osde2e image job] --> B[osd e2e image pod] --> subgraph N
```

So the actual e2e test is executed in ocm-agent e2e test pod.
Using command `oc get namespace | grep osde2e` to find the executor namespace. The pod log can be inspected to see the test results. eg
```
oc -n osde2e-executor-0409i logs executor-cqf92-f7w5p --as backplane-cluster-admin

Running Suite: Ocm Agent - /
============================
Random Seed: 1756259203

Will run 3 of 3 specs
SSS

Ran 0 of 3 Specs in 0.024 seconds
SUCCESS! -- 0 Passed | 0 Failed | 0 Pending | 3 Skipped
PASS
```
In this example, all the test are skipped. To figure more details, debug into the pod and execute command `/e2e.test --ginkgo.vv --ginkgo.trace --ginkgo.fail-on-empty` to run the tests again and check the detailed log.


