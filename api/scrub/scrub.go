package handler

import (
	"encoding/json"
	"net/http"

	"github.com/vici-cdr-scrubber/pkg/enrichment"
	"github.com/vici-cdr-scrubber/pkg/fraud"
	"github.com/vici-cdr-scrubber/pkg/models"
	"github.com/vici-cdr-scrubber/pkg/scrubber"
	"github.com/vici-cdr-scrubber/pkg/validation"
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
			CallDate            string `json:"call_date"`
			CallerCode          string `json:"caller_code"`
			CustomerPhoneNumber string `json:"customer_phone_number"`
			PhoneCode           string `json:"phone_code"`
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

	cfg := models.ScrubberConfig{
		BatchSize:       1000,
		Deduplication:   true,
		NormalizePhones: true,
		RemoveInternal:  true,
		MinDuration:     0,
		MaxDuration:     86400,
	}

	fraudCfg := fraud.FraudConfig{
		Enabled:               true,
		HighVolumeThreshold:   100,
		ShortCallThreshold:    5,
		AnomalyScoreThreshold: 0.6,
	}

	s := scrubber.NewScrubber(cfg)
	fraudEngine := fraud.NewDetectionEngine(fraudCfg)
	v := validation.NewValidator(validation.ValidatorConfig{MaxDuration: 86400})
	enrich := enrichment.NewEnricher()

	var cdrs []models.CDR
	for _, input := range req.CDRs {
		cdrs = append(cdrs, models.CDR{
			UniqueID:            input.UniqueID,
			CallerCode:          input.CallerCode,
			CustomerPhoneNumber: input.CustomerPhoneNumber,
			PhoneCode:           input.PhoneCode,
			CampaignID:          input.CampaignID,
			Status:              input.Status,
			CallDuration:        input.CallDuration,
		})
	}

	scrubbed := s.ProcessBatch(r.Context(), cdrs)

	type result struct {
		UniqueID        string  `json:"unique_id"`
		IsValid         bool    `json:"is_valid"`
		ScrubReason     string  `json:"scrub_reason,omitempty"`
		FraudScore      float64 `json:"fraud_score"`
		ValidationScore float64 `json:"validation_score"`
		CarrierName     string  `json:"carrier_name,omitempty"`
		Country         string  `json:"country,omitempty"`
		NormalizedPhone string  `json:"normalized_phone,omitempty"`
	}

	var results []result
	for i, sb := range scrubbed {
		alerts := fraudEngine.AnalyzeCDR(cdrs[i])
		fraudScore := 0.0
		if len(alerts) > 0 {
			fraudScore = alerts[0].Score
		}
		valResult := v.ValidateCDR(cdrs[i])
		enriched := enrich.EnrichCDR(sb)

		results = append(results, result{
			UniqueID:        sb.UniqueID,
			IsValid:         sb.IsValid,
			ScrubReason:     sb.ScrubReason,
			FraudScore:      fraudScore,
			ValidationScore: valResult.Score,
			CarrierName:     enriched.CarrierName,
			Country:         enriched.Country,
			NormalizedPhone: sb.NormalizedPhone,
		})
	}

	stats := s.GetStats()
	stats.TotalRecords = int64(len(cdrs))

	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":   "Scrubbing complete",
		"processed": len(scrubbed),
		"results":   results,
		"stats":     stats,
	})
}
