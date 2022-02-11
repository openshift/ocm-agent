package webhookreceiver

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

func AMReceiver() Handler {
	return Handler{
		Route: func(r *mux.Route) {
			r.Path(AMReceiverPath).Methods(http.MethodPost)
		},
		Func: func(w http.ResponseWriter, r *http.Request) {
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
			go processAMReceiver(alertData)

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
		},
	}
}
