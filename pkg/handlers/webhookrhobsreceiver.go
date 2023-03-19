package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/prometheus/alertmanager/template"
	log "github.com/sirupsen/logrus"

	oav1alpha1 "github.com/openshift/ocm-agent-operator/api/v1alpha1"
	"github.com/openshift/ocm-agent/pkg/consts"
	"github.com/openshift/ocm-agent/pkg/metrics"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	OCMAgentNamespaceName  = "openshift-ocm-agent-operator"
	AMLabelAlertSourceName = "source"
	AMLabelAlertSourceHCP  = "HCP"
	AMLabelAlertSourceDP   = "DP"
	AMLabelAlertMCID       = "_mc_id"
	AMLabelAlertHCID       = "_id"
)

//go:generate mockgen -destination=mocks/webhookrhobsreceiver.go -package=mocks github.com/openshift/ocm-agent/pkg/handlers NewOCMClient
type NewOCMClient interface {
	SendServiceLogInFleetMode(n *oav1alpha1.FleetNotification, firing bool, clusterID string) error
}

type WebhookRHOBSReceiverHandler struct {
	c   client.Client
	ocm NewOCMClient
}

func NewWebhookRHOBSReceiverHandler(c client.Client, o NewOCMClient) *WebhookRHOBSReceiverHandler {
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

	listOptions := []client.ListOption{
		client.InNamespace(OCMAgentNamespaceName),
	}
	// Let's get all ManagedFleetNotifications
	mfnl := &oav1alpha1.ManagedFleetNotificationList{}
	err := h.c.List(ctx, mfnl, listOptions...)
	if err != nil {
		log.WithError(err).Error("unable to list managedFleetNotifications")
		return &AMReceiverResponse{Error: err, Status: "unable to list managedFleetNotifications", Code: http.StatusInternalServerError}
	}

	// Handle each firing alert
	for _, alert := range d.Alerts.Firing() {
		// Filter actionable alert based on Label
		if !actionableAlert(alert) {
			continue
		}
		err = h.processAlert(alert, mfnl, true)
		if err != nil {
			log.WithError(err).Error("a firing alert could not be successfully processed")
		}
	}
	return &AMReceiverResponse{Error: nil, Status: "ok", Code: http.StatusOK}
}

