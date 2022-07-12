package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/openshift-online/ocm-cli/pkg/arguments"
	sdk "github.com/openshift-online/ocm-sdk-go"
	"github.com/prometheus/alertmanager/template"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	oav1alpha1 "github.com/openshift/ocm-agent-operator/pkg/apis/ocmagent/v1alpha1"
	"github.com/openshift/ocm-agent/pkg/config"
	"github.com/openshift/ocm-agent/pkg/consts"
	"github.com/openshift/ocm-agent/pkg/metrics"
	"github.com/openshift/ocm-agent/pkg/ocm"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/util/retry"
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

// OCMClient enables implementation of OCM Client
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

// Alert Manager receiver response
type AMReceiverResponse struct {
	Error  error
	Code   int
	Status string
}

// Use prometheus alertmanager template type for post data
type AMReceiverData template.Data

type AMReceiverAlert template.Alert

func NewWebhookReceiverHandler(c client.Client, o OCMClient) *WebhookReceiverHandler {
	return &WebhookReceiverHandler{
		c:   c,
		ocm: o,
	}
}

func NewOcmClient(ocm *sdk.Connection) OCMClient {
	return &ocmsdkclient{
		ocm: ocm,
	}
}

func (h *WebhookReceiverHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// validate request
	if r != nil && r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	var err error
	var alertData AMReceiverData
	err = json.NewDecoder(r.Body).Decode(&alertData)
	if err != nil {
		log.Errorf("Failed to process request body: %s\n", err)
		http.Error(w, "Bad request body", http.StatusBadRequest)
		metrics.SetRequestMetricFailure(consts.WebhookReceiverPath)
		return
	}

	// process request
	response := h.processAMReceiver(alertData, r.Context())

	// write response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(response.Code)
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		log.Errorf("Failed to write to response: %s\n", err)
		http.Error(w, "Failed to write to response", http.StatusInternalServerError)
		metrics.SetRequestMetricFailure(consts.WebhookReceiverPath)
		return
	}

	metrics.ResetMetric(metrics.MetricRequestFailure)
}

func (h *WebhookReceiverHandler) processAMReceiver(d AMReceiverData, ctx context.Context) *AMReceiverResponse {
	log.WithField("AMReceiverData", fmt.Sprintf("%+v", d)).Info("Process alert data")

	// Let's get all ManagedNotifications in the
	mnl := &oav1alpha1.ManagedNotificationList{}
	listOptions := []client.ListOption{
		client.InNamespace("openshift-ocm-agent-operator"),
	}
	err := h.c.List(ctx, mnl, listOptions...)
	if err != nil {
		log.WithError(err).Error("unable to list managed notifications")
		return &AMReceiverResponse{Error: err, Status: "unable to list managed notifications", Code: http.StatusInternalServerError}
	}

	// Handle each firing alert
	for _, alert := range d.Alerts.Firing() {
		err = h.processAlert(alert, mnl, true)
		if err != nil {
			log.WithError(err).Error("a firing alert could not be successfully processed")
		}
	}

	// Handle resolved alerts
	for _, alert := range d.Alerts.Resolved() {
		err = h.processAlert(alert, mnl, false)
		if err != nil {
			log.WithError(err).Error("a resolved alert could not be successfully processed")
		}
	}

	return &AMReceiverResponse{Error: nil, Status: "ok", Code: http.StatusOK}
}

