package types

import (
	"net/http"

	"github.com/gorilla/mux"
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
