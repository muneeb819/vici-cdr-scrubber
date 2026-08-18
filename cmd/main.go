package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
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
	version   = "2.0.0"
	buildTime = "unknown"
)

func main() {
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	showVersion := flag.Bool("version", false, "Show version information")
	dryRun := flag.Bool("dry-run", false, "Run without writing to database")
	profileOnly := flag.Bool("profile", false, "Run data profiling only")
	noFraud := flag.Bool("no-fraud", false, "Disable fraud detection")
	noEnrich := flag.Bool("no-enrich", false, "Disable data enrichment")
	flag.Parse()

	if *showVersion {
		fmt.Printf("vici-cdr-scrubber %s (built %s)\n", version, buildTime)
		fmt.Println("Features: Fraud Detection, Data Profiling, Geo Analysis, Link Analysis,")
		fmt.Println("          Real-time Validation, Data Enrichment, Advanced Reporting")
		os.Exit(0)
	}

	cfg, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	printBanner()

	db, err := database.NewPostgresDB(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	fmt.Println("[OK] Connected to database")

	dataProfiler := profiler.NewDataProfiler()
	fmt.Println("[OK] Data profiler initialized")

	var fraudEngine *fraud.DetectionEngine
	if cfg.Fraud.Enabled && !*noFraud {
		fraudEngine = fraud.NewDetectionEngine(cfg.Fraud)
		fmt.Println("[OK] Fraud detection engine enabled")
	}

	var validator *validation.Validator
	if cfg.Validation.StrictMode || len(cfg.Validation.ValidStatuses) > 0 {
		validator = validation.NewValidator(cfg.Validation)
		fmt.Println("[OK] Real-time validator enabled")
	}

	var enricher *enrichment.Enricher
	if cfg.Enrichment.Enabled && !*noEnrich {
		enricher = enrichment.NewEnricher()
		fmt.Println("[OK] Data enrichment enabled")
	}

	s := scrubber.NewScrubber(cfg.Scrubber)
	s.ResetStats()
	fmt.Println("[OK] Scrubber initialized")

	totalCount, err := db.GetTotalCount(ctx)
	if err != nil {
		log.Fatalf("Failed to get record count: %v", err)
	}
	fmt.Printf("\n[INFO] Total CDR records to process: %d\n\n", totalCount)

	if *profileOnly {
		runProfilingOnly(ctx, db, dataProfiler, totalCount, cfg.Scrubber.BatchSize)
		return
	}

	offset := 0
	batchNum := 0
	allScrubbed := []models.ScrubbedCDR{}
	fraudAlertCount := 0
	validationErrorCount := 0
	enrichedCount := 0

	startTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("\n[INFO] Shutdown requested, finishing current batch...")
			goto done
		default:
		}

		cdrs, err := db.FetchCDRs(ctx, offset, cfg.Scrubber.BatchSize)
		if err != nil {
			log.Fatalf("Failed to fetch CDRs: %v", err)
		}

		if len(cdrs) == 0 {
			break
		}

		batchNum++
		fmt.Printf("[BATCH %d] Processing records %d-%d...\n", batchNum, offset+1, offset+len(cdrs))

		dataProfiler.Profile(cdrs)

		scrubbed := s.ProcessBatch(ctx, cdrs)

		for i := range scrubbed {
			if fraudEngine != nil {
				alerts := fraudEngine.AnalyzeCDR(cdrs[i])
				if len(alerts) > 0 {
					scrubbed[i].FraudScore = alerts[0].Score
					for _, a := range alerts {
						scrubbed[i].FraudAlerts = append(scrubbed[i].FraudAlerts, a.AlertType)
					}
					fraudAlertCount += len(alerts)
				}
			}

			if validator != nil {
				result := validator.ValidateCDR(cdrs[i])
				scrubbed[i].ValidationScore = result.Score
				if !result.IsValid {
					validationErrorCount++
				}
			}

			if enricher != nil {
				enriched := enricher.EnrichCDR(scrubbed[i])
				scrubbed[i].CarrierName = enriched.CarrierName
				scrubbed[i].EnrichedCountry = enriched.Country
				scrubbed[i].EnrichedTimezone = enriched.Timezone
				scrubbed[i].EnrichedLineType = enriched.LineType
				scrubbed[i].IsInternational = enriched.IsInternational
				scrubbed[i].IsTollFree = enriched.IsTollFree
				scrubbed[i].IsMobile = enriched.IsMobile
				if enriched.Country != "" {
					enrichedCount++
				}
			}
		}

		allScrubbed = append(allScrubbed, scrubbed...)

		if !*dryRun {
			if err := db.InsertScrubbedCDRs(ctx, scrubbed); err != nil {
				log.Printf("[WARN] Failed to insert batch %d: %v", batchNum, err)
			}
		}

		offset += len(cdrs)

		if offset >= int(totalCount) {
			break
		}
	}

