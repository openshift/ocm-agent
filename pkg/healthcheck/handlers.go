package healthcheck

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"

	"github.com/openshift/ocm-agent/pkg/types"
)

// live probe endpoint response
type LivezResponse struct {
	Status string
}

// Set live probe path
const LivezPath = "/livez"

// ready probe endpoint response
type ReadyzResponse struct {
	Status string
}

// Set ready probe path
const ReadyzPath = "/readyz"

func Livez() types.Handler {
	return types.Handler{
		Route: func(r *mux.Route) {
			r.Path(LivezPath).Methods(http.MethodGet)
		},
		Func: func(w http.ResponseWriter, r *http.Request) {
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
		},
	}
}

func Readyz() types.Handler {
	return types.Handler{
		Route: func(r *mux.Route) {
			r.Path(ReadyzPath).Methods(http.MethodGet)
		},
		Func: func(w http.ResponseWriter, r *http.Request) {
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
		},
	}
}
