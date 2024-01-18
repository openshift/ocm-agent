#!/bin/bash

######################################################################################################
# Script: build-and-run.sh
# Purpose: To locally build and run ocm-agent binary to be perform integration tests
# Execution Overview:
# - Script pre-requisite is a staging cluster from where KUBECONFIG is fetched
# - ocm-agent is built locally using `make build`
# - The arguments for ocm-agent CLI are fetched from the staging cluster itself
# - The logs of ocm-agent will show and progress/errors for the testing done
# - Once the script is stopped using CTRL+c, the KUBECONFIG file is deleted
#
# To perform integration testing, run script test-alerts.sh in another terminal session.
######################################################################################################

trap ctrl_c INT

function ctrl_c() {
	echo
	echo "Deleting temporary KUBECONFIG..."
	rm -f ${TEMPKUBECONFIG}
}

if [[ -z $1 ]]; then
    echo "Please provide the staging cluster name to be used for OCM Agent testing.."
    echo "Usage: $0 CLUSTER_NAME [--fleet-mode]"
    exit 1
fi

export CLUSTER_NAME=$1
export GIT_ROOT=$(git rev-parse --show-toplevel)
TEMPKUBECONFIG=/tmp/${CLUSTER_NAME}-kubeconfig-temp

# Check for the --fleet-mode parameter
if [[ $2 == "--fleet-mode" ]]; then
    FLEET_MODE="--fleet-mode"
else
    FLEET_MODE=""
fi

OCM_STATUS=$(ocm account status)
echo ${OCM_STATUS} | grep -q "https://api.stage.openshift.com"

if [[ $? -ne 0 ]]
then
	echo
	echo "Please login to OCM Stage account to run this script..."
	exit 1
fi

echo
echo "--- Fetching Cluster ID and creating temporary KUBECONFIG..."
export OCM_CLUSTERID=$(ocm list clusters --managed | grep -w ${CLUSTER_NAME} | awk '{ print $1 }')
export EXT_CLUSTERID=$(ocm describe cluster $OCM_CLUSTERID --json | jq -r '.external_id')
ocm get /api/clusters_mgmt/v1/clusters/$OCM_CLUSTERID/credentials | jq -r .kubeconfig > $TEMPKUBECONFIG
echo $TEMPKUBECONFIG
export KUBECONFIG=${TEMPKUBECONFIG}
export OCM_AGENT_CONFIGMAP="ocm-agent-cm"

echo
echo "--- Building ocm-agent locally..."
make -C ${GIT_ROOT} build

echo
echo "--- Running ocm-agent locally..."
echo "(Keep this terminal open to follow log of ocm-agent. Open new terminal and run test scripts.)"
echo "Link: http://localhost:8081"
echo

SERVICE=$(oc -n openshift-ocm-agent-operator exec -ti deployment/ocm-agent -- cat /configs/${OCM_AGENT_CONFIGMAP}/services)
OCM_BASE_URL=$(oc -n openshift-ocm-agent-operator exec -ti deployment/ocm-agent -- cat /configs/${OCM_AGENT_CONFIGMAP}/ocmBaseURL)

if [[ -n $FLEET_MODE ]]; then
	# Load fleet mode credentials for staging from vault
	export VAULT_ADDR="https://vault.devshift.net"
	export VAULT_TOKEN="$(vault login -method=oidc -token-only)"

	for v in $(vault kv get  -format=json osd-sre/ocm-agent/ocm/ocm-agent-staging | jq -r ".data.data|to_entries|map(\"\(.key)=\(.value|tostring)\")|.[]"); do export $v; done
	
	unset VAULT_ADDR VAULT_TOKEN
    ${GIT_ROOT}/build/_output/ocm-agent serve --services "$SERVICE" --ocm-url "$OCM_BASE_URL" $FLEET_MODE --ocm-client-id "$OA_OCM_CLIENT_ID" --ocm-client-secret "$OA_OCM_CLIENT_SECRET"
else
	# Load access token from the cluster for non fleet-mode
	ACCESS_TOKEN=$(oc -n openshift-ocm-agent-operator exec -ti deployment/ocm-agent -- cat /secrets/ocm-access-token/access_token)

    ${GIT_ROOT}/build/_output/ocm-agent serve --access-token "$ACCESS_TOKEN" --services "$SERVICE" --cluster-id "$EXT_CLUSTERID" --ocm-url "$OCM_BASE_URL"
fi
