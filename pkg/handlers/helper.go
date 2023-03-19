package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/openshift-online/ocm-cli/pkg/arguments"
	sdk "github.com/openshift-online/ocm-sdk-go"
	oav1alpha1 "github.com/openshift/ocm-agent-operator/api/v1alpha1"
	"github.com/openshift/ocm-agent/pkg/config"
	"github.com/openshift/ocm-agent/pkg/consts"
	"github.com/openshift/ocm-agent/pkg/ocm"
	"github.com/prometheus/alertmanager/template"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	AMLabelAlertName           = "alertname"
	AMLabelTemplateName        = "managed_notification_template"
	AMLabelManagedNotification = "send_managed_notification"

	LogFieldNotificationName           = "notification"
	LogFieldResendInterval             = "resend_interval"
	LogFieldAlertname                  = "alertname"
	LogFieldAlert                      = "alert"
	LogFieldIsFiring                   = "is_firing"
	LogFieldManagedNotification        = "managed_notification_cr"
	LogFieldPostServiceLogOpId         = "post_servicelog_operation_id"
	LogFieldPostServiceLogFailedReason = "post_servicelog_failed_reason"
	ServiceLogActivePrefix             = "Issue Notification"
	ServiceLogResolvePrefix            = "Issue Resolution"

	// Header returned in OCM responses
	HeaderOperationId = "X-Operation-Id"
)

// Alert Manager receiver response
type AMReceiverResponse struct {
	Error  error
	Code   int
	Status string
}

// Use prometheus alertmanager template type for post data
type AMReceiverData template.Data

type AMReceiverAlert template.Alert

// OCMClient enables implementation of OCM Client
//
//go:generate mockgen -destination=mocks/webhookreceiver.go -package=mocks github.com/openshift/ocm-agent/pkg/handlers OCMClient
type OCMClient interface {
	SendServiceLog(n *oav1alpha1.Notification, firing bool) error
}

type ocmsdkclient struct {
	ocm *sdk.Connection
}

type WebhookReceiverHandler struct {
	c   client.Client
	ocm OCMClient
}

type OCMResponseBody struct {
	Reason string `json:"reason"`
}

func NewOcmClient(ocm *sdk.Connection) OCMClient {
	return &ocmsdkclient{
		ocm: ocm,
	}
}

// isValidAlert indicates whether the supplied alert is one that warrants being processed for a notification.
// Any or all of these situations should be treated as an error as it indicates that AlertManager is forwarding
// alerts to ocm-agent that it should not be.
func isValidAlert(alert template.Alert) bool {
	// An invalid alert won't have a name
	alertname, err := alertName(alert)
	if err != nil {
		log.WithError(err).Info("alertname missing for alert")
		return false
	}

	// An invalid alert won't have a send_managed_notification label
	if val, ok := alert.Labels[AMLabelManagedNotification]; !ok || val == "false" {
		log.WithField(LogFieldAlertname, alertname).Error("alert has no send_managed_notification label")
		return false
	}

	// An invalid alert won't have a managed_notification_template label
	if _, ok := alert.Labels[AMLabelTemplateName]; !ok {
		log.WithField(LogFieldAlertname, alertname).Error("alert has no managed notification defined")
		return false
	}

	return true
}

// alertName looks up the name of an AlertManager alert, or returns error if one does not exist
func alertName(a template.Alert) (*string, error) {
	if name, ok := a.Labels[AMLabelAlertName]; ok {
		return &name, nil
	}
	return nil, fmt.Errorf("no alertname defined in alert")
}

// SendServiceLog sends a servicelog notification for the given alert
func (o *ocmsdkclient) SendServiceLog(n *oav1alpha1.Notification, firing bool) error {
	req := o.ocm.Post()
	err := arguments.ApplyPathArg(req, "/api/service_logs/v1/cluster_logs")
	if err != nil {
		return err
	}

	sl := ocm.ServiceLog{
		ServiceName:  consts.ServiceLogServiceName,
		ClusterUUID:  viper.GetString(config.ClusterID),
		Summary:      n.Summary,
		InternalOnly: false,
	}

	// Use different Summary and Description for firing and resolved status for an alert
	if firing {
		sl.Description = n.ActiveDesc
		sl.Summary = ServiceLogActivePrefix + ": " + n.Summary
	} else {
		sl.Description = n.ResolvedDesc
		sl.Summary = ServiceLogResolvePrefix + ": " + n.Summary
	}
	slAsBytes, err := json.Marshal(sl)
	if err != nil {
		return err
	}

	req.Bytes(slAsBytes)

	res, err := req.Send()
	if err != nil {
		return err
	}

	operationId := res.Header(HeaderOperationId)
	err = responseChecker(operationId, res.Status(), res.Bytes())
	if err != nil {
		return err
	}

	return nil
}

// SendServiceLog sends a servicelog notification for the given alert in fleetmode
func (o *ocmsdkclient) SendServiceLogInFleetMode(n *oav1alpha1.FleetNotification, firing bool, clusterID string) error {
	req := o.ocm.Post()
	err := arguments.ApplyPathArg(req, "/api/service_logs/v1/cluster_logs")
	if err != nil {
		return err
	}

	sl := ocm.ServiceLog{
		ServiceName:  consts.ServiceLogServiceName,
		ClusterUUID:  clusterID,
		Summary:      n.Summary,
		InternalOnly: false,
	}

	// Use different Summary and Description for firing and resolved status for an alert
	if firing {
		sl.Description = n.NotificationMessage
		sl.Summary = ServiceLogActivePrefix + ": " + n.Summary
	}
	slAsBytes, err := json.Marshal(sl)
	if err != nil {
		return err
	}

	req.Bytes(slAsBytes)

	res, err := req.Send()
	if err != nil {
		return err
	}

	operationId := res.Header(HeaderOperationId)
	err = responseChecker(operationId, res.Status(), res.Bytes())
	if err != nil {
		return err
	}

	return nil
}

// responseChecker checks the ocm response returns error or not
func responseChecker(opId string, statusCode int, asBytes []byte) error {
	if statusCode == http.StatusCreated {
		log.WithField(LogFieldPostServiceLogOpId, opId).Info("service log sent succeeded")
		return nil
	}

	var ocmRes OCMResponseBody
	err := json.Unmarshal(asBytes, &ocmRes)
	if err != nil {
		return err
	}

	log.WithFields(log.Fields{LogFieldPostServiceLogOpId: opId, LogFieldPostServiceLogFailedReason: ocmRes.Reason}).Error("service log sent failed")

	switch statusCode {
	case http.StatusBadRequest:
		return fmt.Errorf("validation errors occurred")
	case http.StatusUnauthorized:
		return fmt.Errorf("invalid auth token")
	case http.StatusForbidden:
		return fmt.Errorf("unauthorized to perform operation")
	case http.StatusInternalServerError:
		return fmt.Errorf("internal server error")
	default:
		return fmt.Errorf("unknown Service Log return code")
	}
}
