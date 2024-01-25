#!/bin/bash
set -e
## ocm.sh - Library to handle different ocm operations

function get-servicelog-count() {
        ID=${1}
	COUNT=$(ocm get /api/service_logs/v1/clusters/cluster_logs -p cluster_uuid=${ID} | jq '.total')

        # Check the exit status of the ocm command
        if [ $? -ne 0 ]; then
                echo "Error: Failed to get service log count for cluster ${ID}."
                exit 1
        fi
        
        echo ${COUNT}
}

function get-limited-support-count() {
        ID=${1}
        INTERNAL_ID=$(ocm get /api/clusters_mgmt/v1/clusters --parameter search="external_id like '${ID}'" | jq -r '.items[0].id')
	COUNT=$(ocm get /api/clusters_mgmt/v1/clusters/${INTERNAL_ID}/limited_support_reasons | jq '.total')

        # Check the exit status of the ocm command
        if [ $? -ne 0 ]; then
                echo "Error: Failed to get limited support count for cluster ${ID}."
                exit 1
        fi

        echo ${COUNT}
}

function check-servicelog-count() {
        ID=${1}
        PRE_COUNT=${2}
	EXPECT_NEW_SL=${3}
	EXPECTED_COUNT=$((${PRE_COUNT}+${EXPECT_NEW_SL}))
        ACTUAL_COUNT=$(get-servicelog-count ${ID})

	echo
        if [[ ${ACTUAL_COUNT} = ${EXPECTED_COUNT} ]]
        then
                echo "TEST PASSED = Expected SL count: ${EXPECTED_COUNT}, Got SL count: ${ACTUAL_COUNT}."
        else
                echo "TEST FAILED = Expected SL count: ${EXPECTED_COUNT}, Got SL count: ${ACTUAL_COUNT}."
        fi
}

function check-limited-support-count() {
        ID=${1}
	EXPECTED_COUNT=${2}
	ACTUAL_COUNT=$(get-limited-support-count ${ID})

	echo
        if [[ ${ACTUAL_COUNT} = ${EXPECTED_COUNT} ]]
        then
                echo "TEST PASSED = Expected LS count: ${EXPECTED_COUNT}, Got LS count: ${ACTUAL_COUNT}."
        else
                echo "TEST FAILED = Expected LS count: ${EXPECTED_COUNT}, Got LS count: ${ACTUAL_COUNT}."
        fi
}
