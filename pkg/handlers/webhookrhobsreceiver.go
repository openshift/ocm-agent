package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/prometheus/alertmanager/template"
	"github.com/prometheus/common/model"
	log "github.com/sirupsen/logrus"

	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	oav1alpha1 "github.com/openshift/ocm-agent-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"

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
		Jitter:   3,
	}
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

	for _, alert := range d.Alerts {
		// Can we find a notification template for this alert?
		templateName := alert.Labels[AMLabelTemplateName]
		mfn := &oav1alpha1.ManagedFleetNotification{}
		err := h.c.Get(ctx, client.ObjectKey{
			Namespace: OCMAgentNamespaceName,
			Name:      templateName,
		}, mfn)
		if err != nil {
			log.WithError(err).Error("unable to locate corresponding notification template")
			return &AMReceiverResponse{Error: err,
				Status: fmt.Sprintf("unable to find ManagedFleetNotification %s", templateName),
				Code:   http.StatusInternalServerError}
		}

		// Filter actionable alert based on Label
		if !isValidAlert(alert, true) {
			log.WithField(LogFieldAlert, fmt.Sprintf("%+v", alert)).Info("alert does not meet valid criteria")
			continue
		}

		err = h.processAlert(alert, mfn)
		if err != nil {
			log.WithError(err).Error("failed processing alert")
		}
	}

	return &AMReceiverResponse{Error: nil, Status: "ok", Code: http.StatusOK}
}

func (h *WebhookRHOBSReceiverHandler) processAlert(alert template.Alert, mfn *oav1alpha1.ManagedFleetNotification) error {
	// Handle firing alerts
	if alert.Status == string(model.AlertFiring) {
		err := h.processFiringAlert(alert, mfn)
		if err != nil {
			return fmt.Errorf("a firing alert could not be successfully processed %w", err)
		}
		return nil
	}

	// Handle resolving alerts
	if alert.Status == string(model.AlertResolved) {
		err := h.processResolvedAlert(alert, mfn)
		if err != nil {
			return fmt.Errorf("a resolving alert could not be successfully processed %w", err)
		}
		return nil
	}

	return fmt.Errorf("unable to process alert: unexpected status %s", alert.Status)
}

// processResolvedAlert handles resolve notifications for a particular alert
// currently only handles removing limited support
func (h *WebhookRHOBSReceiverHandler) processResolvedAlert(alert template.Alert, mfn *oav1alpha1.ManagedFleetNotification) error {
	// MFN is not for limited support, thus we don't have an implementation for the alert resolving state yet
	if !mfn.Spec.FleetNotification.LimitedSupport {
		return nil
	}

	hcID := alert.Labels[AMLabelAlertHCID]
	fn := mfn.Spec.FleetNotification
	fnLimitedSupportReason := fn.NotificationMessage

	activeLSReasons, err := h.ocm.GetLimitedSupportReasons(hcID)
	if err != nil {
		return fmt.Errorf("unable to get limited support reasons for cluster %s:, %w", hcID, err)
	}

	for _, reason := range activeLSReasons {
		// If the reason matches the fleet notification LS reason, remove it
		// TODO(Claudio): Find a way to make sure the removed LS was also posted by OA
		if strings.Contains(reason.Details(), fnLimitedSupportReason) {
			log.WithFields(log.Fields{LogFieldNotificationName: fn.Name}).Infof("will remove limited support reason '%s' for notification", reason.ID())
			err := h.ocm.RemoveLimitedSupport(hcID, reason.ID())
			if err != nil {
				metrics.IncrementFailedLimitedSupportRemoved(fn.Name)
				metrics.SetResponseMetricFailure("clusters_mgmt")
				return fmt.Errorf("limited support reason with ID '%s' couldn't be removed for cluster %s, err: %w", reason.ID(), hcID, err)
			}
			metrics.IncrementLimitedSupportRemovedCount(fn.Name)
		}
	}
	// Reset the metric if we got correct Response from OCM
	metrics.ResetMetric(metrics.MetricResponseFailure)

	return h.updateManagedFleetNotificationRecord(alert, mfn)
}

