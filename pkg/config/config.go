package config

const (
	// AccessToken represents the Auth Access Token used for OCM communications
	AccessToken string = "access-token"
	// Services represents the list of OCM service APIs that OCM Agent will proxy
	Services string = "services"
	// OcmURL represents the base URL hosting the OCM API
	OcmURL string = "ocm-url"
	// Debug represents whether debug behaviours will be enabled
	Debug string = "debug"
	// ExternalClusterID represents the ID of the cluster used for OCM notifications
	ExternalClusterID string = "cluster-id"
	// FleetMode represents if ocm-agent is going to run in default OSD/ROSA mode or HyperShift mode
	FleetMode string = "fleet-mode"
	// OCMClientID represents the OCM Client ID that will be used for testing fleet-mode run
	OCMClientID string = "ocm-client-id"
	// OCMClientSecret represents the OCM Client ID that will be used for testing fleet-mode run
	OCMClientSecret string = "ocm-client-secret" //#nosec G101 -- This is a false positive

	ServiceLogService string = "service_logs" //#nosec G101 -- This is a false positive

	ClustersService string = "clusters_mgmt" //#nosec G101 -- This is a false positive

)
