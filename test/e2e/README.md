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
export OCM_THIRDPARTY_TOKEN=$(ocm token)
OCM_AGENT_URL=http://localhost:8081 DISABLE_JUNIT_REPORT=true KUBECONFIG=/(path-to)/kubeconfig ./bin/ginkgo --tags=osde2e -v test/e2e
```
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
```

## ocm-agent e2e test for personal cluster
Should set OCM_THIRDPARTY_TOKEN in e2e-job.yaml via `ocm token`
export OCM_E2E_IMAGE="YOUR TEST IMAGE"
export OCM_E2E_TOKEN="UPPER ocm token"
envsubst < ./test/e2e/e2e-personal-job.yaml | oc apply --as backplane-cluster-admin -f -