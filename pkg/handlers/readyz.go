package handlers

import (
	"encoding/json"
	"net/http"

	log "github.com/sirupsen/logrus"
)

type ReadyzHandler struct {
}

// ready probe endpoint response
type ReadyzResponse struct {
	Status string
}

func NewReadyzHandler() *ReadyzHandler {
	return &ReadyzHandler{}
}

func (h *ReadyzHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Debug("Handling readyz request")
	// validate request
	if r != nil && r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	var err error
	// write response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	response := ReadyzResponse{
		Status: "ok",
	}
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		log.Errorf("Failed to write to response: %s\n", err)
		http.Error(w, "Failed to write to response", http.StatusInternalServerError)
		return
	}
}
