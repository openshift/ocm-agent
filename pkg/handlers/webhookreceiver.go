package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/prometheus/alertmanager/template"
	log "github.com/sirupsen/logrus"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type WebhookReceiverHandler struct {
	c *client.Client
}

// Alert Manager receiver response
type AMReceiverResponse struct {
	Status string
}

// Use prometheus alertmanager template type for post data
type AMReceiverData template.Data

func NewWebhookReceiverHandler(c *client.Client) *WebhookReceiverHandler {
	return &WebhookReceiverHandler{
		c: c,
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
	go h.processAMReceiver(alertData)

	// write response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	response := AMReceiverResponse{
		Status: "ok",
	}
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		log.Errorf("Failed to write to response: %s\n", err)
		http.Error(w, "Failed to write to responss", http.StatusInternalServerError)
		return
	}
}

func (h *WebhookReceiverHandler) processAMReceiver(d AMReceiverData) {
	log.WithField("AMReceiverData", d).Info("Process alert data")
}
