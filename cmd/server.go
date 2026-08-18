package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/vici-cdr-scrubber/internal/database"
	"github.com/vici-cdr-scrubber/internal/enrichment"
	"github.com/vici-cdr-scrubber/internal/fraud"
	"github.com/vici-cdr-scrubber/internal/models"
	"github.com/vici-cdr-scrubber/internal/profiler"
	"github.com/vici-cdr-scrubber/internal/reporting"
	"github.com/vici-cdr-scrubber/internal/scrubber"
	"github.com/vici-cdr-scrubber/internal/validation"
	"gopkg.in/yaml.v3"
)

var (
	version = "2.0.0"
	cfg     *models.Config
)

func main() {
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config.yaml"
	}

	var err error
	cfg, err = loadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/health", handleHealth)
	http.HandleFunc("/scrub", handleScrub)
	http.HandleFunc("/profile", handleProfile)
	http.HandleFunc("/validate", handleValidate)
	http.HandleFunc("/status", handleStatus)
	http.HandleFunc("/version", handleVersion)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("Vici Dialer CDR Scrubber API starting on port %s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"name":        "Vici Dialer CDR Scrubber",
		"version":     version,
		"description": "Enterprise-grade CDR processing API",
		"endpoints": map[string]string{
			"GET /health":  "Health check",
			"GET /status":  "System status",
			"GET /version": "Version info",
			"POST /scrub":  "Scrub CDR data",
			"POST /profile":"Profile data quality",
			"POST /validate":"Validate CDR records",
		},
	}
	jsonResponse(w, response, http.StatusOK)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, map[string]string{"status": "healthy"}, http.StatusOK)
}

func handleVersion(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, map[string]string{
		"version":     version,
		"build_time":  time.Now().Format(time.RFC3339),
		"environment": os.Getenv("ENVIRONMENT"),
	}, http.StatusOK)
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	db, err := database.NewPostgresDB(cfg.Database)
	status := "disconnected"
	if err == nil {
		status = "connected"
		db.Close()
	}

	jsonResponse(w, map[string]interface{}{
		"database":  status,
		"fraud":     cfg.Fraud.Enabled,
		"enrichment": cfg.Enrichment.Enabled,
		"version":   version,
	}, http.StatusOK)
}

func handleScrub(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	db, err := database.NewPostgresDB(cfg.Database)
	if err != nil {
		jsonResponse(w, map[string]string{"error": "Database connection failed"}, http.StatusInternalServerError)
		return
	}
	defer db.Close()

	ctx := r.Context()
	s := scrubber.NewScrubber(cfg.Scrubber)
	fraudEngine := fraud.NewDetectionEngine(cfg.Fraud)
	validator := validation.NewValidator(cfg.Validation)
	enricher := enrichment.NewEnricher()

	cdrs, err := db.FetchCDRs(ctx, 0, cfg.Scrubber.BatchSize)
	if err != nil {
		jsonResponse(w, map[string]string{"error": "Failed to fetch CDRs"}, http.StatusInternalServerError)
		return
	}

	scrubbed := s.ProcessBatch(ctx, cdrs)

	for i := range scrubbed {
		alerts := fraudEngine.AnalyzeCDR(cdrs[i])
		if len(alerts) > 0 {
			scrubbed[i].FraudScore = alerts[0].Score
		}

		result := validator.ValidateCDR(cdrs[i])
		scrubbed[i].ValidationScore = result.Score

		enriched := enricher.EnrichCDR(scrubbed[i])
		scrubbed[i].CarrierName = enriched.CarrierName
		scrubbed[i].EnrichedCountry = enriched.Country
		scrubbed[i].EnrichedTimezone = enriched.Timezone
	}

	if err := db.InsertScrubbedCDRs(ctx, scrubbed); err != nil {
		jsonResponse(w, map[string]string{"error": "Failed to save results"}, http.StatusInternalServerError)
		return
	}

	stats := s.GetStats()
	jsonResponse(w, map[string]interface{}{
		"message":    "Scrubbing complete",
		"processed":  len(scrubbed),
		"stats":      stats,
	}, http.StatusOK)
}

func handleProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	db, err := database.NewPostgresDB(cfg.Database)
	if err != nil {
		jsonResponse(w, map[string]string{"error": "Database connection failed"}, http.StatusInternalServerError)
		return
	}
	defer db.Close()

	ctx := r.Context()
	p := profiler.NewDataProfiler()

	cdrs, err := db.FetchCDRs(ctx, 0, cfg.Scrubber.BatchSize)
	if err != nil {
		jsonResponse(w, map[string]string{"error": "Failed to fetch CDRs"}, http.StatusInternalServerError)
		return
	}

	stats := p.Profile(cdrs)
	jsonResponse(w, stats, http.StatusOK)
}

func handleValidate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	validator := validation.NewValidator(cfg.Validation)

	db, err := database.NewPostgresDB(cfg.Database)
	if err != nil {
		jsonResponse(w, map[string]string{"error": "Database connection failed"}, http.StatusInternalServerError)
		return
	}
	defer db.Close()

	ctx := r.Context()
	cdrs, err := db.FetchCDRs(ctx, 0, cfg.Scrubber.BatchSize)
	if err != nil {
		jsonResponse(w, map[string]string{"error": "Failed to fetch CDRs"}, http.StatusInternalServerError)
		return
	}

	results := validator.ValidateBatch(cdrs)
	summary := validator.GetValidationSummary(results)

	jsonResponse(w, summary, http.StatusOK)
}

func loadConfig(path string) (*models.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg models.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	return &cfg, nil
}

func jsonResponse(w http.ResponseWriter, data interface{}, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
