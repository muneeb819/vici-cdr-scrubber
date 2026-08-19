package handler

import (
	"encoding/json"
	"net/http"

	"github.com/vici-cdr-scrubber/internal/fraud"
	"github.com/vici-cdr-scrubber/internal/models"
	"github.com/vici-cdr-scrubber/internal/scrubber"
	"github.com/vici-cdr-scrubber/internal/enrichment"
	"github.com/vici-cdr-scrubber/internal/validation"
)

type ScrubRequest struct {
	CDRs []CDRInput `json:"cdrs"`
}

type CDRInput struct {
	UniqueID            string `json:"unique_id"`
	CallDate            string `json:"call_date"`
	CallerCode          string `json:"caller_code"`
	CustomerPhoneNumber string `json:"customer_phone_number"`
	PhoneCode           string `json:"phone_code"`
	CampaignID          string `json:"campaign_id"`
	Status              string `json:"status"`
	CallDuration        int    `json:"call_duration"`
}

type ScrubResponse struct {
	Message   string                `json:"message"`
	Processed int                   `json:"processed"`
	Results   []ScrubbedResult      `json:"results"`
	Stats     models.ScrubStats     `json:"stats"`
}

type ScrubbedResult struct {
	UniqueID         string  `json:"unique_id"`
	IsValid          bool    `json:"is_valid"`
	ScrubReason      string  `json:"scrub_reason,omitempty"`
	FraudScore       float64 `json:"fraud_score"`
	ValidationScore  float64 `json:"validation_score"`
	CarrierName      string  `json:"carrier_name,omitempty"`
	Country          string  `json:"country,omitempty"`
	NormalizedPhone  string  `json:"normalized_phone,omitempty"`
}

func Handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
		return
	}

	var req ScrubRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}

	cfg := &models.ScrubberConfig{
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

	s := scrubber.NewScrubber(*cfg)
	fraudEngine := fraud.NewDetectionEngine(fraudCfg)
	validator := validation.NewValidator(validation.ValidatorConfig{MaxDuration: 86400})
	enricher := enrichment.NewEnricher()

	var cdrs []models.CDR
	for _, input := range req.CDRs {
		cdr := models.CDR{
			UniqueID:            input.UniqueID,
			CallerCode:          input.CallerCode,
			CustomerPhoneNumber: input.CustomerPhoneNumber,
			PhoneCode:           input.PhoneCode,
			CampaignID:          input.CampaignID,
			Status:              input.Status,
			CallDuration:        input.CallDuration,
		}
		cdrs = append(cdrs, cdr)
	}

	scrubbed := s.ProcessBatch(r.Context(), cdrs)

	var results []ScrubbedResult
	for i, sb := range scrubbed {
		alerts := fraudEngine.AnalyzeCDR(cdrs[i])
		fraudScore := 0.0
		if len(alerts) > 0 {
			fraudScore = alerts[0].Score
		}

		valResult := validator.ValidateCDR(cdrs[i])
		enriched := enricher.EnrichCDR(sb)

		results = append(results, ScrubbedResult{
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

	response := ScrubResponse{
		Message:   "Scrubbing complete",
		Processed: len(scrubbed),
		Results:   results,
		Stats:     stats,
	}

	json.NewEncoder(w).Encode(response)
}

func init() {
	http.HandleFunc("/api/scrub", Handler)
}
