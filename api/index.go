package handler

import (
	"encoding/json"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/vici-cdr-scrubber/pkg/enrichment"
	"github.com/vici-cdr-scrubber/pkg/fraud"
	"github.com/vici-cdr-scrubber/pkg/models"
	"github.com/vici-cdr-scrubber/pkg/profiler"
	"github.com/vici-cdr-scrubber/pkg/scrubber"
	"github.com/vici-cdr-scrubber/pkg/validation"
)

func Handler(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api")
	path = strings.TrimPrefix(path, "/")
	path = strings.TrimSuffix(path, "/")

	switch path {
	case "", "index":
		handleIndex(w, r)
	case "health":
		handleHealth(w, r)
	case "version":
		handleVersion(w, r)
	case "scrub":
		handleScrub(w, r)
	case "profile":
		handleProfile(w, r)
	case "validate":
		handleValidate(w, r)
	default:
		http.NotFound(w, r)
	}
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"name":        "Vici Dialer CDR Scrubber",
		"version":     "2.0.0",
		"description": "Enterprise-grade CDR processing API",
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

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
		"platform":  "Vercel Serverless",
	})
}

func handleVersion(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"version":    "2.0.0",
		"go_version": runtime.Version(),
		"platform":   runtime.GOOS + "/" + runtime.GOARCH,
		"build_time": time.Now().Format(time.RFC3339),
		"features": []string{
			"Fraud Detection & Anomaly Analysis",
			"Data Profiling & Quality Scoring",
			"Real-time Validation",
			"Data Enrichment (Carrier, Timezone, Geo)",
			"Geographic Analysis & Geo-fencing",
			"Link Analysis & Pattern Detection",
			"Advanced Reporting (CSV, JSON)",
			"Task Scheduling",
			"Deduplication & Phone Normalization",
		},
	})
}

func handleScrub(w http.ResponseWriter, r *http.Request) {
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
		BatchSize: 1000, Deduplication: true, NormalizePhones: true,
		RemoveInternal: true, MinDuration: 0, MaxDuration: 86400,
	}
	fraudCfg := fraud.FraudConfig{
		Enabled: true, HighVolumeThreshold: 100,
		ShortCallThreshold: 5, AnomalyScoreThreshold: 0.6,
	}

	s := scrubber.NewScrubber(cfg)
	fraudEngine := fraud.NewDetectionEngine(fraudCfg)
	v := validation.NewValidator(validation.ValidatorConfig{MaxDuration: 86400})
	enrich := enrichment.NewEnricher()

	var cdrs []models.CDR
	for _, input := range req.CDRs {
		cdrs = append(cdrs, models.CDR{
			UniqueID: input.UniqueID, CallerCode: input.CallerCode,
			CustomerPhoneNumber: input.CustomerPhoneNumber, PhoneCode: input.PhoneCode,
			CampaignID: input.CampaignID, Status: input.Status, CallDuration: input.CallDuration,
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
			UniqueID: sb.UniqueID, IsValid: sb.IsValid, ScrubReason: sb.ScrubReason,
			FraudScore: fraudScore, ValidationScore: valResult.Score,
			CarrierName: enriched.CarrierName, Country: enriched.Country,
			NormalizedPhone: sb.NormalizedPhone,
		})
	}

	stats := s.GetStats()
	stats.TotalRecords = int64(len(cdrs))

	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Scrubbing complete", "processed": len(scrubbed),
		"results": results, "stats": stats,
	})
}

func handleProfile(w http.ResponseWriter, r *http.Request) {
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
			UniqueID: input.UniqueID, CallerCode: input.CallerCode,
			CustomerPhoneNumber: input.CustomerPhoneNumber, CampaignID: input.CampaignID,
			Status: input.Status, CallDuration: input.CallDuration,
		})
	}

	stats := p.Profile(cdrs)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"total_records": stats.TotalRecords, "quality_score": stats.QualityScore,
		"issues": stats.IssuesFound, "field_completeness": stats.FieldCompleteness,
	})
}

func handleValidate(w http.ResponseWriter, r *http.Request) {
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

	v := validation.NewValidator(validation.ValidatorConfig{MaxDuration: 86400, MinDuration: 0})
	var cdrs []models.CDR
	for _, input := range req.CDRs {
		cdrs = append(cdrs, models.CDR{
			UniqueID: input.UniqueID, CallerCode: input.CallerCode,
			CustomerPhoneNumber: input.CustomerPhoneNumber, CampaignID: input.CampaignID,
			Status: input.Status, CallDuration: input.CallDuration,
		})
	}

	results := v.ValidateBatch(cdrs)
	total := len(results)
	valid := 0
	totalScore := 0.0
	type item struct {
		UniqueID string  `json:"unique_id"`
		IsValid  bool    `json:"is_valid"`
		Score    float64 `json:"score"`
		Errors   int     `json:"errors"`
		Warnings int     `json:"warnings"`
	}
	var items []item
	for i, result := range results {
		if result.IsValid {
			valid++
		}
		totalScore += result.Score
		items = append(items, item{
			UniqueID: cdrs[i].UniqueID, IsValid: result.IsValid, Score: result.Score,
			Errors: len(result.Errors), Warnings: len(result.Warnings),
		})
	}

	avgScore := 0.0
	if total > 0 {
		avgScore = totalScore / float64(total)
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"total": total, "valid": valid, "invalid": total - valid,
		"pass_rate": float64(valid) / float64(total) * 100,
		"avg_score": avgScore, "results": items,
	})
}
