package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/openshift-online/ocm-cli/pkg/arguments"
	sdk "github.com/openshift-online/ocm-sdk-go"
	"github.com/prometheus/alertmanager/template"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	oav1alpha1 "github.com/openshift/ocm-agent-operator/pkg/apis/ocmagent/v1alpha1"
	"github.com/openshift/ocm-agent/pkg/config"
	"github.com/openshift/ocm-agent/pkg/ocm"

	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	AMLabelAlertName           = "alertname"
	AMLabelTemplateName        = "managed_notification_template"
	AMLabelManagedNotification = "send_managed_notification"
)

type WebhookReceiverHandler struct {
	c   client.Client
	ocm *sdk.Connection
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

func NewWebhookReceiverHandler(c client.Client, ocm *sdk.Connection) *WebhookReceiverHandler {
	return &WebhookReceiverHandler{
		c:   c,
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
		return
	}
}

func (h *WebhookReceiverHandler) processAMReceiver(d AMReceiverData, ctx context.Context) *AMReceiverResponse {
	log.WithField("AMReceiverData", d).Info("Process alert data")

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

	for _, alert := range d.Alerts.Firing() {
		// Ignore alerts which don't have a name
		alertname, err := alertName(alert)
		if err != nil {
			log.WithError(err).Error("alertname missing for alert")
			continue
		}

		// Ignore alerts which don't have the send_managed_notification label
		if val, ok := alert.Labels[AMLabelManagedNotification]; !ok || val == "false" {
			log.WithField("alertname", alertname).Debug("ignoring alert with no send_managed_notification label")
			continue
		}

		// Ignore alerts which don't have a template defined
		if _, ok := alert.Labels[AMLabelTemplateName]; !ok {
			log.WithField("alertname", alertname).Error("alert does not have template defined")
			continue
		}

		template, mn, err := getTemplate(alert.Labels[AMLabelTemplateName], mnl)
		if err != nil {
			log.WithError(err).WithField("alertname", alertname).Warning("an alert fired which no template definition exists for")
			continue
		}
		log.WithFields(log.Fields{"template": template.Name, "alertname": alertname}).Info("Found a template")

		err = h.sendServiceLog(template, true)
		if err != nil {
			log.WithError(err).WithFields(log.Fields{"template": template.Name, "firing": "true"}).Error("unable to send service log")
			continue
		}

		err = h.updateTemplateStatus(template, mn)
		if err != nil {
			log.WithFields(log.Fields{"template": template.Name, "managednotification": mn.Name}).WithError(err).Error("unable to update template status")
			continue
		}
	}
	return &AMReceiverResponse{Error: nil, Status: "ok", Code: http.StatusOK}
}

func getTemplate(name string, m *oav1alpha1.ManagedNotificationList) (*oav1alpha1.Template, *oav1alpha1.ManagedNotification, error) {
	for _, mn := range m.Items {
		template, err := mn.GetTemplateForName(name)
		if template != nil && err == nil {
			return template, &mn, nil
		}
	}
	return nil, nil, fmt.Errorf("matching managed notification template not found for %s", name)
}

func alertName(a template.Alert) (*string, error) {
	if name, ok := a.Labels[AMLabelAlertName]; ok {
		return &name, nil
	}
	return nil, fmt.Errorf("no alertname defined in alert")
}

func (h *WebhookReceiverHandler) sendServiceLog(t *oav1alpha1.Template, firing bool) error {
	req := h.ocm.Post()
	err := arguments.ApplyPathArg(req, "/api/service_logs/v1/cluster_logs")
	if err != nil {
		return err
	}

	sl := ocm.ServiceLog{
		ServiceName:  "SREManualAction",
		ClusterUUID:  viper.GetString(config.ClusterID),
		Summary:      t.Summary,
		InternalOnly: false,
	}
	if firing {
		sl.Description = t.ActiveDesc
	} else {
		sl.Description = t.ResolvedDesc
	}
	slAsBytes, err := json.Marshal(sl)
	if err != nil {
		return err
	}

	req.Bytes(slAsBytes)
	_, err = req.Send()
	if err != nil {
		return err
	}

	return nil
}

func (h *WebhookReceiverHandler) updateTemplateStatus(t *oav1alpha1.Template, mn *oav1alpha1.ManagedNotification) error {

	// Update lastSent timestamp
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		m := &oav1alpha1.ManagedNotification{}

		err := h.c.Get(context.TODO(), client.ObjectKey{
			Namespace: mn.Namespace,
			Name:      mn.Name,
		}, m)
		if err != nil {
			return err
		}

		status, err := m.Status.GetNotificationRecord(t.Name)
		if err != nil {
			// Status does not exist, create it
			status = &oav1alpha1.NotificationRecord{
				Name:                t.Name,
				ServiceLogSentCount: 0,
			}
			status.SetStatus(oav1alpha1.ConditionServiceLogSent, "Service log sent")
		} else {
			// Status exists, update it
			status.SetStatus(oav1alpha1.ConditionServiceLogSent, "Service log sent")
		}
		m.Status.Notifications.SetNotificationRecord(*status)

		err = h.c.Status().Update(context.TODO(), m)
		return err
	})

	return err
}
