package consts

const (
	// Listening port for the OCM Agent web service
	OCMAgentServicePort = 8081
	// Listening port for the OCM Agent metrics
	OCMAgentMetricsPort = 8383

	// Metrics path for OCM Agent service
	MetricsPath = "/metrics"
	// Ready probe path for OCM Agent web service
	ReadyzPath = "/readyz"
	// Live probe path for OCM Agent web service
	LivezPath = "/livez"
	// Alertmanger webhook receiver path
	WebhookReceiverPath = "/alertmanager-receiver"

	// Service name for the sending service logs
	ServiceLogServiceName = "SREManualAction"
)
