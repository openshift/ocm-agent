# Metrics

OCM Agent will expose the follow metrics via web server metric port `8383`.

## Related resources

`Service` and `ServiceMonitor`: Those are managed by the [OCM Agent operator](https://github.com/openshift/ocm-agent-operator/)

`PrometheusRules`: Defines the alerting rules based on the metrics in
[Managed Cluster Config](https://github.com/openshift/managed-cluster-config/)

`Role` and `RoleBinding`: Required role and rolebinding to scrape the metrics from
OCM Agent are managed by [OCM Agent operator](https://github.com/openshift/ocm-agent-operator/)

## List of metrics

|name|type|description|
|----|----|----|
|ocm_agent_requests_total|Counter|A count of total requests to ocm agent service|
|ocm_agent_requests_by_service|Counter|A count of total requests to ocm agent based on sub service|
|ocm_agent_failed_requests_total|Counter|A count of total failed requests received by the OCM Agent service|
|ocm_agent_request_failure|Gauge|Indicates that OCM Agent could not successfully process a request|
|ocm_agent_response_failure|Gauge|Indicates that the call to the OCM service endpoint failed|
|ocm_agent_service_log_sent|Counter|A count of service log sent based on managedNotification template for the current session|
|ocm_agent_failed_service_logs_total|Counter|A count of service logs which failed to be sent. This includes service logs which failed to be formatted.|
|ocm_agent_service_log_sent_total|Gauge|A total number of service log being sent based on managedNotification template|
|ocm_agent_pull_secret_invalid|Gauge|Pull Secret auth token is not valid|
|ocm_agent_limited_support_sent_total|Counter| Total number of limited support being sent based on fleetNotification template|
|ocm_agent_limited_support_removed_total|Counter|Total number of limited support removed based on fleetNotification template|
|ocm_agent_limited_support_send_failure_total|Counter|Total number of failures for limited support posts based on fleetNotification template|
|ocm_agent_limited_support_removal_failure_total|Counter|Total number of failures for limited support removals based on fleetNotification template|

## Metrics reset

The reset for the Gauge metric `ocm_agent_request_failure` and `ocm_agent_response_failure`
will be triggered automatically when the next request/response got succeeded.
