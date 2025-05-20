package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/openshift/ocm-agent/pkg/config"
	"github.com/openshift/ocm-agent/pkg/httpchecker"
	"github.com/openshift/ocm-agent/pkg/ocm"
	"github.com/spf13/viper"

	"github.com/prometheus/alertmanager/template"
	log "github.com/sirupsen/logrus"

	oav1alpha1 "github.com/openshift/ocm-agent-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/ocm-agent/pkg/consts"
	"github.com/openshift/ocm-agent/pkg/metrics"

	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewWebhookReceiverHandler(c client.Client, o ocm.OCMClient) *WebhookReceiverHandler {
	return &WebhookReceiverHandler{
		c:   c,
		ocm: o,
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

	// Let's get all ManagedNotifications
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
		err := h.processAlert(alert, mnl, false)
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
	if !isValidAlert(alert, false) {
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
			// Reset the metric for correct service log response from OCM
			metrics.ResetResponseMetricFailure(config.ServiceLogService, notification.Name, alert.Labels["alertname"])
		} else {
			log.WithFields(log.Fields{"notification": notification.Name}).Info("not sending a resolve notification if it was not firing or resolved body is empty")
			s, err := managedNotifications.Status.GetNotificationRecord(notification.Name)
			// If a status history exists but can't be fetched, this is an irregular situation
			if err != nil {
				return err
			}
			firingStatus := s.Conditions.GetCondition(oav1alpha1.ConditionAlertFiring).Status
			if firingStatus == corev1.ConditionTrue {
				// Update the notification status for the resolved alert without sending resolved SL
				_, err := h.updateNotificationStatus(notification, managedNotifications, firing, corev1.ConditionTrue)
				if err != nil {
					log.WithFields(log.Fields{LogFieldNotificationName: notification.Name, LogFieldManagedNotification: managedNotifications.Name}).WithError(err).Error("unable to update notification status")
					return err
				}
			}
		}
		// This is not an error state
		return nil
	}

	var attempts int = 3
	var sleep time.Duration = 30 * time.Second
	ocmURL := viper.GetString(config.OcmURL)
	if ocmURL == "" {
		return fmt.Errorf("OCM URL is missing or empty in the configuration")
	}
	err = checkURLWithRetries(ocmURL, attempts, sleep)
	if err != nil {
		return err
	}

	// Send the servicelog for the alert
	log.WithFields(log.Fields{LogFieldNotificationName: notification.Name}).Info("will send servicelog for notification")
	slerr := ocm.BuildAndSendServiceLog(
		ocm.NewServiceLogBuilder(notification.Summary, notification.ActiveDesc, notification.ResolvedDesc, viper.GetString(config.ExternalClusterID), notification.Severity, notification.LogType, notification.References),
		firing, &alert, h.ocm)
	if slerr != nil {
		log.WithError(err).WithFields(log.Fields{LogFieldNotificationName: notification.Name, LogFieldIsFiring: true}).Error("unable to send a notification")
		_, err := h.updateNotificationStatus(notification, managedNotifications, firing, corev1.ConditionFalse)
		if err != nil {
			log.WithFields(log.Fields{LogFieldNotificationName: notification.Name, LogFieldManagedNotification: managedNotifications.Name}).WithError(err).Error("unable to update notification status")
		}
		// Set the metric for failed service log response from OCM
		metrics.SetResponseMetricFailure(config.ServiceLogService, notification.Name, alert.Labels["alertname"])
		metrics.CountFailedServiceLogs(notification.Name)
		return slerr
	}

	// Reset the metric for correct service log response from OCM
	metrics.ResetResponseMetricFailure(config.ServiceLogService, notification.Name, alert.Labels["alertname"])

	// Count the service log sent by the template name
	if firing {
		metrics.CountServiceLogSent(notification.Name, "firing")
	} else {
		metrics.CountServiceLogSent(notification.Name, "resolved")
	}
	// Update the notification status to indicate a servicelog has been sent
	m, err := h.updateNotificationStatus(notification, managedNotifications, firing, corev1.ConditionTrue)
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

// checkURLWithRetries returns err for response code outside >=200 and <300
func checkURLWithRetries(url string, attempts int, sleep time.Duration) error {
	// Use the default HTTP client
	urlchecker := httpchecker.NewHTTPChecker(nil)
	err := httpchecker.Reattempt(attempts, sleep, func() error {
		return urlchecker.UrlAvailabilityCheck(url)
	})
	if err != nil {
		return err
	}
	return nil
}

func (h *WebhookReceiverHandler) updateNotificationStatus(n *oav1alpha1.Notification, mn *oav1alpha1.ManagedNotification, firing bool, slsentstatus corev1.ConditionStatus) (*oav1alpha1.ManagedNotification, error) {
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
			_ = status.SetStatus(oav1alpha1.ConditionServiceLogSent, "Service log sent for firing alert", slsentstatus, timeNow)
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
					_ = status.SetStatus(oav1alpha1.ConditionServiceLogSent, "Service log sent again after the resend window passed", slsentstatus, timeNow)
				} else {
					// Status transition is Firing to Resolved
					// Update the condition status and timestamp for AlertFiring
					// Update the condition status and timestamp for AlertResolved
					// Update the timestamp for the ServiceLogSent
					_ = status.SetStatus(oav1alpha1.ConditionAlertFiring, "Alert is not firing", corev1.ConditionFalse, timeNow)
					_ = status.SetStatus(oav1alpha1.ConditionAlertResolved, "Alert resolved", corev1.ConditionTrue, timeNow)
					if len(n.ResolvedDesc) > 0 {
						_ = status.SetStatus(oav1alpha1.ConditionServiceLogSent, "Service log sent for alert resolved", slsentstatus, timeNow)
					} else {
						// This is for the total serviceLogSentCount while should not be increased by SetNotificationRecord if resolved SL is not sent
						status.ServiceLogSentCount--
					}
				}
			} else {
				// Status transition is Resolved to Firing
				// Update the condition status and timestamp for AlertFiring
				// Update the condition status and timestamp for AlertResolved
				// Update the timestamp for the ServiceLogSent
				_ = status.SetStatus(oav1alpha1.ConditionAlertFiring, "Alert fired again", corev1.ConditionTrue, timeNow)
				_ = status.SetStatus(oav1alpha1.ConditionAlertResolved, "Alert has not resolved", corev1.ConditionFalse, timeNow)
				_ = status.SetStatus(oav1alpha1.ConditionServiceLogSent, "Service log sent for alert firing", slsentstatus, timeNow)
			}
		}

		m.Status.NotificationRecords.SetNotificationRecord(*status)

		err = h.c.Status().Update(context.TODO(), m)

		return err
	})

	return m, err
}
