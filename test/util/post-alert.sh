#!/bin/bash
set -e
## post-alert-sh - Script to POST alert json payload to OCM Agent

if [[ -z $1 ]]
then
        echo "Please provide the json payload file for the alert that needs to be POSTed to OCM agent..."
        echo "Usage: $0 FILE"
        exit 1
fi

export ALERT_FILE=$1

echo "- Command: curl -s -X POST http://localhost:8081/alertmanager-receiver -H 'Content-Type: application/json' -d @${ALERT_FILE}"
RESPONSE=$(curl -s -X POST http://localhost:8081/alertmanager-receiver -H 'Content-Type: application/json' -d @${ALERT_FILE})
echo "- Output: ${RESPONSE}"

if [[ ${RESPONSE} == "" ]]
then
	echo "- ERROR: There was no response from ocm-agent! Check client side. Increase verbosity of curl command by adding '-v'"
	echo "- Alert failed to be POSTed to ocm-agent!"
	exit 1
fi

OUTPUT=$(echo ${RESPONSE} | jq -r '.Status')

if [[ ${OUTPUT} == "ok" ]]
then
	echo "- Alert POSTed successfully to ocm-agent!"
else
	echo "- Alert failed to be POSTed to ocm-agent!"
	echo "- Tip: Check ocm-agent logs for server error"
	exit 1
fi
