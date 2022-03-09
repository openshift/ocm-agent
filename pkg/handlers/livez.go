package handlers

import (
	"encoding/json"
	"net/http"

	log "github.com/sirupsen/logrus"
)

type LivezHandler struct {
}

// live probe endpoint response
type LivezResponse struct {
	Status string
}

func NewLivezHandler() *LivezHandler {
	return &LivezHandler{}
}

func (h *LivezHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Debug("Handling livez request")
	// validate request
	if r != nil && r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	var err error

	// write response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	response := LivezResponse{
		Status: "ok",
	}
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		log.Errorf("Failed to write to response: %s\n", err)
		http.Error(w, "Failed to write to response", http.StatusInternalServerError)
		return
	}
}