// processAlert handles the pre-check verification and sending of a notification for a particular alert
// and returns an error if that process completed successfully or false otherwise
func (h *WebhookRHOBSReceiverHandler) processAlert(alert template.Alert, mfnl *oav1alpha1.ManagedFleetNotificationList, firing bool) error {
	// Should this alert be handled?
	if !isValidAlert(alert) {
		log.WithField(LogFieldAlert, fmt.Sprintf("%+v", alert)).Info("alert does not meet valid criteria")
		return fmt.Errorf("alert does not meet valid criteria")
	}

	// Can the alert be mapped to an existing notification definition?
	fn, mfn, err := getFleetNotification(alert.Labels[AMLabelTemplateName], mfnl)
	if err != nil {
		log.WithError(err).WithField(LogFieldAlert, fmt.Sprintf("%+v", alert)).Warning("an alert fired with no associated notification template definition")
		return err
	}

	mcID := alert.Labels[AMLabelAlertMCID]
	hcID := alert.Labels[AMLabelAlertHCID]
	slSent := false
	for !slSent {
		var mfnr *oav1alpha1.ManagedFleetNotificationRecord
		err = h.c.Get(context.Background(), client.ObjectKey{
			Namespace: OCMAgentNamespaceName,
			Name:      mcID,
		}, mfnr)

		if err != nil {
			log.WithError(err).Error("unable to fetch managedFleetNotificationRecord")
			return fmt.Errorf("unable to fetch managedFleetNotificationRecord for %s", mcID)
		}
		if mfnr != nil {
			// Fetch notificationRecordByName and ADD if it doesn't exist
			nfrbn, err := mfnr.GetNotificationRecordByName(mcID, fn.Name)
			if err != nil {
				return fmt.Errorf("cannot find notification record for name %s", fn.Name)
			}
			if nfrbn != nil {
				nri, err := mfnr.GetNotificationRecordItem(mcID, fn.Name, hcID)
				if err != nil {
					return err
				}
				if nri != nil {
					// Has a servicelog already been sent
					canBeSent, err := mfnr.CanBeSent(alert.Labels[mcID], fn.Name, alert.Labels[hcID])
					if err != nil {
						log.WithError(err).WithField(LogFieldNotificationName, fn.Name).Error("unable to fetch NotificationrecordByName or NotificationRecordItem")
						return err
					}
					if !canBeSent {
						if firing {
							log.WithFields(log.Fields{"notification": fn.Name,
								LogFieldResendInterval: fn.ResendWait,
							}).Info("not sending a notification as one was already sent recently")
						}
					}
					// Send the servicelog for the alert
					log.WithFields(log.Fields{LogFieldNotificationName: fn.Name}).Info("will send servicelog for notification")
					err = h.ocm.SendServiceLogInFleetMode(fn, firing, alert.Labels[hcID])
					if err != nil {
						log.WithError(err).WithFields(log.Fields{LogFieldNotificationName: fn.Name, LogFieldIsFiring: true}).Error("unable to send a notification")
						metrics.SetResponseMetricFailure("service_logs")
						return err
					}
					slSent = true
					// Reset the metric if we got correct Response from OCM
					metrics.ResetMetric(metrics.MetricResponseFailure)

					// Count the service log sent by the template name
					if firing {
						metrics.CountServiceLogSent(fn.Name, "firing")
					}
					ri := mfnr.UpdateNotificationRecordItem(nri)
					if ri == nil {
						log.WithFields(log.Fields{LogFieldNotificationName: fn.Name, LogFieldManagedNotification: mfn.Name}).WithError(err).Error("unable to update notification status")
						return err
					}
				} else {
					// add AddNotificationRecordItem
					mfnr.AddNotificationRecordItem(hcID, nfrbn)
				}
			} else {
				//  add NotificationRecordByName
				addNotificationRecordByName(fn.Name, fn.ResendWait, mfnr)
			}
		} else {
			// create ManagedFleetNotificationRecord if not found
			err := h.createManagedFleetNotificationRecord(mcID)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// getFleetNotification returns the fleetnotification from the ManagedFleetNotification bundle if one exists, or error if one does not
func getFleetNotification(name string, m *oav1alpha1.ManagedFleetNotificationList) (*oav1alpha1.FleetNotification, *oav1alpha1.ManagedFleetNotification, error) {
	for _, mn := range m.Items {
		mfn, err := mn.GetNotificationByName(name)
		if mfn.Spec.FleetNotification.Name != "" && err == nil {
			return &mfn.Spec.FleetNotification, &mn, nil
		}
	}
	return nil, nil, fmt.Errorf("matching managed notification not found for %s", name)
}

// create ManagedFleetNotificationRecord
func (h *WebhookRHOBSReceiverHandler) createManagedFleetNotificationRecord(mcID string) error {
	mfnr := &oav1alpha1.ManagedFleetNotificationRecord{
		ObjectMeta: v1.ObjectMeta{
			Name:      mcID,
			Namespace: OCMAgentNamespaceName,
		},
		Status: oav1alpha1.ManagedFleetNotificationRecordStatus{
			ManagementCluster:        mcID,
			NotificationRecordByName: nil,
		},
	}
	err := h.c.Create(context.Background(), mfnr)
	if err != nil {
		return err
	}
	return nil
}

// add NotificationRecordByName for fleetnotification
func addNotificationRecordByName(name string, rswait int32, mfrn *oav1alpha1.ManagedFleetNotificationRecord) *oav1alpha1.ManagedFleetNotificationRecord {
	nfrbn := oav1alpha1.NotificationRecordByName{
		NotificationName:        name,
		ResendWait:              rswait,
		NotificationRecordItems: nil,
	}
	mfrn.Status.NotificationRecordByName = append(mfrn.Status.NotificationRecordByName, nfrbn)
	return mfrn
}

// filter actionable alert based on alert Label
func actionableAlert(alert template.Alert) bool {
	alertLabels := alert.Labels
	if name, ok := alertLabels[AMLabelAlertSourceName]; ok {
		if name == AMLabelAlertSourceHCP || name == AMLabelAlertSourceDP {
			return true
		}
	}
	return false
}
