package handler

import (
	"encoding/json"
	"net/http"
	"time"
)

func Handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"name":        "Vici Dialer CDR Scrubber",
		"version":     "2.0.0",
		"description": "Enterprise-grade CDR processing API - Deployed on Vercel",
		"endpoints": map[string]string{
			"GET /api/health":    "Health check",
			"GET /api/version":   "Version info",
			"POST /api/scrub":    "Scrub CDR data",
			"POST /api/profile":  "Profile data quality",
			"POST /api/validate": "Validate CDR records",
		},
		"timestamp": time.Now().Format(time.RFC3339),
	})
}
