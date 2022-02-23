package main

import (
	"encoding/json"
	"net/http"
)

func serveJson(w http.ResponseWriter, status int, body interface{}) {
	w.WriteHeader(status)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(body); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
