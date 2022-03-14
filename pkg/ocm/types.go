package ocm

type ServiceLog struct {
	ServiceName  string `json:"service_name"`
	ClusterUUID  string `json:"cluster_uuid,omitempty"`
	Summary      string `json:"summary"`
	Description  string `json:"description"`
	InternalOnly bool   `json:"internal_only"`
}
