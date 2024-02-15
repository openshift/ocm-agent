#!/bin/bash
set -e
## oc.sh - Library with different oc helper functions

function get-mfnri-firingcount() {
    MC_ID=${1}
    CLUSTER_ID=${2}
    FIRING_NOTIFICATIONS_SENT_COUNT=$(oc get managedfleetnotificationrecords ${MC_ID} -n openshift-ocm-agent-operator -ojson | jq -r ".status.notificationRecordByName[0].notificationRecordItems[] | select(.hostedClusterID == \"$CLUSTER_ID\") | .firingNotificationSentCount")
    echo ${FIRING_NOTIFICATIONS_SENT_COUNT}
}

function get-mfnri-resolvedcount() {
    MC_ID=${1}
    CLUSTER_ID=${2}
    RESOLVED_NOTIFICATIONS_SENT_COUNT=$(oc get managedfleetnotificationrecords ${MC_ID} -n openshift-ocm-agent-operator -ojson | jq -r ".status.notificationRecordByName[0].notificationRecordItems[] | select(.hostedClusterID == \"$CLUSTER_ID\") | .resolvedNotificationSentCount")
    echo ${RESOLVED_NOTIFICATIONS_SENT_COUNT}
}

function check-mfnri-count() {
    MC_ID=${1}
    CLUSTER_ID=${2}
	EXPECTED_COUNT_FIRING=${3}
    EXPECTED_COUNT_RESOLVED=${4}

    ACTUAL_COUNT_FIRING=$(get-mfnri-firingcount ${MC_ID} ${CLUSTER_ID})
    ACTUAL_COUNT_RESOLVED=$(get-mfnri-resolvedcount ${MC_ID} ${CLUSTER_ID})
	echo
    if [[ ${ACTUAL_COUNT_FIRING} = ${EXPECTED_COUNT_FIRING} && ${ACTUAL_COUNT_RESOLVED} = ${EXPECTED_COUNT_RESOLVED} ]]
    then
        echo "TEST PASSED = Expected firingNotificationSentCount: ${EXPECTED_COUNT_FIRING}, Got Firing count: ${ACTUAL_COUNT_FIRING}."
        echo "              Expected resolvedNotificationSentCount: ${EXPECTED_COUNT_RESOLVED}, Got Resolved count: ${ACTUAL_COUNT_RESOLVED}."
    else
        echo "TEST FAILED = Expected firingNotificationSentCount: ${EXPECTED_COUNT_FIRING}, Got Firing count: ${ACTUAL_COUNT_FIRING}."
        echo "              Expected resolvedNotificationSentCount: ${EXPECTED_COUNT_RESOLVED}, Got Resolved count: ${ACTUAL_COUNT_RESOLVED}."
    fi
}