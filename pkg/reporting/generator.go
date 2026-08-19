package reporting

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/vici-cdr-scrubber/pkg/models"
)

// ReportGenerator generates various report formats
type ReportGenerator struct {
	outputDir string
}

// ReportConfig holds report generation configuration
type ReportConfig struct {
	OutputDir       string `yaml:"output_dir"`
	Format          string `yaml:"format"`
	IncludeHeaders  bool   `yaml:"include_headers"`
	IncludeSummary  bool   `yaml:"include_summary"`
	CompressOutput  bool   `yaml:"compress_output"`
}

// SummaryReport holds overall scrubbing summary
type SummaryReport struct {
	GeneratedAt      time.Time              `json:"generated_at"`
	TotalRecords     int64                  `json:"total_records"`
	ValidRecords     int64                  `json:"valid_records"`
	InvalidRecords   int64                  `json:"invalid_records"`
	DuplicateRecords int64                  `json:"duplicate_records"`
	FilteredRecords  int64                  `json:"filtered_records"`
	QualityScore     float64                `json:"quality_score"`
	FraudAlerts      int                    `json:"fraud_alerts"`
	EnrichmentRate   float64                `json:"enrichment_rate"`
	CampaignStats    []CampaignStat         `json:"campaign_stats,omitempty"`
	HourlyBreakdown  map[string]int64       `json:"hourly_breakdown,omitempty"`
	TopDispositions  []DispositionStat      `json:"top_dispositions,omitempty"`
	DurationStats    DurationStatistics     `json:"duration_stats"`
}

// CampaignStat holds per-campaign statistics
type CampaignStat struct {
	CampaignID    string  `json:"campaign_id"`
	TotalCalls    int64   `json:"total_calls"`
	AvgDuration   float64 `json:"avg_duration"`
	ConnectRate   float64 `json:"connect_rate"`
	ValidRate     float64 `json:"valid_rate"`
}

// DispositionStat holds call disposition statistics
type DispositionStat struct {
	Status  string  `json:"status"`
	Count   int64   `json:"count"`
	Percent float64 `json:"percent"`
}

// DurationStatistics holds duration analysis
type DurationStatistics struct {
	Average float64 `json:"average"`
	Median  float64 `json:"median"`
	Min     int     `json:"min"`
	Max     int     `json:"max"`
	P90     float64 `json:"p90"`
	P95     float64 `json:"p95"`
}

// NewReportGenerator creates a new report generator
func NewReportGenerator(config ReportConfig) *ReportGenerator {
	os.MkdirAll(config.OutputDir, 0755)
	return &ReportGenerator{outputDir: config.OutputDir}
}

// GenerateSummaryReport generates a summary report
func (rg *ReportGenerator) GenerateSummaryReport(stats models.ScrubStats, cdrs []models.ScrubbedCDR) (*SummaryReport, error) {
	report := &SummaryReport{
		GeneratedAt:      time.Now(),
		TotalRecords:     stats.TotalRecords,
		ValidRecords:     stats.ValidRecords,
		InvalidRecords:   stats.InvalidRecords,
		DuplicateRecords: stats.DuplicateRecords,
		FilteredRecords:  stats.FilteredRecords,
	}

	if stats.TotalRecords > 0 {
		report.QualityScore = float64(stats.ValidRecords) / float64(stats.TotalRecords) * 100
	}

	report.CampaignStats = rg.calculateCampaignStats(cdrs)
	report.HourlyBreakdown = rg.calculateHourlyBreakdown(cdrs)
	report.TopDispositions = rg.calculateDispositionStats(cdrs)
	report.DurationStats = rg.calculateDurationStats(cdrs)

	return report, nil
}

// calculateCampaignStats calculates per-campaign statistics
func (rg *ReportGenerator) calculateCampaignStats(cdrs []models.ScrubbedCDR) []CampaignStat {
	campaignMap := make(map[string]*CampaignStat)

	for _, cdr := range cdrs {
		stat, exists := campaignMap[cdr.CampaignID]
		if !exists {
			stat = &CampaignStat{CampaignID: cdr.CampaignID}
			campaignMap[cdr.CampaignID] = stat
		}
		stat.TotalCalls++
	}

	var stats []CampaignStat
	for _, stat := range campaignMap {
		stats = append(stats, *stat)
	}
	return stats
}

// calculateHourlyBreakdown calculates call distribution by hour
func (rg *ReportGenerator) calculateHourlyBreakdown(cdrs []models.ScrubbedCDR) map[string]int64 {
	breakdown := make(map[string]int64)
	for i := 0; i < 24; i++ {
		breakdown[fmt.Sprintf("%02d:00", i)] = 0
	}

	for _, cdr := range cdrs {
		hour := fmt.Sprintf("%02d:00", cdr.CallDate.Hour())
		breakdown[hour]++
	}

	return breakdown
}

// calculateDispositionStats calculates disposition statistics
func (rg *ReportGenerator) calculateDispositionStats(cdrs []models.ScrubbedCDR) []DispositionStat {
	statusMap := make(map[string]int64)
	total := int64(len(cdrs))

	for _, cdr := range cdrs {
		statusMap[cdr.Status]++
	}

	var stats []DispositionStat
	for status, count := range statusMap {
		stats = append(stats, DispositionStat{
			Status:  status,
			Count:   count,
			Percent: float64(count) / float64(total) * 100,
		})
	}
	return stats
}