// processFiringAlert handles the pre-check verification and sending of a notification for a particular alert
// and returns an error if that process completed successfully or false otherwise
func (h *WebhookRHOBSReceiverHandler) processFiringAlert(alert template.Alert, mfn *oav1alpha1.ManagedFleetNotification) error {
	fn := mfn.Spec.FleetNotification
	mcID := alert.Labels[AMLabelAlertMCID]
	hcID := alert.Labels[AMLabelAlertHCID]

	// We need the latest record just to compare timestamps for resending
	// We will later re-query for the update of the status field
	mfnr, err := h.getOrCreateManagedFleetNotificationRecord(mcID, hcID, mfn)
	if err != nil {
		return err
	}

	// Check if a firing notification can be sent
	canBeSent, err := mfnr.FiringCanBeSent(mcID, fn.Name, hcID)
	if err != nil {
		log.WithError(err).WithField(LogFieldNotificationName, fn.Name).Error("unable to fetch NotificationrecordByName or NotificationRecordItem")
		return err
	}
	// There's no need to send a service log, so just return
	if !canBeSent {
		log.WithFields(log.Fields{"notification": fn.Name,
			LogFieldResendInterval: fn.ResendWait,
		}).Info("not sending a notification as one was already sent recently")
		return nil
	}

	if mfn.Spec.FleetNotification.LimitedSupport {
		// Send the limited support for the alert
		log.WithFields(log.Fields{LogFieldNotificationName: fn.Name}).Info("will send limited support for notification")
		builder := &cmv1.LimitedSupportReasonBuilder{}
		builder.Summary(fn.Summary)
		builder.Details(fn.NotificationMessage)
		builder.DetectionType(cmv1.DetectionTypeManual)
		reason, err := builder.Build()
		if err != nil {
			return fmt.Errorf("unable to build limited support for fleetnotification '%s' reason: %w", fn.Name, err)
		}
		err = h.ocm.SendLimitedSupport(hcID, reason)
		if err != nil {
			metrics.SetResponseMetricFailure("clusters_mgmt")
			metrics.IncrementFailedLimitedSupportSend(fn.Name)
			return fmt.Errorf("limited support reason for fleetnotification '%s' could not be set for cluster %s, err: %w", fn.Name, hcID, err)
		}
		metrics.IncrementLimitedSupportSentCount(fn.Name)
	} else { // Notification is for a service log
		log.WithFields(log.Fields{LogFieldNotificationName: fn.Name}).Info("will send servicelog for notification")
		err = ocm.BuildAndSendServiceLog(
			ocm.NewServiceLogBuilder(fn.Summary, fn.NotificationMessage, "", hcID, fn.Severity, fn.LogType, fn.References),
			true, &alert, h.ocm)
		if err != nil {
			log.WithError(err).WithFields(log.Fields{LogFieldNotificationName: fn.Name, LogFieldIsFiring: true}).Error("unable to send service log for notification")
			metrics.SetResponseMetricFailure("service_logs")
			metrics.CountFailedServiceLogs(fn.Name)
			return err
		}
		// Count the service log sent by the template name
		metrics.CountServiceLogSent(fn.Name, "firing")
	}

	// Reset the metric if we got correct Response from OCM
	metrics.ResetMetric(metrics.MetricResponseFailure)

	return h.updateManagedFleetNotificationRecord(alert, mfn)
}

// Get or create ManagedFleetNotificationRecord
func (h *WebhookRHOBSReceiverHandler) getOrCreateManagedFleetNotificationRecord(mcID string, hcID string, mfn *oav1alpha1.ManagedFleetNotification) (*oav1alpha1.ManagedFleetNotificationRecord, error) {
	fn := mfn.Spec.FleetNotification
	mfnr := &oav1alpha1.ManagedFleetNotificationRecord{}

	err := retry.RetryOnConflict(retryConfig, func() error {
		err := h.c.Get(context.Background(), client.ObjectKey{
			Namespace: OCMAgentNamespaceName,
			Name:      mcID,
		}, mfnr)

		if err != nil {
			if errors.IsNotFound(err) {
				// Record does not exist, attempt to create it
				mfnr = &oav1alpha1.ManagedFleetNotificationRecord{
					ObjectMeta: v1.ObjectMeta{
						Name:      mcID,
						Namespace: OCMAgentNamespaceName,
					},
				}

				if err := h.c.Create(context.Background(), mfnr); err != nil {
					return err
				}
			} else {
				return err
			}
		}

		// Update the status: can only be done after creation
		// We want to make sure the following exists or is added:
		// - the status needs the management cluster ID
		// - the status needs to have a NotificationRecordByName: notification name specific
		// - the NotificationRecordByName needs to have a NotificationRecordItem: hosted cluster specific
		// If any changes are made, we update the status.
		statusChanges := false

		// Ideally, this field should have probably been part of the ManagedFleetNotificationRecord
		// definition, not the status.
		if mfnr.Status.ManagementCluster == "" {
			mfnr.Status.ManagementCluster = mcID
			statusChanges = true
		}

		// Fetch notificationRecordByName
		recordByName, err := mfnr.GetNotificationRecordByName(mcID, fn.Name)
		if err != nil {
			// add it to our patch if it doesn't exist
			recordByName = &oav1alpha1.NotificationRecordByName{
				NotificationName:        fn.Name,
				ResendWait:              fn.ResendWait,
				NotificationRecordItems: nil,
			}
			mfnr.Status.NotificationRecordByName = append(mfnr.Status.NotificationRecordByName, *recordByName)

			statusChanges = true
		}

		// Fetch notificationRecordItem
		_, err = mfnr.GetNotificationRecordItem(mcID, fn.Name, hcID)
		if err != nil {
			// add it to our patch if it doesn't exist
			_, err = mfnr.AddNotificationRecordItem(hcID, recordByName)
			if err != nil {
				return err
			}

			statusChanges = true
		}

		if statusChanges {
			return h.c.Status().Update(context.TODO(), mfnr)
		}

		return nil
	})

	return mfnr, err
}

func (h *WebhookRHOBSReceiverHandler) updateManagedFleetNotificationRecord(alert template.Alert, mfn *oav1alpha1.ManagedFleetNotification) error {
	fn := mfn.Spec.FleetNotification
	mcID := alert.Labels[AMLabelAlertMCID]
	hcID := alert.Labels[AMLabelAlertHCID]
	firing := alert.Status == string(model.AlertFiring)

	err := retry.RetryOnConflict(retryConfig, func() error {
		// Fetch the ManagedFleetNotificationRecord, or create it if it does not already exist
		mfnr, err := h.getOrCreateManagedFleetNotificationRecord(mcID, hcID, mfn)
		if err != nil {
			return err
		}

		_, err = mfnr.UpdateNotificationRecordItem(fn.Name, hcID, firing)
		if err != nil {
			return err
		}

		err = h.c.Status().Update(context.TODO(), mfnr)
		if err != nil {
			return err
		}

		return nil
	})

	return err
}
