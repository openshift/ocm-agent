package webhookreceiver

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/prometheus/alertmanager/template"
)

type Handler struct {
	// Route to set method, path etc.
	Route func(r *mux.Route)

	// HTTP Handler
	Func http.HandlerFunc
}

// AddRoute adds the handler's route the to the router.
func (h Handler) AddRoute(r *mux.Router) {
	h.Route(r.NewRoute().HandlerFunc(h.Func))
}

// Alert Manager receiver response
type AMReceiverResponse struct {
	Status string
}

// Set webhook receiver path
const AMReceiverPath = "/alertmanager-receiver"

// Use prometheus alertmanager template type for post data
type AMReceiverData template.Data
