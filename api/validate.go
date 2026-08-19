package handler

import (
	"encoding/json"
	"net/http"

	"github.com/vici-cdr-scrubber/pkg/models"
	"github.com/vici-cdr-scrubber/pkg/validation"
)

type ValidateResponse struct {
	Total         int                    `json:"total"`
	Valid         int                    `json:"valid"`
	Invalid       int                    `json:"invalid"`
	PassRate      float64                `json:"pass_rate"`
	AvgScore      float64                `json:"avg_score"`
	Results       []ValidationItem       `json:"results"`
}

type ValidationItem struct {
	UniqueID string  `json:"unique_id"`
	IsValid  bool    `json:"is_valid"`
	Score    float64 `json:"score"`
	Errors   int     `json:"errors"`
	Warnings int     `json:"warnings"`
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

	validator := validation.NewValidator(validation.ValidatorConfig{
		MaxDuration: 86400,
		MinDuration: 0,
	})

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

	results := validator.ValidateBatch(cdrs)

	total := len(results)
	valid := 0
	totalScore := 0.0
	var items []ValidationItem

	for i, result := range results {
		if result.IsValid {
			valid++
		}
		totalScore += result.Score

		item := ValidationItem{
			UniqueID: cdrs[i].UniqueID,
			IsValid:  result.IsValid,
			Score:    result.Score,
			Errors:   len(result.Errors),
			Warnings: len(result.Warnings),
		}
		items = append(items, item)
	}

	avgScore := 0.0
	if total > 0 {
		avgScore = totalScore / float64(total)
	}

	response := ValidateResponse{
		Total:    total,
		Valid:    valid,
		Invalid:  total - valid,
		PassRate: float64(valid) / float64(total) * 100,
		AvgScore: avgScore,
		Results:  items,
	}

	json.NewEncoder(w).Encode(response)
}

func init() {
	http.HandleFunc("/api/validate", Handler)
}
