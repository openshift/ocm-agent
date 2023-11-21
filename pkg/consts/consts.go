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

	// OCMAgentAccessFleetSecretPathBase is the base path where to find the secret
	OCMAgentAccessFleetSecretPathBase = "/secrets/"
	// OCMAgentAccessFleetSecretClientKey is the secret of client_id key for OA HS
	OCMAgentAccessFleetSecretClientKey = "OA_OCM_CLIENT_ID" //#nosec G101 -- This is a false positive
	// OCMAgentAccessFleetSecretClientSecretKey is the secret of client_secret key for OA HS
	OCMAgentAccessFleetSecretClientSecretKey = "OA_OCM_CLIENT_SECRET" //#nosec G101 -- This is a false positive
	// OCMAgentAccessFleetSecretURLKey is the secret of URL key for OA HS
	OCMAgentAccessFleetSecretURLKey = "OA_OCM_URL" //#nosec G101 -- This is a false positive

	// Service name for the sending service logs
	ServiceLogServiceName = "SREManualAction"

	// The URI parameter that represents the upgrade policy ID in OCM
	UpgradePolicyIdParam = "upgrade_policy_id"

	// The first page of a paginated 'list' request to OCM
	OCMListRequestStartPage = 1

	// The max amount of pages allowed for a single OCM request.
	OCMListRequestMaxPerPage = 100
)
