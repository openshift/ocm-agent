package hack

import (
	"encoding/json"
	"net/http"
)

// Dummy is a struct that contains dummy data
type Dummy struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}

var data []Dummy = []Dummy{}

func AddItem(w http.ResponseWriter, r *http.Request) {
	var newData Dummy
	json.NewDecoder(r.Body).Decode(&newData)
	data = append(data, newData)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func ListItem(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
