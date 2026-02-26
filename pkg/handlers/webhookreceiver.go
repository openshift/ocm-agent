package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/openshift/ocm-agent/pkg/config"
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

type notificationRetriever struct {
	ctx                                       context.Context
	kubeCli                                   client.Client
	notificationNameToManagedNotificationName map[string]string
}

func newNotificationRetriever(kubeCli client.Client, ctx context.Context) (*notificationRetriever, error) {
	managedNotificationList := &oav1alpha1.ManagedNotificationList{}
	listOptions := []client.ListOption{
		client.InNamespace(OCMAgentNamespaceName),
	}

	err := kubeCli.List(ctx, managedNotificationList, listOptions...)
	if err != nil {
		log.WithError(err).Error("unable to list managed notifications")
		return nil, err
	}

	result := &notificationRetriever{ctx, kubeCli, make(map[string]string)}

	for _, managedNotification := range managedNotificationList.Items {
		for _, notification := range managedNotification.Spec.Notifications {
			result.notificationNameToManagedNotificationName[notification.Name] = managedNotification.Name
		}
	}

	return result, nil
}

// retrieveNotificationContext returns the notification from the ManagedNotification bundle if one exists, or error if one does not
func (r *notificationRetriever) retrieveNotificationContext(notificationName string) (*notificationContext, error) {
	managedNotificationName, ok := r.notificationNameToManagedNotificationName[notificationName]
	if !ok {
		return nil, fmt.Errorf("no managed notification found for notification %s", notificationName)
	}

	managedNotification := &oav1alpha1.ManagedNotification{}

	err := r.kubeCli.Get(r.ctx, client.ObjectKey{
		Namespace: OCMAgentNamespaceName,
		Name:      managedNotificationName,
	}, managedNotification)
	if err != nil {
		return nil, err
	}

	notification, err := managedNotification.GetNotificationForName(notificationName)
	if err != nil {
		return nil, err
	}
	if notification == nil {
		return nil, fmt.Errorf("managed notification %s does not contain notification %s anymore", managedNotificationName, notificationName)
	}

	var notificationRecord *oav1alpha1.NotificationRecord
	if managedNotification.Status.HasNotificationRecord(notificationName) {
		notificationRecord, err = managedNotification.Status.GetNotificationRecord(notificationName)
		if err != nil {
			return nil, err
		}
		if notificationRecord == nil {
			return nil, fmt.Errorf("managed notification %s does not have its status contain notification record %s", managedNotificationName, notificationName)
		}
	} else {
		notificationRecord = &oav1alpha1.NotificationRecord{
			Name:                notificationName,
			ServiceLogSentCount: 0,
		}
	}

	firingCondition := notificationRecord.Conditions.GetCondition(oav1alpha1.ConditionAlertFiring)
	return &notificationContext{
		retriever:           r,
		notification:        notification,
		managedNotification: managedNotification,
		notificationRecord:  notificationRecord,
		wasFiring:           firingCondition != nil && firingCondition.Status == corev1.ConditionTrue,
	}, nil
}

func (h *WebhookReceiverHandler) processAMReceiver(d AMReceiverData, ctx context.Context) *AMReceiverResponse {
	log.WithField("AMReceiverData", fmt.Sprintf("%+v", d)).Info("Process alert data")

	notificationRetriever, err := newNotificationRetriever(h.c, ctx)
	if err != nil {
		return &AMReceiverResponse{Error: err, Status: "unable to retrieve managed notifications", Code: http.StatusInternalServerError}
	}

	// Handle each firing alert
	for _, alert := range d.Alerts.Firing() {
		err := h.processAlert(alert, notificationRetriever, true)
		if err != nil {
			log.WithError(err).Error("a firing alert could not be successfully processed")
		}
	}

	// Handle resolved alerts
	for _, alert := range d.Alerts.Resolved() {
		err := h.processAlert(alert, notificationRetriever, false)
		if err != nil {
			log.WithError(err).Error("a resolved alert could not be successfully processed")
		}
	}
	return &AMReceiverResponse{Error: nil, Status: "ok", Code: http.StatusOK}
}

type notificationContext struct {
	retriever           *notificationRetriever
	notification        *oav1alpha1.Notification
	managedNotification *oav1alpha1.ManagedNotification // The custom resource wrapping the notification among others
	notificationRecord  *oav1alpha1.NotificationRecord  // The notification status
	wasFiring           bool
}