// calculateDurationStats calculates duration statistics
func (rg *ReportGenerator) calculateDurationStats(cdrs []models.ScrubbedCDR) DurationStatistics {
	if len(cdrs) == 0 {
		return DurationStatistics{}
	}

	var durations []int
	totalDuration := 0
	minDur := cdrs[0].CallDuration
	maxDur := cdrs[0].CallDuration

	for _, cdr := range cdrs {
		durations = append(durations, cdr.CallDuration)
		totalDuration += cdr.CallDuration
		if cdr.CallDuration < minDur {
			minDur = cdr.CallDuration
		}
		if cdr.CallDuration > maxDur {
			maxDur = cdr.CallDuration
		}
	}

	for i := 0; i < len(durations)-1; i++ {
		for j := i + 1; j < len(durations); j++ {
			if durations[j] < durations[i] {
				durations[i], durations[j] = durations[j], durations[i]
			}
		}
	}

	avg := float64(totalDuration) / float64(len(cdrs))
	median := float64(durations[len(durations)/2])
	p90 := float64(durations[int(float64(len(durations))*0.9)])
	p95 := float64(durations[int(float64(len(durations))*0.95)])

	return DurationStatistics{
		Average: avg,
		Median:  median,
		Min:     minDur,
		Max:     maxDur,
		P90:     p90,
		P95:     p95,
	}
}

// ExportCSV exports scrubbed CDRs to CSV
func (rg *ReportGenerator) ExportCSV(cdrs []models.ScrubbedCDR, filename string) error {
	path := filepath.Join(rg.outputDir, filename)
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating CSV file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	header := []string{
		"unique_id", "call_date", "caller_code", "customer_phone_number",
		"phone_code", "lead_id", "campaign_id", "list_id", "status",
		"call_duration", "is_valid", "scrub_reason", "normalized_phone",
	}
	writer.Write(header)

	for _, cdr := range cdrs {
		row := []string{
			cdr.UniqueID,
			cdr.CallDate.Format(time.RFC3339),
			cdr.CallerCode,
			cdr.CustomerPhoneNumber,
			cdr.PhoneCode,
			fmt.Sprintf("%d", cdr.LeadID),
			cdr.CampaignID,
			fmt.Sprintf("%d", cdr.ListID),
			cdr.Status,
			fmt.Sprintf("%d", cdr.CallDuration),
			fmt.Sprintf("%t", cdr.IsValid),
			cdr.ScrubReason,
			cdr.NormalizedPhone,
		}
		writer.Write(row)
	}

	return nil
}

// ExportJSON exports scrubbed CDRs to JSON
func (rg *ReportGenerator) ExportJSON(cdrs []models.ScrubbedCDR, filename string) error {
	path := filepath.Join(rg.outputDir, filename)
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating JSON file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(cdrs)
}

// ExportSummaryJSON exports summary report to JSON
func (rg *ReportGenerator) ExportSummaryJSON(report *SummaryReport, filename string) error {
	path := filepath.Join(rg.outputDir, filename)
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating JSON file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

// PrintSummary prints a formatted summary to console
func (rg *ReportGenerator) PrintSummary(report *SummaryReport) {
	fmt.Println("\n========================================")
	fmt.Println("       CDR SCRUBBING SUMMARY REPORT     ")
	fmt.Println("========================================")
	fmt.Printf("Generated: %s\n\n", report.GeneratedAt.Format(time.RFC3339))

	fmt.Println("--- Record Statistics ---")
	fmt.Printf("  Total Records:     %d\n", report.TotalRecords)
	fmt.Printf("  Valid Records:     %d\n", report.ValidRecords)
	fmt.Printf("  Invalid Records:   %d\n", report.InvalidRecords)
	fmt.Printf("  Duplicate Records: %d\n", report.DuplicateRecords)
	fmt.Printf("  Filtered Records:  %d\n", report.FilteredRecords)
	fmt.Printf("  Quality Score:     %.2f%%\n", report.QualityScore)

	fmt.Println("\n--- Duration Statistics ---")
	fmt.Printf("  Average: %.1f seconds\n", report.DurationStats.Average)
	fmt.Printf("  Median:  %.1f seconds\n", report.DurationStats.Median)
	fmt.Printf("  Min:     %d seconds\n", report.DurationStats.Min)
	fmt.Printf("  Max:     %d seconds\n", report.DurationStats.Max)
	fmt.Printf("  P90:     %.1f seconds\n", report.DurationStats.P90)
	fmt.Printf("  P95:     %.1f seconds\n", report.DurationStats.P95)

	if len(report.TopDispositions) > 0 {
		fmt.Println("\n--- Top Dispositions ---")
		for i, d := range report.TopDispositions {
			if i >= 10 {
				break
			}
			fmt.Printf("  %-20s: %d (%.1f%%)\n", d.Status, d.Count, d.Percent)
		}
	}

	fmt.Println("\n========================================")
}
