package main

import (
	"encoding/json"
	"net/http"

	"github.com/vici-cdr-scrubber/pkg/models"
	"github.com/vici-cdr-scrubber/pkg/profiler"
)

type ProfileResponse struct {
	TotalRecords int64                   `json:"total_records"`
	QualityScore float64                 `json:"quality_score"`
	Issues       []profiler.QualityIssue `json:"issues"`
	Completeness  map[string]float64      `json:"field_completeness"`
}

func Handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
		return
	}

	var req struct {
		CDRs []struct {
			UniqueID            string `json:"unique_id"`
			CallerCode          string `json:"caller_code"`
			CustomerPhoneNumber string `json:"customer_phone_number"`
			CampaignID          string `json:"campaign_id"`
			Status              string `json:"status"`
			CallDuration        int    `json:"call_duration"`
		} `json:"cdrs"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}

	p := profiler.NewDataProfiler()

	var cdrs []models.CDR
	for _, input := range req.CDRs {
		cdr := models.CDR{
			UniqueID:            input.UniqueID,
			CallerCode:          input.CallerCode,
			CustomerPhoneNumber: input.CustomerPhoneNumber,
			CampaignID:          input.CampaignID,
			Status:              input.Status,
			CallDuration:        input.CallDuration,
		}
		cdrs = append(cdrs, cdr)
	}

	stats := p.Profile(cdrs)

	response := ProfileResponse{
		TotalRecords: stats.TotalRecords,
		QualityScore: stats.QualityScore,
		Issues:       stats.IssuesFound,
		Completeness: stats.FieldCompleteness,
	}

	json.NewEncoder(w).Encode(response)
}
