#!/bin/bash

######################################################################################################
# Script: test-alerts.sh
# Purpose: To run the integration tests between ocm-agent and OCM (Service Log)
# Execution Overview:
# - To add/remove/modify any tests, this script needs to be edited.
# - This script uses scripts under 'util/' directory to add any new test.
# - Each run of the script should give idempotent result i.e, one or more runs of the script should give same result.
# - Right now, Service Log count is referred before and after the test but more criteria can be added.
######################################################################################################

shopt -s expand_aliases

# Defaults

if [[ -z $1 ]]
then
        echo "Please provide the staging cluster name to be used for OCM Agent alert testing.."
        echo "Usage: $0 CLUSTER_NAME"
        exit 1
fi

export CLUSTER=$1
export OCM_CLUSTERID=$(ocm list clusters --managed | grep -w ${CLUSTER} | awk '{ print $1 }')
export EXT_CLUSTERID=$(ocm describe cluster $OCM_CLUSTERID --json | jq -r '.external_id')
TEMPKUBECONFIG=/tmp/${CLUSTER}-kubeconfig-temp
export GIT_ROOT=$(git rev-parse --show-toplevel)
TEST_DIR=${GIT_ROOT}/test
alias create-alert=${TEST_DIR}/util/create-alert.sh
alias post-alert=${TEST_DIR}/util/post-alert.sh
source ${TEST_DIR}/util/ocm.sh

# Can be used for --start-date and/or --end-date tests if required
TODAY_DATE=$(date -u +%Y-%m-%d)
TOMORROW_DATE=$(date -u +%Y-%m-%d -d "$DATE +1 day")

# Test default managednotification which will exist on call clusters
DEFAULT_TEST_MN_NAME="sre-managed-notifications"

function PreCheck() {
	echo
	echo "- Running Pre Test Check to recreate the default ManagedNotification for successful tests..."
	export KUBECONFIG=${TEMPKUBECONFIG}
	oc -n openshift-ocm-agent-operator delete managednotification ${DEFAULT_TEST_MN_NAME}
	oc -n openshift-ocm-agent-operator apply -f ${TEST_DIR}/manifests/${DEFAULT_TEST_MN_NAME}.yaml
}

PreCheck

# Test Service Log for a firing alert using defaults
echo
echo "### TEST 1 - Send Service Log for a firing alert"
echo
ALERT_FILE=/tmp/firing-alert.json
PRE_SL_COUNT=$(get-servicelog-count ${EXT_CLUSTERID})
create-alert > ${ALERT_FILE}
post-alert ${ALERT_FILE}
sleep 3
check-servicelog-count ${EXT_CLUSTERID} ${PRE_SL_COUNT} 1

# Test Service Log again for same firing alert using defaults. Service Log should not be sent again
echo
echo "### TEST 2 - Do not send Service Log again for the same firing alert for same day"
echo
ALERT_FILE=/tmp/firing-alert.json
PRE_SL_COUNT=$(get-servicelog-count ${EXT_CLUSTERID})
create-alert > ${ALERT_FILE}
post-alert ${ALERT_FILE}
sleep 3
check-servicelog-count ${EXT_CLUSTERID} ${PRE_SL_COUNT} 0

# Test Service Log for resolved alert using defaults. Service Log should be sent now
echo
echo "### TEST 3 - Send Servicelog for resolved alert"
echo
ALERT_FILE=/tmp/resolved-alert.json
PRE_SL_COUNT=$(get-servicelog-count ${EXT_CLUSTERID})
create-alert --alert-status resolved > ${ALERT_FILE}
post-alert ${ALERT_FILE}
sleep 3
check-servicelog-count ${EXT_CLUSTERID} ${PRE_SL_COUNT} 1

echo
