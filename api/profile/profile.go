package handler

import (
	"encoding/json"
	"net/http"

	"github.com/vici-cdr-scrubber/pkg/models"
	"github.com/vici-cdr-scrubber/pkg/profiler"
)

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
		cdrs = append(cdrs, models.CDR{
			UniqueID:            input.UniqueID,
			CallerCode:          input.CallerCode,
			CustomerPhoneNumber: input.CustomerPhoneNumber,
			CampaignID:          input.CampaignID,
			Status:              input.Status,
			CallDuration:        input.CallDuration,
		})
	}

	stats := p.Profile(cdrs)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"total_records":      stats.TotalRecords,
		"quality_score":      stats.QualityScore,
		"issues":             stats.IssuesFound,
		"field_completeness": stats.FieldCompleteness,
	})
}