done:
	stats := s.GetStats()
	stats.TotalRecords = totalCount
	stats.FraudDetected = fraudAlertCount
	stats.EnrichedRecords = enrichedCount
	stats.ValidationErrors = validationErrorCount

	if stats.TotalRecords > 0 {
		stats.QualityScore = float64(stats.ValidRecords) / float64(stats.TotalRecords) * 100
	}

	reportGen := reporting.NewReportGenerator(reporting.ReportConfig{
		OutputDir:      cfg.Output.Directory,
		Format:         cfg.Output.Format,
		IncludeHeaders: cfg.Output.IncludeHeaders,
	})

	summaryReport, err := reportGen.GenerateSummaryReport(stats, allScrubbed)
	if err != nil {
		log.Printf("[WARN] Failed to generate summary report: %v", err)
	} else {
		reportGen.PrintSummary(summaryReport)
		reportGen.ExportSummaryJSON(summaryReport, "summary_report.json")
	}

	dataProfiler.PrintReport()

	if len(allScrubbed) > 0 {
		reportGen.ExportCSV(allScrubbed, fmt.Sprintf("cdr_scrubbed_%s.csv", time.Now().Format("20060102_150405")))
	}

	if fraudEngine != nil {
		topCallers := fraudEngine.GetTopCallers(10)
		if len(topCallers) > 0 {
			fmt.Println("\n=== Top Callers by Volume ===")
			for _, caller := range topCallers {
				fmt.Printf("  %s: %d calls, %d unique targets\n",
					caller.PhoneNumber, caller.TotalCalls, len(caller.UniqueTargets))
			}
		}
	}

	fmt.Printf("\n[DONE] Scrubbing complete in %.2f seconds\n", time.Since(startTime).Seconds())
}

func runProfilingOnly(ctx context.Context, db *database.PostgresDB, p *profiler.DataProfiler, totalCount int64, batchSize int) {
	fmt.Println("=== Running Data Profiling Only ===\n")

	offset := 0
	for {
		select {
		case <-ctx.Done():
			break
		default:
		}

		cdrs, err := db.FetchCDRs(ctx, offset, batchSize)
		if err != nil {
			log.Fatalf("Failed to fetch CDRs: %v", err)
		}

		if len(cdrs) == 0 {
			break
		}

		p.Profile(cdrs)
		offset += len(cdrs)

		if offset >= int(totalCount) {
			break
		}
	}

	p.PrintReport()

	timeDist := p.GetTimeDistribution(nil)
	_ = timeDist
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

	setDefaults(&cfg)
	return &cfg, nil
}

func setDefaults(cfg *models.Config) {
	if cfg.Database.Port == 0 {
		cfg.Database.Port = 5432
	}
	if cfg.Database.SSLMode == "" {
		cfg.Database.SSLMode = "disable"
	}
	if cfg.Database.MaxOpenConns == 0 {
		cfg.Database.MaxOpenConns = 10
	}
	if cfg.Database.MaxIdleConns == 0 {
		cfg.Database.MaxIdleConns = 5
	}
	if cfg.Scrubber.BatchSize == 0 {
		cfg.Scrubber.BatchSize = 1000
	}
	if cfg.Scrubber.MaxDuration == 0 {
		cfg.Scrubber.MaxDuration = 86400
	}
	if cfg.Fraud.HighVolumeThreshold == 0 {
		cfg.Fraud.HighVolumeThreshold = 100
	}
	if cfg.Fraud.ShortCallThreshold == 0 {
		cfg.Fraud.ShortCallThreshold = 5
	}
	if cfg.Fraud.NightCallStartHour == 0 {
		cfg.Fraud.NightCallStartHour = 22
	}
	if cfg.Fraud.NightCallEndHour == 0 {
		cfg.Fraud.NightCallEndHour = 6
	}
	if cfg.Fraud.GeoVelocityKmh == 0 {
		cfg.Fraud.GeoVelocityKmh = 500
	}
	if cfg.Fraud.AnomalyScoreThreshold == 0 {
		cfg.Fraud.AnomalyScoreThreshold = 0.6
	}
	if cfg.Validation.MaxDuration == 0 {
		cfg.Validation.MaxDuration = 86400
	}
	if cfg.Output.Format == "" {
		cfg.Output.Format = "csv"
	}
	if cfg.Output.Directory == "" {
		cfg.Output.Directory = "./output"
	}
}

func printBanner() {
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║          VICI DIALER CDR SCRUBBER v2.0.0                   ║")
	fmt.Println("║          Enterprise-Grade CDR Processing                   ║")
	fmt.Println("╠══════════════════════════════════════════════════════════════╣")
	fmt.Println("║  Features:                                                 ║")
	fmt.Println("║  • Fraud Detection & Anomaly Analysis                     ║")
	fmt.Println("║  • Data Profiling & Quality Scoring                       ║")
	fmt.Println("║  • Real-time Validation                                   ║")
	fmt.Println("║  • Data Enrichment (Carrier, Timezone, Geo)              ║")
	fmt.Println("║  • Advanced Reporting (CSV, JSON)                         ║")
	fmt.Println("║  • Deduplication & Phone Normalization                    ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Println()
}
