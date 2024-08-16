package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/prometheus/alertmanager/template"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openshift/ocm-agent/pkg/ocm"

	_ "github.com/golang/mock/mockgen/model"
)

const (
	AMLabelAlertName           = "alertname"
	AMLabelTemplateName        = "managed_notification_template"
	AMLabelManagedNotification = "send_managed_notification"
	AMLabelAlertMCID           = "_mc_id"
	AMLabelAlertHCID           = "_id"

	LogFieldNotificationName           = "notification"
	LogFieldNotificationRecordName     = "notification_record"
	LogFieldResendInterval             = "resend_interval"
	LogFieldAlertname                  = "alertname"
	LogFieldAlert                      = "alert"
	LogFieldIsFiring                   = "is_firing"
	LogFieldManagedNotification        = "managed_notification_cr"
	LogFieldPostServiceLogOpId         = "post_servicelog_operation_id"
	LogFieldPostServiceLogFailedReason = "post_servicelog_failed_reason"

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

type WebhookReceiverHandler struct {
	c   client.Client
	ocm ocm.OCMClient
}

type OCMResponseBody struct {
	Reason string `json:"reason"`
}

// isValidAlert indicates whether the supplied alert is one that warrants being processed for a notification.
// Any or all of these situations should be treated as an error as it indicates that AlertManager is forwarding
// alerts to ocm-agent that it should not be.
func isValidAlert(alert template.Alert, fleetMode bool) bool {
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

	if fleetMode {
		// An alert in fleet mode must have a management cluster ID label
		if _, ok := alert.Labels[AMLabelAlertMCID]; !ok {
			log.WithField(LogFieldAlertname, alertname).Error("fleet mode alert has no management cluster ID")
			return false
		}

		// An alert in fleet mode must have a hosted cluster ID label
		if _, ok := alert.Labels[AMLabelAlertHCID]; !ok {
			log.WithField(LogFieldAlertname, alertname).Error("fleet mode alert has no hosted cluster ID")
			return false
		}
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
