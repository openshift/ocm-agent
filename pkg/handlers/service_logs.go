package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	sdk "github.com/openshift-online/ocm-sdk-go"
	slv1 "github.com/openshift-online/ocm-sdk-go/servicelogs/v1"
	"github.com/openshift/ocm-agent-operator/api/v1alpha1"
	"github.com/openshift/ocm-agent/pkg/consts"
	log "github.com/sirupsen/logrus"
)

type OCMClient interface {
	SendServiceLog(summary, firingDesc, resolveDesc, clusterID string, severity v1alpha1.NotificationSeverity, logType string, references []v1alpha1.NotificationReferenceType, firing bool) error
}

// ServiceLogsHandler manages service logs via the OCM API.
type ServiceLogsHandler struct {
	ocm *sdk.Connection
}

// NewServiceLogsHandler creates a new handler for service logs.
func NewServiceLogsHandler(o *sdk.Connection) *ServiceLogsHandler {
	log.Debug("Creating new service logs handler")
	return &ServiceLogsHandler{
		ocm: o,
	}
}

// PostServiceLog sends a service log for a specific cluster to the OCM API.
func (h *ServiceLogsHandler) PostServiceLog(logEntry *slv1.LogEntry) error {
	// Use the OCM SDK to construct the request for posting a service log for a specific cluster.
	request := h.ocm.ServiceLogs().V1().ClusterLogs().Add().Body(logEntry)

	// Send the request to the OCM API.
	response, err := request.Send()
	if err != nil {
		return fmt.Errorf("can't post service log: %v", err)
	}

	// Check the response status code.
	if response.Status() != http.StatusCreated {
		// Extract error details from the response and return an appropriate error.
		return fmt.Errorf("unexpected status: %d", response.Status())
	}

	return nil
}

func (o *ocmsdkclient) SendServiceLog(summary, firingDesc, resolveDesc, clusterUUID string, severity v1alpha1.NotificationSeverity, logType string, references []v1alpha1.NotificationReferenceType, firing bool) error {
	// Construct the LogEntry using the builder pattern provided by the SDK.
	logBuilder := slv1.NewLogEntry().
		Summary(summary).
		Description(firingDesc).
		ServiceName(consts.ServiceLogServiceName).
		ClusterUUID(clusterUUID).
		InternalOnly(false).
		Severity(slv1.Severity(severity)).
		LogType(slv1.LogType(logType))

	// Adjust the summary based on whether the alert is firing or resolved.
	if firing {
		refs, err := json.Marshal(references)
		if err != nil {
			return fmt.Errorf("failed to marshal references: %v", err)
		}
		refsDesc := fmt.Sprintf("References: %s", string(refs))
		descriptionWithRefs := fmt.Sprintf("%s\n%s", firingDesc, refsDesc)
		logBuilder = logBuilder.Summary(ServiceLogActivePrefix + ": " + summary).Description(descriptionWithRefs)
	} else {
		logBuilder = logBuilder.Summary(ServiceLogResolvePrefix + ": " + summary).Description(resolveDesc)
	}

	// Build the log entry.
	logEntry, err := logBuilder.Build()
	if err != nil {
		return fmt.Errorf("failed to build log entry: %v", err)
	}

	// Initialize the ServiceLogsHandler with the OCM client.
	serviceLogsHandler := NewServiceLogsHandler(o.ocm)

	// Use the servicelog handler to send the service log.
	if err := serviceLogsHandler.PostServiceLog(logEntry); err != nil {
		return fmt.Errorf("failed to post service log: %v", err)
	}

	return nil
}
