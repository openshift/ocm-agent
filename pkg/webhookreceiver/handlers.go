package webhookreceiver

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/mux"
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
			// process request
			var alertData AMReceiverData
			json.NewDecoder(r.Body).Decode(&alertData)
			go processAMReceiver(alertData)

			// write response
			w.Header().Set("Content-Type", "application/json")
			response := AMReceiverResponse{
				Status: "ok",
			}
			err := json.NewEncoder(w).Encode(response)
			if err != nil {
				log.Printf("Failed to write to response: %s\n", err)
			}
		},
	}
}
