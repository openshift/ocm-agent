package handlers

import (
	"fmt"
	"net/http"
	"regexp"

	sdk "github.com/openshift-online/ocm-sdk-go"
	slv1 "github.com/openshift-online/ocm-sdk-go/servicelogs/v1"
	"github.com/openshift/ocm-agent-operator/api/v1alpha1"
	"github.com/openshift/ocm-agent/pkg/consts"
	"github.com/prometheus/alertmanager/template"
)

const (
	ServiceLogActivePrefix  = "Issue Notification"
	ServiceLogResolvePrefix = "Issue Resolution"
)

type ServiceLogBuilder struct {
	wrappedBuilder *slv1.LogEntryBuilder
	summary        string
	firingDesc     string
	resolveDesc    string
	references     []v1alpha1.NotificationReferenceType
}

type ServiceLog = slv1.LogEntry

func NewServiceLogBuilder(summary, firingDesc, resolveDesc, clusterUUID string, severity v1alpha1.NotificationSeverity, logType string, references []v1alpha1.NotificationReferenceType) *ServiceLogBuilder {
	return &ServiceLogBuilder{
		wrappedBuilder: slv1.NewLogEntry().
			ServiceName(consts.ServiceLogServiceName).
			ClusterUUID(clusterUUID).
			InternalOnly(false).
			Severity(slv1.Severity(severity)).
			LogType(slv1.LogType(logType)),
		summary:     summary,
		firingDesc:  firingDesc,
		resolveDesc: resolveDesc,
		references:  references,
	}
}

var (
	slVarRefRe = regexp.MustCompile(`\${[^{}]*}`)
)

// Replace place holders in the given string with the alert labels and annotations
func replacePlaceHoldersInString(s string, alert *template.Alert) (string, error) {
	var err error
	resolvePlaceHolder := func(placeHolder string) string {
		if err == nil {
			key, value, isOk := placeHolder[2:len(placeHolder)-1], "", false

			if value, isOk = alert.Labels[key]; !isOk {
				if value, isOk = alert.Annotations[key]; !isOk {
					err = fmt.Errorf("alert has no '%s' label or annotation which could be used to replace place holders in the template", key)

					return placeHolder
				}
			}
			return value
		}

		return placeHolder
	}

	return slVarRefRe.ReplaceAllStringFunc(s, resolvePlaceHolder), err
}

func (b *ServiceLogBuilder) Build(firing bool, alert *template.Alert) (*ServiceLog, error) {
	var summary, description string
	var docReferences []string

	// Adjust the summary based on whether the alert is firing or resolved.
	if firing {
		summary = ServiceLogActivePrefix + ": " + b.summary
		description = b.firingDesc
	} else {
		summary = ServiceLogResolvePrefix + ": " + b.summary
		description = b.resolveDesc
	}

	// Replace the place holders in the summary & the description with alert labels & annotations
	if alert != nil {
		var err error

		if summary, err = replacePlaceHoldersInString(summary, alert); err != nil {
			return nil, err
		}
		if description, err = replacePlaceHoldersInString(description, alert); err != nil {
			return nil, err
		}
	}

	// Handle DocReferences
	if b.references != nil && len(b.references) > 0 {
		for _, ref := range b.references {
			docReferences = append(docReferences, string(ref))
		}
	}

	// Directly assign the docReferences slice
	logEntry, err := b.wrappedBuilder.
		Summary(summary).
		Description(description).
		DocReferences(docReferences...).
		Build()

	return logEntry, err
}

type OCMClient interface {
	SendServiceLog(logEntry *slv1.LogEntry) error
}

type ocmClientImpl struct {
	ocmConnection *sdk.Connection
}

//go:generate mockgen -destination=mocks/service_logs.go -package=mocks github.com/openshift/ocm-agent/pkg/handlers OCMClient
func NewOcmClient(ocmConnection *sdk.Connection) OCMClient {
	return &ocmClientImpl{
		ocmConnection: ocmConnection,
	}
}

func (o *ocmClientImpl) SendServiceLog(logEntry *slv1.LogEntry) error {
	// Use the OCM SDK to construct the request for posting a service log for a specific cluster.
	request := o.ocmConnection.ServiceLogs().V1().ClusterLogs().Add().Body(logEntry)

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

func BuildAndSendServiceLog(slBuilder *ServiceLogBuilder, firing bool, alert *template.Alert, ocmClient OCMClient) error {
	logEntry, err := slBuilder.Build(firing, alert)
	if err != nil {
		return err
	}
	return ocmClient.SendServiceLog(logEntry)
}