// processAlert handles the pre-check verification and sending of a notification for a particular alert
// and returns an error if that process completed successfully or false otherwise
func (h *WebhookReceiverHandler) processAlert(alert template.Alert, mnl *oav1alpha1.ManagedNotificationList, firing bool) error {
	// Should this alert be handled?
	if !isValidAlert(alert) {
		log.WithField(LogFieldAlert, fmt.Sprintf("%+v", alert)).Info("alert does not meet valid criteria")
		return fmt.Errorf("alert does not meet valid criteria")
	}

	// Can the alert be mapped to an existing notification definition?
	notification, managedNotifications, err := getNotification(alert.Labels[AMLabelTemplateName], mnl)
	if err != nil {
		log.WithError(err).WithField(LogFieldAlert, fmt.Sprintf("%+v", alert)).Warning("an alert fired with no associated notification template definition")
		return err
	}

	// Has a servicelog already been sent and we are within the notification's "do-not-resend" window?
	canBeSent, err := managedNotifications.CanBeSent(notification.Name, firing)
	if err != nil {
		log.WithError(err).WithField(LogFieldNotificationName, notification.Name).Error("unable to validate if notification can be sent")
		return err
	}
	if !canBeSent {
		if firing {
			log.WithFields(log.Fields{"notification": notification.Name,
				LogFieldResendInterval: notification.ResendWait,
			}).Info("not sending a notification as one was already sent recently")
		} else {
			log.WithFields(log.Fields{"notification": notification.Name}).Info("not sending a resolve notification if it was not firing")
		}
		// This is not an error state
		return nil
	}

	// Send the servicelog for the alert
	log.WithFields(log.Fields{LogFieldNotificationName: notification.Name}).Info("will send servicelog for notification")

	err = h.ocm.SendServiceLog(notification, firing)
	if err != nil {
		log.WithError(err).WithFields(log.Fields{LogFieldNotificationName: notification.Name, LogFieldIsFiring: true}).Error("unable to send a notification")
		metrics.SetResponseMetricFailure("service_logs")
		return err
	}
	// Reset the metric if we got correct Response from OCM
	metrics.ResetMetric(metrics.MetricResponseFailure)

	// Count the service log sent by the template name
	if firing {
		metrics.CountServiceLogSent(notification.Name, "firing")
	} else {
		metrics.CountServiceLogSent(notification.Name, "resolved")
	}

	// Update the notification status to indicate a servicelog has been sent
	m, err := h.updateNotificationStatus(notification, managedNotifications, firing)
	if err != nil {
		log.WithFields(log.Fields{LogFieldNotificationName: notification.Name, LogFieldManagedNotification: managedNotifications.Name}).WithError(err).Error("unable to update notification status")
		return err
	}

	status, err := m.Status.GetNotificationRecord(notification.Name)
	if err != nil {
		return err
	}

	metrics.SetTotalServiceLogCount(notification.Name, status.ServiceLogSentCount)

	return nil
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

// getNotification returns the notification from the ManagedNotification bundle if one exists, or error if one does not
func getNotification(name string, m *oav1alpha1.ManagedNotificationList) (*oav1alpha1.Notification, *oav1alpha1.ManagedNotification, error) {
	for _, mn := range m.Items {
		notification, err := mn.GetNotificationForName(name)
		if notification != nil && err == nil {
			return notification, &mn, nil
		}
	}
	return nil, nil, fmt.Errorf("matching managed notification not found for %s", name)
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

type OCMResponseBody struct {
	Reason string `json:"reason"`
}

func (h *WebhookReceiverHandler) updateNotificationStatus(n *oav1alpha1.Notification, mn *oav1alpha1.ManagedNotification, firing bool) (*oav1alpha1.ManagedNotification, error) {
	var m *oav1alpha1.ManagedNotification

	// Update lastSent timestamp
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		m = &oav1alpha1.ManagedNotification{}

		err := h.c.Get(context.TODO(), client.ObjectKey{
			Namespace: mn.Namespace,
			Name:      mn.Name,
		}, m)
		if err != nil {
			return err
		}

		timeNow := &v1.Time{Time: time.Now()}
		status, err := m.Status.GetNotificationRecord(n.Name)
		if err != nil {
			// Status does not exist, create it
			// This will happen only when the alert is firing first time
			status = &oav1alpha1.NotificationRecord{
				Name:                n.Name,
				ServiceLogSentCount: 0,
			}
			_ = status.SetStatus(oav1alpha1.ConditionAlertFiring, "Alert starts firing", corev1.ConditionTrue, timeNow)
			_ = status.SetStatus(oav1alpha1.ConditionAlertResolved, "Alert has not resolved", corev1.ConditionFalse, timeNow)
			_ = status.SetStatus(oav1alpha1.ConditionServiceLogSent, "Service log sent for firing alert", corev1.ConditionTrue, timeNow)
		} else {
			// Status exists, update it
			// When the alert is already firing
			firingCondition := status.Conditions.GetCondition(oav1alpha1.ConditionAlertFiring).Status
			if firingCondition == corev1.ConditionTrue {
				firedConditionTime := status.Conditions.GetCondition(oav1alpha1.ConditionAlertFiring).LastTransitionTime
				resolvedConditionTime := status.Conditions.GetCondition(oav1alpha1.ConditionAlertResolved).LastTransitionTime
				if firing {
					// Status transition is Firing to Firing
					// Do not update the condition for AlertFiring and AlertResolved
					// Only update the timestamp for the ServiceLogSent
					_ = status.SetStatus(oav1alpha1.ConditionAlertFiring, "Alert is still firing", corev1.ConditionTrue, firedConditionTime)
					_ = status.SetStatus(oav1alpha1.ConditionAlertResolved, "Alert has not resolved", corev1.ConditionFalse, resolvedConditionTime)
					_ = status.SetStatus(oav1alpha1.ConditionServiceLogSent, "Service log sent again after the resend window passed", corev1.ConditionTrue, timeNow)
				} else {
					// Status transition is Firing to Resolved
					// Update the condition status and timestamp for AlertFiring
					// Update the condition status and timestamp for AlertResolved
					// Update the timestamp for the ServiceLogSent
					_ = status.SetStatus(oav1alpha1.ConditionAlertFiring, "Alert is not firing", corev1.ConditionFalse, timeNow)
					_ = status.SetStatus(oav1alpha1.ConditionAlertResolved, "Alert resolved", corev1.ConditionTrue, timeNow)
					_ = status.SetStatus(oav1alpha1.ConditionServiceLogSent, "Service log sent for alert resolved", corev1.ConditionTrue, timeNow)
				}
			} else {
				// Status transition is Resolved to Firing
				// Update the condition status and timestamp for AlertFiring
				// Update the condition status and timestamp for AlertResolved
				// Update the timestamp for the ServiceLogSent
				_ = status.SetStatus(oav1alpha1.ConditionAlertFiring, "Alert fired again", corev1.ConditionTrue, timeNow)
				_ = status.SetStatus(oav1alpha1.ConditionAlertResolved, "Alert has not resolved", corev1.ConditionFalse, timeNow)
				_ = status.SetStatus(oav1alpha1.ConditionServiceLogSent, "Service log sent for alert firing", corev1.ConditionTrue, timeNow)
			}
		}

		m.Status.NotificationRecords.SetNotificationRecord(*status)

		err = h.c.Status().Update(context.TODO(), m)

		return err
	})

	return m, err
}
