package handler

import (
	"encoding/json"
	"net/http"
	"time"
)

type HealthResponse struct {
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
	Platform  string `json:"platform"`
}

func Handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	response := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now().Format(time.RFC3339),
		Platform:  "Vercel Serverless",
	}

	json.NewEncoder(w).Encode(response)
}

func init() {
	http.HandleFunc("/api/health", Handler)
}
