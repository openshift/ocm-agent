package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/prometheus/alertmanager/template"
	log "github.com/sirupsen/logrus"

	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	oav1alpha1 "github.com/openshift/ocm-agent-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"

	"github.com/openshift/ocm-agent/pkg/config"
	"github.com/openshift/ocm-agent/pkg/consts"
	"github.com/openshift/ocm-agent/pkg/metrics"
	"github.com/openshift/ocm-agent/pkg/ocm"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	OCMAgentNamespaceName = "openshift-ocm-agent-operator"
)

var (
	// We need a solid backoff duration and jitter as we expect a lot of webhooks
	// to be executed at the exact same time when an alert initially is created.
	retryConfig = wait.Backoff{
		Steps:    5,
		Duration: 2 * time.Second,
		Factor:   1.0,
		Jitter:   5,
	}

	customIs409 = func(err error) bool { return errors.IsConflict(err) || errors.IsAlreadyExists(err) }
)

type WebhookRHOBSReceiverHandler struct {
	c   client.Client
	ocm ocm.OCMClient
}

func NewWebhookRHOBSReceiverHandler(c client.Client, o ocm.OCMClient) *WebhookRHOBSReceiverHandler {
	return &WebhookRHOBSReceiverHandler{
		c:   c,
		ocm: o,
	}
}

func (h *WebhookRHOBSReceiverHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

func (h *WebhookRHOBSReceiverHandler) processAMReceiver(d AMReceiverData, ctx context.Context) *AMReceiverResponse {
	log.WithField("AMReceiverData", fmt.Sprintf("%+v", d)).Info("Process alert data")

	// Handle each firing alert
	for _, alert := range d.Alerts.Firing() {
		err := h.processAlert(alert, true)
		if err != nil {
			log.WithError(err).Error("a firing alert could not be successfully processed")
		}
	}

	// Handle resolved alerts
	for _, alert := range d.Alerts.Resolved() {
		err := h.processAlert(alert, false)
		if err != nil {
			log.WithError(err).Error("a resolved alert could not be successfully processed")
		}
	}

	return &AMReceiverResponse{Error: nil, Status: "ok", Code: http.StatusOK}
}

type fleetNotificationRetriever struct {
	ctx                 context.Context
	kubeCli             client.Client
	fleetNotification   *oav1alpha1.FleetNotification
	managementClusterID string
	hostedClusterID     string
}

func newFleetNotificationRetriever(kubeCli client.Client, ctx context.Context, alert template.Alert) (*fleetNotificationRetriever, error) {
	managedFleetNotificationName := alert.Labels[AMLabelTemplateName]
	managedFleetNotification := &oav1alpha1.ManagedFleetNotification{}
	err := kubeCli.Get(ctx, client.ObjectKey{
		Namespace: OCMAgentNamespaceName,
		Name:      managedFleetNotificationName,
	}, managedFleetNotification)
	if err != nil {
		log.WithError(err).Error("unable to locate corresponding notification template")
		return nil, err
	}

	return &fleetNotificationRetriever{
		ctx:                 ctx,
		kubeCli:             kubeCli,
		fleetNotification:   &managedFleetNotification.Spec.FleetNotification,
		managementClusterID: alert.Labels[AMLabelAlertMCID],
		hostedClusterID:     alert.Labels[AMLabelAlertHCID],
	}, nil
}

func (r *fleetNotificationRetriever) retrieveFleetNotificationContext() (*fleetNotificationContext, error) {
	managedFleetNotificationRecord := &oav1alpha1.ManagedFleetNotificationRecord{}
	err := r.kubeCli.Get(r.ctx, client.ObjectKey{
		Namespace: OCMAgentNamespaceName,
		Name:      r.managementClusterID,
	}, managedFleetNotificationRecord)

	if err != nil {
		if errors.IsNotFound(err) {
			// Record does not exist, attempt to create it
			managedFleetNotificationRecord = &oav1alpha1.ManagedFleetNotificationRecord{
				ObjectMeta: v1.ObjectMeta{
					Name:      r.managementClusterID,
					Namespace: OCMAgentNamespaceName,
				},
			}
			err = r.kubeCli.Create(r.ctx, managedFleetNotificationRecord)
		}
		if err != nil {
			return nil, err
		}
	}

	// Ideally, this field should have probably been part of the ManagedFleetNotificationRecord
	// definition, not the status.
	if managedFleetNotificationRecord.Status.ManagementCluster == "" {
		managedFleetNotificationRecord.Status.ManagementCluster = r.managementClusterID
	}

	notificationRecordItem, err := managedFleetNotificationRecord.GetNotificationRecordItem(r.managementClusterID, r.fleetNotification.Name, r.hostedClusterID)
	if err != nil {
		// Add the item it doesn't exist

		notificationRecordByName, err := managedFleetNotificationRecord.GetNotificationRecordByName(r.managementClusterID, r.fleetNotification.Name)
		if err != nil {
			notificationRecordByName = &oav1alpha1.NotificationRecordByName{
				NotificationName:        r.fleetNotification.Name,
				ResendWait:              r.fleetNotification.ResendWait,
				NotificationRecordItems: nil,
			}
			managedFleetNotificationRecord.Status.NotificationRecordByName = append(managedFleetNotificationRecord.Status.NotificationRecordByName, *notificationRecordByName)
		}

		notificationRecordItem, err = managedFleetNotificationRecord.AddNotificationRecordItem(r.hostedClusterID, notificationRecordByName)
		if err != nil {
			return nil, err
		}
	}

	wasClusterInLimitedSupport :=
		r.fleetNotification.LimitedSupport && // Sending limited support notifications in place of service logs
			notificationRecordItem.FiringNotificationSentCount > notificationRecordItem.ResolvedNotificationSentCount
		// Counters are identical when no limited support is active
		// Sent counter is higher than resolved counter by 1 when limited support is active
		// TODO(ngrauss): record the limited support reason ID in the NotificationRecordItem object to be able to
		// precisely track if limited support is active or not.

	return &fleetNotificationContext{
		retriever:                      r,
		managedFleetNotificationRecord: managedFleetNotificationRecord,
		notificationRecordItem:         notificationRecordItem,
		notificationRecordInitialValue: *notificationRecordItem,
		wasClusterInLimitedSupport:     wasClusterInLimitedSupport,
	}, nil
}

type fleetNotificationContext struct {
	retriever                          *fleetNotificationRetriever
	managedFleetNotificationRecord     *oav1alpha1.ManagedFleetNotificationRecord
	notificationRecordItem             *oav1alpha1.NotificationRecordItem
	notificationRecordInitialValue     oav1alpha1.NotificationRecordItem
	notificationRecordValueAfterUpdate oav1alpha1.NotificationRecordItem
	wasClusterInLimitedSupport         bool
}

func (c *fleetNotificationContext) canSendNotification() bool {
	nowTime := time.Now()

	// Cluster already in limited support -> nothing to do
	if c.wasClusterInLimitedSupport {
		log.WithFields(log.Fields{"notification": c.retriever.fleetNotification.Name}).Info("not sending a limited support notification as the previous one didn't resolve yet")
		return false
	}

	// No last transition time -> send a notification
	if c.notificationRecordItem.LastTransitionTime == nil {
		return true
	}

	var dontResendDuration time.Duration
	if c.retriever.fleetNotification.ResendWait > 0 {
		dontResendDuration = time.Duration(c.retriever.fleetNotification.ResendWait) * time.Hour
	} else {
		dontResendDuration = time.Duration(3) * time.Minute
	}

	// Check if we are within the "don't resend" time window; if so -> nothing to do
	nextAllowedSendTime := c.notificationRecordItem.LastTransitionTime.Add(dontResendDuration)
	return nowTime.After(nextAllowedSendTime)
}

func (c *fleetNotificationContext) inPlaceStatusUpdate() error {
	// c.notificationRecordItem is a pointer but it is not part of the managedFleetNotificationRecord object
	// Below code makes sure to update the oav1alpha1.NotificationRecordItem inside the managedFleetNotificationRecord object with the latest values.
	// TODO(ngrauss): refactor GetNotificationRecordItem method to return a reference to the object inside the managedFleetNotificationRecord
	for i, notificationRecordByName := range c.managedFleetNotificationRecord.Status.NotificationRecordByName {
		if notificationRecordByName.NotificationName != c.retriever.fleetNotification.Name {
			continue
		}
		for j, notificationRecordItem := range notificationRecordByName.NotificationRecordItems {
			if notificationRecordItem.HostedClusterID == c.retriever.hostedClusterID {
				c.managedFleetNotificationRecord.Status.NotificationRecordByName[i].NotificationRecordItems[j] = *c.notificationRecordItem
			}
		}
	}

	err := c.retriever.kubeCli.Status().Update(c.retriever.ctx, c.managedFleetNotificationRecord)
	if err != nil {
		log.WithFields(log.Fields{LogFieldNotificationRecordName: c.managedFleetNotificationRecord.Name}).Infof("update of managedfleetnotificationrecord failed: %s", err.Error())
		return err
	}
	return nil
}

func (c *fleetNotificationContext) updateNotificationStatus(isCurrentlyFiring, canSendNotification bool) error {
	if isCurrentlyFiring {
		if canSendNotification {
			c.notificationRecordItem.FiringNotificationSentCount++
			c.notificationRecordItem.LastTransitionTime = &v1.Time{Time: time.Now()}
		}
	} else if c.retriever.fleetNotification.LimitedSupport {
		c.notificationRecordItem.ResolvedNotificationSentCount = c.notificationRecordItem.FiringNotificationSentCount
	}

	c.notificationRecordValueAfterUpdate = *c.notificationRecordItem

	return c.inPlaceStatusUpdate()
}

// TODO(ngrauss): to be removed
// ManagedFleetNotificationRecord record item counters are currently updated before the notification is sent.
// This is because:
// - Those counters are also used to determine if the cluster is already in limitied support or not.
// - Determining this state needs to be done in an atomic way to avoid race conditions
// A new field about whether the alert is firing or not will be added to the NotificationRecordItem object
// to avoid this workaround.
func (c *fleetNotificationContext) restoreNotificationStatus() error {
	// Critical section: notification record item is read and set/updated in an atomic way

	fleetNotificationContext, err := c.retriever.retrieveFleetNotificationContext()
	if err != nil {
		return err
	}

	if *c.notificationRecordItem == c.notificationRecordValueAfterUpdate {
		*fleetNotificationContext.notificationRecordItem = c.notificationRecordInitialValue

		return fleetNotificationContext.inPlaceStatusUpdate()
	}

	return nil
}

func (c *fleetNotificationContext) sendNotification(ocmCli ocm.OCMClient, alert template.Alert) error {
	fleetNotification := c.retriever.fleetNotification
	hostedClusterID := c.retriever.hostedClusterID

	if fleetNotification.LimitedSupport { // Limited support case
		log.WithFields(log.Fields{LogFieldNotificationName: fleetNotification.Name}).Info("will send limited support for notification")
		builder := &cmv1.LimitedSupportReasonBuilder{}
		builder.Summary(fleetNotification.Summary)
		builder.Details(fleetNotification.NotificationMessage)
		builder.DetectionType(cmv1.DetectionTypeManual)
		reason, err := builder.Build()
		if err != nil {
			return fmt.Errorf("unable to build limited support for fleetnotification '%s' reason: %w", fleetNotification.Name, err)
		}
		err = ocmCli.SendLimitedSupport(hostedClusterID, reason)
		if err != nil {
			// Set the metric for failed limited support response from OCM
			return fmt.Errorf("limited support reason for fleetnotification '%s' could not be set for cluster %s, err: %w", fleetNotification.Name, hostedClusterID, err)
		}
	} else { // Service log case
		log.WithFields(log.Fields{LogFieldNotificationName: fleetNotification.Name}).Info("will send servicelog for notification")
		err := ocm.BuildAndSendServiceLog(
			ocm.NewServiceLogBuilder(fleetNotification.Summary, fleetNotification.NotificationMessage, "", hostedClusterID, fleetNotification.Severity, fleetNotification.LogType, fleetNotification.References),
			true, &alert, ocmCli)
		if err != nil {
			log.WithError(err).WithFields(log.Fields{LogFieldNotificationName: fleetNotification.Name, LogFieldIsFiring: true}).Error("unable to send service log for notification")

			return err
		}
	}

	return nil
}

func (c *fleetNotificationContext) removeLimitedSupport(ocmCli ocm.OCMClient) error {
	hostedClusterID := c.retriever.hostedClusterID

	limitedSupportReasons, err := ocmCli.GetLimitedSupportReasons(hostedClusterID)
	if err != nil {
		return fmt.Errorf("unable to get limited support reasons for cluster %s:, %w", hostedClusterID, err)
	}

	fleetNotification := c.retriever.fleetNotification

	for _, limitedSupportReason := range limitedSupportReasons {
		// If the reason matches the fleet notification LS reason, remove it
		// TODO(ngrauss): The limitedSupportReason.ID() should be stored in the ManagedFleetNotificationRecord record item object
		// and be given to this method; this would avoid:
		// 1. removing limited support reasons potentially not created by OCM Agent.
		// 2. do some kind of string matching which is prone to errors if the message format changes.
		if strings.Contains(limitedSupportReason.Details(), fleetNotification.NotificationMessage) {
			log.WithFields(log.Fields{LogFieldNotificationName: fleetNotification.Name}).Infof("will remove limited support reason '%s' for notification", limitedSupportReason.ID())
			err := ocmCli.RemoveLimitedSupport(hostedClusterID, limitedSupportReason.ID())
			if err != nil {
				return fmt.Errorf("limited support reason with ID '%s' couldn't be removed for cluster %s, err: %w", limitedSupportReason.ID(), hostedClusterID, err)
			}
		}
	}

	return nil
}

func (h *WebhookRHOBSReceiverHandler) processAlert(alert template.Alert, isCurrentlyFiring bool) error {
	// Filter actionable alert based on Label
	if !isValidAlert(alert, true) {
		log.WithField(LogFieldAlert, fmt.Sprintf("%+v", alert)).Info("alert does not meet valid criteria")
		return fmt.Errorf("alert does not meet valid criteria")
	}

	fleetNotificationRetriever, err := newFleetNotificationRetriever(h.c, context.Background(), alert)
	if err != nil {
		return fmt.Errorf("unable to find ManagedFleetNotification %s", alert.Labels[AMLabelTemplateName])
	}

	if !fleetNotificationRetriever.fleetNotification.LimitedSupport && !isCurrentlyFiring {
		return nil
	}

	var c *fleetNotificationContext
	canSend := false
	err = retryOnConflictOrAlreadyExists(retryConfig, func() error {
		c, err = fleetNotificationRetriever.retrieveFleetNotificationContext()
		if err != nil {
			return err
		}

		if isCurrentlyFiring {
			canSend = c.canSendNotification()
		}

		return c.updateNotificationStatus(isCurrentlyFiring, canSend)
	})
	if err != nil {
		return err
	}

	fleetNotification := c.retriever.fleetNotification
	alertName := alert.Labels[AMLabelAlertName]

	if isCurrentlyFiring {
		if canSend {
			err := c.sendNotification(h.ocm, alert)

			var logService string
			if fleetNotification.LimitedSupport { // Limited support case
				logService = config.ClustersService
			} else { // Service log case
				logService = config.ServiceLogService
			}

			if err != nil {
				if fleetNotification.LimitedSupport { // Limited support case
					metrics.IncrementFailedLimitedSupportSend(fleetNotification.Name)
				} else { // Service log case
					metrics.CountFailedServiceLogs(fleetNotification.Name)
				}
				metrics.SetResponseMetricFailure(logService, fleetNotification.Name, alertName)
				_ = c.restoreNotificationStatus()
				return err
			}

			if fleetNotification.LimitedSupport { // Limited support case
				metrics.IncrementLimitedSupportSentCount(fleetNotification.Name)
			} else { // Service log case
				metrics.CountServiceLogSent(fleetNotification.Name, "firing")
			}
			metrics.ResetResponseMetricFailure(logService, fleetNotification.Name, alertName)
		}
	} else {
		if c.wasClusterInLimitedSupport {
			err := c.removeLimitedSupport(h.ocm)

			if err != nil {
				metrics.IncrementFailedLimitedSupportRemoved(fleetNotification.Name)
				metrics.SetResponseMetricFailure(config.ClustersService, fleetNotification.Name, alertName)
				_ = c.restoreNotificationStatus()
				return err
			}
			metrics.IncrementLimitedSupportRemovedCount(fleetNotification.Name)
			metrics.ResetResponseMetricFailure(config.ClustersService, fleetNotification.Name, alertName)
		}
	}

	return nil
}

// The upstream implementation of `RetryOnConflict`
// calls `IsConflict` which doesn't handle `AlreadyExists` as a conflict error,
// even though it is meant to be a subcategory of conflict.
// This function implements a retry for errors of type Conflict or AlreadyExists (both status code 409):
// - conflict errors are triggered when failing to  Update() a resource
// - alreadyexists errors are triggered when failing to Create() a resource
func retryOnConflictOrAlreadyExists(backoff wait.Backoff, fn func() error) error {
	return retry.OnError(backoff, customIs409, fn)
}
