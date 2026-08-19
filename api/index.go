package handler

import (
	"encoding/json"
	"net/http"
	"time"
)

type Response struct {
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Description string            `json:"description"`
	Endpoints   map[string]string `json:"endpoints"`
}

func Handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	response := Response{
		Name:        "Vici Dialer CDR Scrubber",
		Version:     "2.0.0",
		Description: "Enterprise-grade CDR processing API - Deployed on Vercel",
		Endpoints: map[string]string{
			"GET /api/health":   "Health check",
			"GET /api/version":  "Version info",
			"POST /api/scrub":   "Scrub CDR data (demo)",
			"POST /api/profile": "Profile data quality (demo)",
			"POST /api/validate":"Validate CDR records (demo)",
		},
	}

	json.NewEncoder(w).Encode(response)
}

func init() {
	http.HandleFunc("/", Handler)
}

var _ = time.Now
