#!/bin/bash

######################################################################################################
# Script: test-fleet-mode-alerts.sh
# Purpose: To run the integration tests between ocm-agent and OCM (Service Log/Limited Support)
# Execution Overview:
# - To add/remove/modify any tests, this script needs to be edited.
# - This script uses scripts under 'util/' directory to add any new test.
# - Each run of the script should give idempotent result i.e, one or more runs of the script should give same result.
######################################################################################################
set -e
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
export EXTERNAL_ID=$(ocm describe cluster $OCM_CLUSTERID --json | jq -r '.external_id')
TEMPKUBECONFIG=/tmp/${CLUSTER}-kubeconfig-temp
export GIT_ROOT=$(git rev-parse --show-toplevel)
TEST_DIR=${GIT_ROOT}/test
alias create-alert=${TEST_DIR}/util/create-alert.sh
alias post-alert=${TEST_DIR}/util/post-alert.sh
source ${TEST_DIR}/util/ocm.sh
source ${TEST_DIR}/util/oc.sh

TEMPLATE_ALERT_JSON_FILE="${TEST_DIR}/template-alert-fleet-notification.json"
TMP_ALERT_FILE=/tmp/alert_oa_integration.json

# Can be used for --start-date and/or --end-date tests if required
TODAY_DATE=$(date -u +%Y-%m-%d)
TOMORROW_DATE=$(date -u +%Y-%m-%d -d "$DATE +1 day")

# Test default managedfleetnotification which will exist on call clusters
SERVICE_LOG_TEST_MFN_NAME="sre-managed-fleet-notification-sl"
LIMITED_SUPPORT_TEST_MFN_NAME="sre-managed-fleet-notification-ls"

function PreCheck() {
	echo
	echo "- Running Pre Test Check to recreate the default ManagedFleetNotification for successful tests..."
	export KUBECONFIG=${TEMPKUBECONFIG}
	oc -n openshift-ocm-agent-operator delete managedfleetnotification ${SERVICE_LOG_TEST_MFN_NAME} || echo "Found no existing MFN with name ${SERVICE_LOG_TEST_MFN_NAME} to delete"
	oc -n openshift-ocm-agent-operator delete managedfleetnotification ${LIMITED_SUPPORT_TEST_MFN_NAME} || echo "Found no existing MFN with name ${LIMITED_SUPPORT_TEST_MFN_NAME} to delete"
	oc -n openshift-ocm-agent-operator apply -f ${TEST_DIR}/manifests/${SERVICE_LOG_TEST_MFN_NAME}.yaml
	oc -n openshift-ocm-agent-operator apply -f ${TEST_DIR}/manifests/${LIMITED_SUPPORT_TEST_MFN_NAME}.yaml

	# Clean up records to allow re-send
	oc -n openshift-ocm-agent-operator delete --all managedfleetnotificationrecord || echo "Found no existing MFN records to delete"
}

PreCheck

# Test fleet mode Service Log for a firing alert using defaults
echo
echo "### TEST 1 - Send Service log for a firing alert"
echo
PRE_SL_COUNT=$(get-servicelog-count ${EXTERNAL_ID})
echo "Pre-service-log count: $PRE_LS_COUNT"
# We are using a random MC id to make sure we get a new managedfleetnotificationrecord object
random_mc_id=$(tr -dc 'a-z' < /dev/urandom | head -c 5)
create-alert --hc-id ${EXTERNAL_ID} --mc-id $random_mc_id -t "audit-webhook-error-putting-minimized-cloudwatch-log" --template "$TEMPLATE_ALERT_JSON_FILE" > ${TMP_ALERT_FILE}
post-alert ${TMP_ALERT_FILE}
sleep 3
check-servicelog-count ${EXTERNAL_ID} ${PRE_SL_COUNT} 1
check-mfnri-count ${random_mc_id} ${EXTERNAL_ID} 1 0 # Expect 1 firing, 0 resolved


# We are using a random MC id to make sure we get a new managedfleetnotificationrecord object
# And avoid the resend timeout
# For test 2 and 3 they are shared, as we want to map to the same record (fire and resolve)
# Test Limited Support for a firing alert using defaults
echo
echo "### TEST 2 - Send Limited Support for a firing alert"
echo
random_mc_id=$(tr -dc 'a-z' < /dev/urandom | head -c 5)
PRE_LS_COUNT=$(get-limited-support-count ${EXTERNAL_ID})
create-alert --hc-id ${EXTERNAL_ID} --mc-id $random_mc_id -t "oidc-deleted-notification" --template "$TEMPLATE_ALERT_JSON_FILE" > ${TMP_ALERT_FILE}
post-alert ${TMP_ALERT_FILE}
sleep 3
EXPECTED_COUNT=$((${PRE_LS_COUNT} + 1))
check-limited-support-count ${EXTERNAL_ID} ${EXPECTED_COUNT}
check-mfnri-count ${random_mc_id} ${EXTERNAL_ID} 1 0 # Expect 1 firing, 0 resolved

echo
echo "### TEST 3 - Resend Limited Support for a firing alert without a resolve inbetween"
echo
sleep 3
check-limited-support-count ${EXTERNAL_ID} ${EXPECTED_COUNT}
check-mfnri-count ${random_mc_id} ${EXTERNAL_ID} 1 0 # Expect 1 firing, 0 resolved

# Test Limited support for resolved alert using defaults. 
echo
echo "### TEST 4 - Remove Limited Support for resolved alert"
echo
PRE_LS_COUNT=$(get-limited-support-count ${EXTERNAL_ID})
create-alert --hc-id ${EXTERNAL_ID} --mc-id $random_mc_id --template "$TEMPLATE_ALERT_JSON_FILE" -t "oidc-deleted-notification" --alert-status resolved > ${TMP_ALERT_FILE}
post-alert ${TMP_ALERT_FILE}
sleep 3
EXPECTED_COUNT=$((${PRE_LS_COUNT} - 1))
check-limited-support-count ${EXTERNAL_ID} ${EXPECTED_COUNT}
check-mfnri-count ${random_mc_id} ${EXTERNAL_ID} 1 1 # Expect 1 firing, 1 resolved

# Test parallel execution for a single record
echo
echo "### TEST 5 - Parallel execution"
echo
random_mc_id=$(tr -dc 'a-z' < /dev/urandom | head -c 5)
create-alert --hc-id ${EXTERNAL_ID} --mc-id $random_mc_id -t "audit-webhook-error-putting-minimized-cloudwatch-log" --template "$TEMPLATE_ALERT_JSON_FILE" > ${TMP_ALERT_FILE}
PRE_SL_COUNT=$(get-servicelog-count ${EXTERNAL_ID})

for ((i=1; i<=10; i++)); do
  post-alert ${TMP_ALERT_FILE} &
done
sleep 25 # Wait for everything to be handled
check-servicelog-count ${EXTERNAL_ID} ${PRE_SL_COUNT} 10
check-mfnri-count ${random_mc_id} ${EXTERNAL_ID} 10 0 # Expect 10 firing, 0 resolved