#!/bin/bash
set -e

## create-alert.sh script creates the json format for the alert payload that's passed to the OCM agent.

usage() {
    cat <<EOF
    usage: $0 [ OPTIONS ]
    Options:
    -s|--alert-status              Status of the alert (Valid values are "firing", "resolved")
    -n|--alert-name                Alert name (Default value: LoggingVolumeFillingUpNotificationSRE)
    -m|--send-managed-notification Send managed notification boolean value (Default value: "true")
    -t|--notification-template     Name of managed notification template (Default: LoggingVolumeFillingUp)
    --hc-id                        [For fleet-mode only] ID of the cluster  (Default: None)
    --mc-id                        [For fleet-mode only] ID of the management cluster  (Default: None)
    --start-date                   The date when the alert state started being in effect (Default: Today ($(date -u +%Y-%m-%d))
    --end-date                     The date when the alert state stopped being in effect (Default: 0001-01-01)
    --template                     The template alert json file (Default: ${GIT_ROOT}/test/template-alert.json)

    Example:
    $0 -s firing -n LoggingVolumeFillingUpNotificationSRE -m true -t LoggingVolumeFillingUp --start-date $(date -u +%Y-%m-%d) --end-date 0001-01-01
EOF
}

VALID_ARGS=$(getopt -o s:n:m:t:hf --long alert-status:,alert-name:,send-managed-notification:,template:,start-date:,end-date:,hc-id:,mc-id:,limited-support:,help -n "$(basename "$0")" -- "$@")
if [[ $? -ne 0 ]]; then
    exit 1
fi

# Template defaults
export ALERT_STATUS="firing"
export ALERT_NAME="LoggingVolumeFillingUpNotificationSRE"
export SEND_MANAGED_NOTIFICATION_BOOL="true"
export MANAGED_NOTIFICATION_TEMPLATE="LoggingVolumeFillingUp"
export START_DATE=$(date -u +%Y-%m-%d)
export END_DATE="0001-01-01"

# Script defaults
export GIT_ROOT=$(git rev-parse --show-toplevel)
TEST_DIR="${GIT_ROOT}/test"
TEMPLATE_ALERT_JSON_FILE="${TEST_DIR}/template-alert.json"

eval set -- "$VALID_ARGS"
while true ; do
  case "$1" in
    --hc-id)
        export HC_ID=${2}
        shift 2
        ;;
    --mc-id)
        export MANAGEMENT_CLUSTER_ID=${2}
        shift 2
        ;;
    --template)
        export TEMPLATE_ALERT_JSON_FILE=${2}
        shift 2
        ;;
    -s | --alert-status)
        export ALERT_STATUS=${2}
        shift 2
        ;;
    -n | --alert-name)
        export ALERT_NAME=${2}
        shift 2
        ;;
    -m | --send-managed-notification)
        export SEND_MANAGED_NOTIFICATION_BOOL=${2}
        shift 2
        ;;
    -t | --notification-template)
        export MANAGED_NOTIFICATION_TEMPLATE=${2}
        shift 2
        ;;
    --start-date)
        export START_DATE=${2}
        shift 2
        ;;
    --end-date)
        export END_DATE=${2}
        shift 2
        ;;
    -h | --help)
        usage
        exit
        ;;
    --) 
        break 
        ;;
    *)
        usage
        exit 1
        ;;
  esac
done

# Replace the environment variables from the template alert json file and output json on standard output
envsubst < "$TEMPLATE_ALERT_JSON_FILE"