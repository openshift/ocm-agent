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
	// ClusterID represents the ID of the cluster used for OCM notifications
	ClusterID string = "cluster-id"
)
