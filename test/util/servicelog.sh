#!/bin/bash

## servicelog.sh - Library to handle different operations regarding Service Logs

function get-servicelog-count() {
        ID=${1}
	COUNT=$(ocm get /api/service_logs/v1/cluster_logs --parameter search="cluster_uuid='${ID}'" | jq '.total')
        return ${COUNT}
}

function check-servicelog-count() {
        ID=${1}
        PRE_COUNT=${2}
	EXPECT_NEW_SL=${3}
	EXPECTED_COUNT=$((${PRE_COUNT} + ${EXPECT_NEW_SL}))
	get-servicelog-count ${ID}
        ACTUAL_COUNT=$?

	echo
        if [[ ${ACTUAL_COUNT} = ${EXPECTED_COUNT} ]]
        then
                echo "TEST PASSED = Expected SL count: ${EXPECTED_COUNT}, Got SL count: ${ACTUAL_COUNT}."
        else
                echo "TEST FAILED = Expected SL count: ${EXPECTED_COUNT}, Got SL count: ${ACTUAL_COUNT}."
        fi
}