func (c *notificationContext) canSendServiceLog(isCurrentlyFiring bool) bool {
	nowTime := time.Now()
	slSentCondition := c.notificationRecord.Conditions.GetCondition(oav1alpha1.ConditionServiceLogSent)

	// If alert is firing
	if isCurrentlyFiring {
		// No SL sent yet -> send a SL
		if slSentCondition == nil {
			return true
		}

		// Check if we are within the "don't resend" time window; if so -> nothing to do
		nextAllowedSendTime := slSentCondition.LastTransitionTime.Add(time.Duration(c.notification.ResendWait) * time.Hour)
		if slSentCondition.Status == corev1.ConditionTrue && nowTime.Before(nextAllowedSendTime) {
			return false
		}

		// Change of state -> send a SL
		if !c.wasFiring {
			return true
		}

		// We use the AlertResolved condition as, unlike the AlertFiring condition, the timestamp for this
		// condition is updated when the webhook is called and the alert is already firing.
		// The timestamp for the AlertFiring condition is only updated when the condition status toggles.
		resolvedCondition := c.notificationRecord.Conditions.GetCondition(oav1alpha1.ConditionAlertResolved)

		// While AlertResolved condition is tested and set in an atomic way; ServiceLogSent condition is not atomically managed.
		// ServiceLogSent may be udated up to 2 minutes after the AlertResolved condition is updated (that's the max allowed time to send a SL)
		// If we are in this time window; this means, we are currently already trying to send the SL -> nothing to do
		if resolvedCondition != nil {
			lastWebhookCallTime := resolvedCondition.LastTransitionTime

			if nowTime.Before(lastWebhookCallTime.Add(3 * time.Minute)) {
				return false
			}
		}
	} else {
		// No resolved body -> nothing to do
		if len(c.notification.ResolvedDesc) == 0 {
			return false
		}

		// No change of state -> nothing to do
		if !c.wasFiring {
			return false
		}

		// We use the AlertFiring condition as, unlike the AlertResolved condition, its timestamp is only updated
		// when the condition status toggles.
		firingCondition := c.notificationRecord.Conditions.GetCondition(oav1alpha1.ConditionAlertFiring)

		// No SL sent when the alert was firing -> nothing to do
		if slSentCondition == nil || firingCondition == nil || slSentCondition.Status != corev1.ConditionTrue || slSentCondition.LastTransitionTime.Before(firingCondition.LastTransitionTime) {
			return false
		}
	}

	return true
}

func (c *notificationContext) setCondition(condType oav1alpha1.NotificationConditionType, reason string, boolStatus bool, updateTime bool, nowTime *v1.Time) {
	var status corev1.ConditionStatus
	if boolStatus {
		status = corev1.ConditionTrue
	} else {
		status = corev1.ConditionFalse
	}

	condTime := nowTime
	if !updateTime {
		currentCondition := c.notificationRecord.Conditions.GetCondition(condType)
		if currentCondition != nil {
			condTime = currentCondition.LastTransitionTime
		}
	}

	_ = c.notificationRecord.SetStatus(condType, reason, status, condTime)
}

func (c *notificationContext) updateFiringAndResolvedConditions(isCurrentlyFiring bool) error {
	nowTime := &v1.Time{Time: time.Now()}
	var firingReason, resolvedReason string

	if isCurrentlyFiring {
		firingReason = "Alert is firing"
		resolvedReason = "Alert has not resolved"
	} else {
		firingReason = "Alert is not firing"
		resolvedReason = "Alert has resolved"
	}

	hasStateChanged := isCurrentlyFiring != c.wasFiring

	c.setCondition(oav1alpha1.ConditionAlertFiring, firingReason, isCurrentlyFiring, hasStateChanged, nowTime)
	c.setCondition(oav1alpha1.ConditionAlertResolved, resolvedReason, !isCurrentlyFiring, hasStateChanged || isCurrentlyFiring, nowTime)

	c.managedNotification.Status.NotificationRecords.SetNotificationRecord(*c.notificationRecord)

	return c.retriever.kubeCli.Status().Update(c.retriever.ctx, c.managedNotification)
}

func (c *notificationContext) updateServiceLogSentCondition(isCurrentlyFiring, hasSLBeenSent bool) error {
	var slSentReason string

	if isCurrentlyFiring {
		if c.wasFiring {
			if !hasSLBeenSent {
				return nil
			}
			slSentReason = "Service log sent again after the resend window passed"
		} else {
			slSentReason = "Service log sent for alert firing"
		}
	} else {
		slSentReason = "Service log sent for alert resolved"
	}

	c.setCondition(oav1alpha1.ConditionServiceLogSent, slSentReason, hasSLBeenSent, true, &v1.Time{Time: time.Now()})

	c.managedNotification.Status.NotificationRecords.SetNotificationRecord(*c.notificationRecord)

	return c.retriever.kubeCli.Status().Update(c.retriever.ctx, c.managedNotification)
}

