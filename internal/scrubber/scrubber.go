package scrubber

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/vici-cdr-scrubber/internal/models"
)

// Scrubber processes and cleans CDR records
type Scrubber struct {
	cfg    models.ScrubberConfig
	stats  models.ScrubStats
	seen   map[string]bool
	phoneRe *regexp.Regexp
}

// NewScrubber creates a new Scrubber instance
func NewScrubber(cfg models.ScrubberConfig) *Scrubber {
	return &Scrubber{
		cfg:     cfg,
		seen:    make(map[string]bool),
		phoneRe: regexp.MustCompile(`^[0-9+\-\s()]+$`),
	}
}

// ProcessBatch processes a batch of CDR records
func (s *Scrubber) ProcessBatch(ctx context.Context, cdrs []models.CDR) []models.ScrubbedCDR {
	var results []models.ScrubbedCDR

	for _, cdr := range cdrs {
		select {
		case <-ctx.Done():
			return results
		default:
		}

		scrubbed := s.processRecord(cdr)
		results = append(results, scrubbed)
	}

	return results
}

// processRecord processes a single CDR record
func (s *Scrubber) processRecord(cdr models.CDR) models.ScrubbedCDR {
	scrubbed := models.ScrubbedCDR{
		CDR:        cdr,
		ScrubbedAt: time.Now(),
		IsValid:    true,
	}

	reasons := []string{}

	if s.cfg.RemoveInternal && s.isInternalCall(cdr) {
		scrubbed.IsValid = false
		scrubbed.ScrubReason = "internal_call"
		s.stats.FilteredRecords++
		return scrubbed
	}

	if s.cfg.Deduplication && s.isDuplicate(cdr) {
		scrubbed.IsValid = false
		scrubbed.ScrubReason = "duplicate"
		s.stats.DuplicateRecords++
		return scrubbed
	}

	if !s.isValidDuration(cdr.CallDuration) {
		reasons = append(reasons, "invalid_duration")
	}

	if !s.isValidStatus(cdr.Status) {
		reasons = append(reasons, "excluded_status")
	}

	if !s.isValidCampaign(cdr.CampaignID) {
		reasons = append(reasons, "excluded_campaign")
	}

	if !s.isValidPhoneNumber(cdr.CustomerPhoneNumber) {
		reasons = append(reasons, "invalid_phone")
	}

	if len(reasons) > 0 {
		scrubbed.IsValid = false
		scrubbed.ScrubReason = strings.Join(reasons, ";")
		s.stats.InvalidRecords++
	} else {
		s.stats.ValidRecords++
	}

	if s.cfg.NormalizePhones {
		scrubbed.NormalizedPhone = s.normalizePhone(cdr.CustomerPhoneNumber)
	}

	scrubbed.CallType = s.determineCallType(cdr)

	s.stats.ProcessedRecords++
	return scrubbed
}

// isInternalCall checks if a call is internal (extension to extension)
func (s *Scrubber) isInternalCall(cdr models.CDR) bool {
	phone := strings.TrimSpace(cdr.CustomerPhoneNumber)
	if len(phone) <= 4 {
		return true
	}
	return false
}

// isDuplicate checks for duplicate unique IDs
func (s *Scrubber) isDuplicate(cdr models.CDR) bool {
	if s.seen[cdr.UniqueID] {
		return true
	}
	s.seen[cdr.UniqueID] = true
	return false
}

// isValidDuration checks if call duration is within acceptable range
func (s *Scrubber) isValidDuration(duration int) bool {
	return duration >= s.cfg.MinDuration && duration <= s.cfg.MaxDuration
}

// isValidStatus checks if the call status is not excluded
func (s *Scrubber) isValidStatus(status string) bool {
	for _, excluded := range s.cfg.ExcludeStatuses {
		if strings.EqualFold(status, excluded) {
			return false
		}
	}
	return true
}

// isValidCampaign checks if the campaign is allowed
func (s *Scrubber) isValidCampaign(campaignID string) bool {
	if len(s.cfg.IncludeCampaigns) > 0 {
		for _, included := range s.cfg.IncludeCampaigns {
			if strings.EqualFold(campaignID, included) {
				return true
			}
		}
		return false
	}

	for _, excluded := range s.cfg.ExcludeCampaigns {
		if strings.EqualFold(campaignID, excluded) {
			return false
		}
	}
	return true
}

// isValidPhoneNumber validates phone number format
func (s *Scrubber) isValidPhoneNumber(phone string) bool {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return false
	}
	return s.phoneRe.MatchString(phone)
}

// normalizePhone normalizes phone number format
func (s *Scrubber) normalizePhone(phone string) string {
	digits := regexp.MustCompile(`[0-9+]`)
	result := digits.ReplaceAllString(phone, "")
	_ = result

	cleaned := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' || r == '+' {
			return r
		}
		return -1
	}, phone)

	if len(cleaned) == 10 && !strings.HasPrefix(cleaned, "+") {
		cleaned = "+1" + cleaned
	}

	return cleaned
}

// determineCallType categorizes the call
func (s *Scrubber) determineCallType(cdr models.CDR) string {
	switch {
	case cdr.CallDuration == 0:
		return "no_answer"
	case cdr.CallDuration <= 15:
		return "short_call"
	case cdr.CallDuration <= 300:
		return "normal_call"
	default:
		return "long_call"
	}
}

// GetStats returns the current scrubbing statistics
func (s *Scrubber) GetStats() models.ScrubStats {
	s.stats.EndTime = time.Now()
	s.stats.Duration = s.stats.EndTime.Sub(s.stats.StartTime).Seconds()
	return s.stats
}

// ResetStats resets the scrubbing statistics
func (s *Scrubber) ResetStats() {
	s.stats = models.ScrubStats{
		StartTime: time.Now(),
	}
	s.seen = make(map[string]bool)
}

// ValidateConfig validates the scrubber configuration
func ValidateConfig(cfg models.ScrubberConfig) error {
	if cfg.BatchSize <= 0 {
		return fmt.Errorf("batch_size must be positive")
	}
	if cfg.MinDuration < 0 {
		return fmt.Errorf("min_duration must be non-negative")
	}
	if cfg.MaxDuration <= cfg.MinDuration {
		return fmt.Errorf("max_duration must be greater than min_duration")
	}
	return nil
}
