package profiler

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/vici-cdr-scrubber/pkg/models"
)

// DataProfiler analyzes CDR data quality and patterns
type DataProfiler struct {
	mu          sync.RWMutex
	stats       *ProfileStats
	rules       []QualityRule
}

// ProfileStats holds data profiling statistics
type ProfileStats struct {
	TotalRecords      int64                  `json:"total_records"`
	FieldCompleteness  map[string]float64     `json:"field_completeness"`
	FieldDistribution  map[string]map[string]int `json:"field_distribution"`
	NullCounts         map[string]int64       `json:"null_counts"`
	UniqueCounts       map[string]int64       `json:"unique_counts"`
	MinValues          map[string]interface{} `json:"min_values"`
	MaxValues          map[string]interface{} `json:"max_values"`
	AvgValues          map[string[float64]    `json:"avg_values"`
	AnomalyScores      map[string]float64     `json:"anomaly_scores"`
	QualityScore       float64               `json:"quality_score"`
	IssuesFound        []QualityIssue        `json:"issues_found"`
	GeneratedAt        time.Time             `json:"generated_at"`
}

// QualityRule defines a data quality check
type QualityRule struct {
	Name        string
	Description string
	Field       string
	Validate    func(value interface{}) bool
	Severity    string
}

// QualityIssue represents a detected data quality issue
type QualityIssue struct {
	Rule        string  `json:"rule"`
	Field       string  `json:"field"`
	Count       int64   `json:"count"`
	Percentage  float64 `json:"percentage"`
	Severity    string  `json:"severity"`
	Suggestion  string  `json:"suggestion"`
}

// TimeDistribution holds call distribution by time period
type TimeDistribution struct {
	Hourly   [24]int64
	Daily    [7]int64
	Monthly  [12]int64
}

// NewDataProfiler creates a new data profiler
func NewDataProfiler() *DataProfiler {
	profiler := &DataProfiler{
		stats: &ProfileStats{
			FieldCompleteness:  make(map[string]float64),
			FieldDistribution:  make(map[string]map[string]int),
			NullCounts:         make(map[string]int64),
			UniqueCounts:       make(map[string]int64),
			MinValues:          make(map[string]interface{}),
			MaxValues:          make(map[string]interface{}),
			AvgValues:          make(map[string[float64]),
			AnomalyScores:      make(map[string]float64),
			IssuesFound:        []QualityIssue{},
		},
	}
	profiler.initDefaultRules()
	return profiler
}

// initDefaultRules sets up default quality rules
func (p *DataProfiler) initDefaultRules() {
	p.rules = []QualityRule{
		{
			Name:        "empty_phone",
			Description: "Phone number is empty or null",
			Field:       "customer_phone_number",
			Validate:    func(v interface{}) bool { return v != nil && v.(string) != "" },
			Severity:    "high",
		},
		{
			Name:        "invalid_duration",
			Description: "Call duration is negative",
			Field:       "call_duration",
			Validate:    func(v interface{}) bool { return v.(int) >= 0 },
			Severity:    "high",
		},
		{
			Name:        "future_date",
			Description: "Call date is in the future",
			Field:       "call_date",
			Validate:    func(v interface{}) bool { return v.(time.Time).Before(time.Now().Add(time.Hour)) },
			Severity:    "medium",
		},
		{
			Name:        "empty_campaign",
			Description: "Campaign ID is empty",
			Field:       "campaign_id",
			Validate:    func(v interface{}) bool { return v != nil && v.(string) != "" },
			Severity:    "low",
		},
		{
			Name:        "empty_status",
			Description: "Call status is empty",
			Field:       "status",
			Validate:    func(v interface{}) bool { return v != nil && v.(string) != "" },
			Severity:    "medium",
		},
	}
}

// Profile analyzes a batch of CDR records
func (p *DataProfiler) Profile(cdrs []models.CDR) *ProfileStats {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.stats.TotalRecords += int64(len(cdrs))
	p.stats.GeneratedAt = time.Now()

	for _, cdr := range cdrs {
		p.analyzeRecord(cdr)
	}

	p.calculateCompleteness()
	p.detectIssues()
	p.calculateQualityScore()

	return p.stats
}

// analyzeRecord analyzes a single CDR record
func (p *DataProfiler) analyzeRecord(cdr models.CDR) {
	p.updateFieldStats("uniqueid", cdr.UniqueID)
	p.updateFieldStats("caller_code", cdr.CallerCode)
	p.updateFieldStats("customer_phone_number", cdr.CustomerPhoneNumber)
	p.updateFieldStats("phone_code", cdr.PhoneCode)
	p.updateFieldStats("campaign_id", cdr.CampaignID)
	p.updateFieldStats("status", cdr.Status)
	p.updateFieldStats("user", cdr.User)

	p.updateNumericStats("call_duration", float64(cdr.CallDuration))
	p.updateNumericStats("park_time", float64(cdr.ParkTime))
	p.updateNumericStats("called_count", float64(cdr.CalledCount))

	if cdr.CallDate.Before(p.getMinTime("call_date")) || p.stats.MinValues["call_date"] == nil {
		p.stats.MinValues["call_date"] = cdr.CallDate
	}
	if cdr.CallDate.After(p.getMaxTime("call_date")) {
		p.stats.MaxValues["call_date"] = cdr.CallDate
	}
}

// updateFieldStats updates statistics for a string field
func (p *DataProfiler) updateFieldStats(field, value string) {
	if p.stats.FieldDistribution[field] == nil {
		p.stats.FieldDistribution[field] = make(map[string]int)
	}

	if value == "" {
		p.stats.NullCounts[field]++
	} else {
		p.stats.FieldDistribution[field][value]++
		p.stats.UniqueCounts[field]++
	}
}

// updateNumericStats updates statistics for a numeric field
func (p *DataProfiler) updateNumericStats(field string, value float64) {
	if p.stats.MinValues[field] == nil || value < p.stats.MinValues[field].(float64) {
		p.stats.MinValues[field] = value
	}
	if p.stats.MaxValues[field] == nil || value > p.stats.MaxValues[field].(float64) {
		p.stats.MaxValues[field] = value
	}

	current := p.stats.AvgValues[field]
	count := current["count"]
	current["sum"] += value
	current["count"] = count + 1
	current["avg"] = current["sum"] / current["count"]
	p.stats.AvgValues[field] = current
}

// calculateCompleteness calculates field completeness percentages
func (p *DataProfiler) calculateCompleteness() {
	for field, nullCount := range p.stats.NullCounts {
		total := p.stats.TotalRecords
		if total > 0 {
			p.stats.FieldCompleteness[field] = float64(total-nullCount) / float64(total) * 100
		}
	}
}

// detectIssues detects data quality issues
func (p *DataProfiler) detectIssues() {
	for _, rule := range p.rules {
		count := int64(0)
		for field, distribution := range p.stats.FieldDistribution {
			if field == rule.Field {
				for value := range distribution {
					if !rule.Validate(value) {
						count++
					}
				}
			}
		}

		if count > 0 {
			percentage := float64(count) / float64(p.stats.TotalRecords) * 100
			issue := QualityIssue{
				Rule:       rule.Name,
				Field:      rule.Field,
				Count:      count,
				Percentage: percentage,
				Severity:   rule.Severity,
				Suggestion: fmt.Sprintf("Review and fix %s records", rule.Field),
			}
			p.stats.IssuesFound = append(p.stats.IssuesFound, issue)
		}
	}
}

// calculateQualityScore calculates overall data quality score
func (p *DataProfiler) calculateQualityScore() {
	if p.stats.TotalRecords == 0 {
		p.stats.QualityScore = 0
		return
	}

	totalScore := 0.0
	ruleCount := 0

	for _, issue := range p.stats.IssuesFound {
		penalty := issue.Percentage / 100
		switch issue.Severity {
		case "high":
			penalty *= 2
		case "medium":
			penalty *= 1.5
		}
		totalScore += (1 - penalty)
		ruleCount++
	}

	if ruleCount > 0 {
		p.stats.QualityScore = totalScore / float64(ruleCount) * 100
	} else {
		p.stats.QualityScore = 100
	}
}

// GetTimeDistribution returns call distribution by time
func (p *DataProfiler) GetTimeDistribution(cdrs []models.CDR) *TimeDistribution {
	dist := &TimeDistribution{}

	for _, cdr := range cdrs {
		dist.Hourly[cdr.CallDate.Hour()]++
		dist.Daily[cdr.CallDate.Weekday()]++
		dist.Monthly[cdr.CallDate.Month()-1]++
	}

	return dist
}

// GetTopValues returns top N values for a field
func (p *DataProfiler) GetTopValues(field string, n int) []FieldValueCount {
	p.mu.RLock()
	defer p.mu.RUnlock()

	distribution := p.stats.FieldDistribution[field]
	if distribution == nil {
		return nil
	}

	var counts []FieldValueCount
	for value, count := range distribution {
		counts = append(counts, FieldValueCount{Value: value, Count: count})
	}

	sort.Slice(counts, func(i, j int) bool {
		return counts[i].Count > counts[j].Count
	})

	if n > len(counts) {
		n = len(counts)
	}
	return counts[:n]
}

// GetAnomalyScores calculates anomaly scores for fields
func (p *DataProfiler) GetAnomalyScores() map[string]float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()

	scores := make(map[string]float64)
	for field, completeness := range p.stats.FieldCompleteness {
		scores[field] = 100 - completeness
	}
	return scores
}

func (p *DataProfiler) getMinTime(field string) time.Time {
	if val, ok := p.stats.MinValues[field]; ok && val != nil {
		return val.(time.Time)
	}
	return time.Now()
}

func (p *DataProfiler) getMaxTime(field string) time.Time {
	if val, ok := p.stats.MaxValues[field]; ok && val != nil {
		return val.(time.Time)
	}
	return time.Time{}
}

// FieldValuePair holds a field value and its count
type FieldValueCount struct {
	Value string
	Count int
}

// PrintReport prints a formatted profiling report
func (p *DataProfiler) PrintReport() {
	p.mu.RLock()
	defer p.mu.RUnlock()

	fmt.Println("\n=== Data Quality Profile Report ===")
	fmt.Printf("Total Records Analyzed: %d\n", p.stats.TotalRecords)
	fmt.Printf("Overall Quality Score: %.2f%%\n\n", p.stats.QualityScore)

	fmt.Println("--- Field Completeness ---")
	for field, completeness := range p.stats.FieldCompleteness {
		fmt.Printf("  %-30s: %.2f%%\n", field, completeness)
	}

	fmt.Println("\n--- Data Quality Issues ---")
	for _, issue := range p.stats.IssuesFound {
		fmt.Printf("  [%s] %s: %d records (%.2f%%)\n",
			issue.Severity, issue.Rule, issue.Count, issue.Percentage)
	}

	if p.stats.TotalRecords > 0 {
		fmt.Println("\n--- Numeric Summaries ---")
		for field, avg := range p.stats.AvgValues {
			if avg["count"] > 0 {
				fmt.Printf("  %-30s: avg=%.2f, min=%.2f, max=%.2f\n",
					field, avg["avg"],
					p.stats.MinValues[field],
					p.stats.MaxValues[field])
			}
		}
	}
}

// Reset resets the profiler statistics
func (p *DataProfiler) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.stats = &ProfileStats{
		FieldCompleteness:  make(map[string]float64),
		FieldDistribution:  make(map[string]map[string]int),
		NullCounts:         make(map[string]int64),
		UniqueCounts:       make(map[string]int64),
		MinValues:          make(map[string]interface{}),
		MaxValues:          make(map[string]interface{}),
		AvgValues:          make(map[string]float64),
		AnomalyScores:      make(map[string]float64),
		IssuesFound:        []QualityIssue{},
	}
}

// GetStats returns current profiling statistics
func (p *DataProfiler) GetStats() *ProfileStats {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.stats
}

// CalculateFieldEntropy calculates Shannon entropy for a field
func (p *DataProfiler) CalculateFieldEntropy(field string) float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()

	distribution := p.stats.FieldDistribution[field]
	if distribution == nil {
		return 0
	}

	total := 0
	for _, count := range distribution {
		total += count
	}

	if total == 0 {
		return 0
	}

	entropy := 0.0
	for _, count := range distribution {
		if count > 0 {
			prob := float64(count) / float64(total)
			entropy -= prob * math.Log2(prob)
		}
	}
	return entropy
}