func (c *notificationContext) sendServiceLog(ocmCli ocm.OCMClient, alert template.Alert, isCurrentlyFiring bool) error {
	// Send the servicelog for the alert
	log.WithFields(log.Fields{LogFieldNotificationName: c.notification.Name}).Info("will send service log")

	slErr := ocm.BuildAndSendServiceLog(
		ocm.NewServiceLogBuilder(c.notification.Summary, c.notification.ActiveDesc, c.notification.ResolvedDesc, viper.GetString(config.ExternalClusterID), c.notification.Severity, c.notification.LogType, c.notification.References),
		isCurrentlyFiring, &alert, ocmCli)

	err := c.updateServiceLogSentCondition(isCurrentlyFiring, slErr == nil)
	if err != nil {
		log.WithFields(log.Fields{LogFieldNotificationName: c.notification.Name, LogFieldManagedNotification: c.managedNotification.Name}).WithError(err).Error("unable to update ServiceLogSent condition")
	}

	return slErr
}

// processAlert handles the pre-check verification and sending of a notification for a particular alert
// and returns an error if that process completed successfully or false otherwise
func (h *WebhookReceiverHandler) processAlert(alert template.Alert, notificationRetriever *notificationRetriever, isCurrentlyFiring bool) error {
	// Should this alert be handled?
	if !isValidAlert(alert, false) {
		log.WithField(LogFieldAlert, fmt.Sprintf("%+v", alert)).Info("alert does not meet valid criteria")
		return fmt.Errorf("alert does not meet valid criteria")
	}

	// Can the alert be mapped to an existing notification definition?
	notificationName := alert.Labels[AMLabelTemplateName]
	if _, ok := notificationRetriever.notificationNameToManagedNotificationName[notificationName]; !ok {
		log.WithField(LogFieldAlert, fmt.Sprintf("%+v", alert)).Warning("an alert fired with no associated notification")
		return fmt.Errorf("an alert fired with no associated notification")
	}

	var c *notificationContext
	var canSend bool

	// Critical section: AlertFiring and AlertResolved conditions are read and set/updated in an atomic way
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var err error

		c, err = notificationRetriever.retrieveNotificationContext(notificationName)
		if err != nil {
			return err
		}

		// Has a servicelog already been sent and we are within the notification's "do-not-resend" window?
		canSend = c.canSendServiceLog(isCurrentlyFiring)

		return c.updateFiringAndResolvedConditions(isCurrentlyFiring)
	})
	if err != nil {
		return err
	}

	if !canSend {
		if isCurrentlyFiring {
			log.WithFields(log.Fields{"notification": notificationName,
				LogFieldResendInterval: c.notification.ResendWait,
			}).Info("not sending a notification as one was already sent recently")
			// Reset the metric for correct service log response from OCM
			metrics.ResetResponseMetricFailure(config.ServiceLogService, notificationName, alert.Labels["alertname"])
		} else {
			log.WithFields(log.Fields{"notification": notificationName}).Info("not sending a resolve notification if it was not firing or resolved body is empty")
		}
		// This is not an error state
		return nil
	}

	err = c.sendServiceLog(h.ocm, alert, isCurrentlyFiring)
	if err != nil {
		log.WithError(err).WithFields(log.Fields{LogFieldNotificationName: notificationName, LogFieldIsFiring: isCurrentlyFiring}).Error("unable to send a service log")

		// Set the metric for failed service log response from OCM
		metrics.SetResponseMetricFailure(config.ServiceLogService, notificationName, alert.Labels["alertname"])
		metrics.CountFailedServiceLogs(notificationName)
		return err
	}

	// Reset the metric for correct service log response from OCM
	metrics.ResetResponseMetricFailure(config.ServiceLogService, notificationName, alert.Labels["alertname"])

	// Count the service log sent by the template name
	if isCurrentlyFiring {
		metrics.CountServiceLogSent(notificationName, "firing")
	} else {
		metrics.CountServiceLogSent(notificationName, "resolved")
	}

	metrics.SetTotalServiceLogCount(notificationName, c.notificationRecord.ServiceLogSentCount)

	return nil
}
