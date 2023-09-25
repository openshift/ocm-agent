package ocm

import "github.com/openshift/ocm-agent-operator/api/v1alpha1"

type ServiceLog struct {
	ServiceName   string                               `json:"service_name"`
	ClusterUUID   string                               `json:"cluster_uuid,omitempty"`
	Summary       string                               `json:"summary"`
	Description   string                               `json:"description"`
	InternalOnly  bool                                 `json:"internal_only"`
	Severity      v1alpha1.NotificationSeverity        `json:"severity"`
	LogType       string                               `json:"log_type,omitempty"`
	DocReferences []v1alpha1.NotificationReferenceType `json:"doc_references,omitempty"`
}
