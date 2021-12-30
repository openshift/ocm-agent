package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/openshift/ocm-agent/hack"
)

func main() {
	port := 8081

	log.Println("starting ocm-agent server")
	// create a new router
	r := mux.NewRouter()

	r.HandleFunc("/readyz", HealthCheck).Methods("GET")
	r.HandleFunc("/data", hack.AddItem).Methods("POST")
	r.HandleFunc("/data", hack.ListItem).Methods("GET")

	log.Printf("Start listening on port %v", port)
	log.Fatal(http.ListenAndServe(":8081", r))
}

func HealthCheck(w http.ResponseWriter, r *http.Request) {
	log.Println("registering health check end point")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "API is up and running")
}
