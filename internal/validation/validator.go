package validation

import (
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/vici-cdr-scrubber/internal/models"
)

// Validator performs real-time validation on CDR records
type Validator struct {
	mu          sync.RWMutex
	rules       []ValidationRule
	ruleResults map[string]*RuleResult
}

// ValidationRule defines a validation check
type ValidationRule struct {
	ID          string
	Name        string
	Description string
	Category    string
	Severity    string
	Validate    func(cdr models.CDR) ValidationError
}

// ValidationError represents a validation failure
type ValidationError struct {
	RuleID    string `json:"rule_id"`
	Field     string `json:"field"`
	Value     string `json:"value"`
	Message   string `json:"message"`
	Severity  string `json:"severity"`
}

// ValidationResult holds the result of validating a CDR
type ValidationResult struct {
	IsValid      bool               `json:"is_valid"`
	Errors       []ValidationError  `json:"errors"`
	Warnings     []ValidationError  `json:"warnings"`
	Score        float64            `json:"score"`
	ValidatedAt  time.Time          `json:"validated_at"`
}

// RuleResult tracks rule execution statistics
type RuleResult struct {
	RuleID        string  `json:"rule_id"`
	TotalChecked  int64   `json:"total_checked"`
	Passed        int64   `json:"passed"`
	Failed        int64   `json:"failed"`
	PassRate      float64 `json:"pass_rate"`
}

// ValidatorConfig holds validator configuration
type ValidatorConfig struct {
	StrictMode     bool     `yaml:"strict_mode"`
	PhoneRegex     string   `yaml:"phone_regex"`
	MaxDuration    int      `yaml:"max_duration"`
	MinDuration    int      `yaml:"min_duration"`
	ValidStatuses  []string `yaml:"valid_statuses"`
	ValidCampaigns []string `yaml:"valid_campaigns"`
}

// NewValidator creates a new validator
func NewValidator(config ValidatorConfig) *Validator {
	v := &Validator{
		ruleResults: make(map[string]*RuleResult),
	}
	v.initDefaultRules(config)
	return v
}

// initDefaultRules sets up default validation rules
func (v *Validator) initDefaultRules(config ValidatorConfig) {
	v.rules = []ValidationRule{
		{
			ID:          "phone_format",
			Name:        "Phone Number Format",
			Description: "Validates phone number format",
			Category:    "format",
			Severity:    "high",
			Validate: func(cdr models.CDR) ValidationError {
				if cdr.CustomerPhoneNumber == "" {
					return ValidationError{
						RuleID:   "phone_format",
						Field:    "customer_phone_number",
						Value:    cdr.CustomerPhoneNumber,
						Message:  "Phone number is empty",
						Severity: "high",
					}
				}
				phoneRegex := regexp.MustCompile(`^[0-9+\-\s()]{7,20}$`)
				if !phoneRegex.MatchString(cdr.CustomerPhoneNumber) {
					return ValidationError{
						RuleID:   "phone_format",
						Field:    "customer_phone_number",
						Value:    cdr.CustomerPhoneNumber,
						Message:  "Invalid phone number format",
						Severity: "high",
					}
				}
				return ValidationError{}
			},
		},
		{
			ID:          "duration_range",
			Name:        "Duration Range",
			Description: "Validates call duration is within acceptable range",
			Category:    "range",
			Severity:    "medium",
			Validate: func(cdr models.CDR) ValidationError {
				if cdr.CallDuration < config.MinDuration {
					return ValidationError{
						RuleID:   "duration_range",
						Field:    "call_duration",
						Value:    string(rune(cdr.CallDuration)),
						Message:  "Duration below minimum threshold",
						Severity: "medium",
					}
				}
				if cdr.CallDuration > config.MaxDuration {
					return ValidationError{
						RuleID:   "duration_range",
						Field:    "call_duration",
						Value:    string(rune(cdr.CallDuration)),
						Message:  "Duration exceeds maximum threshold",
						Severity: "medium",
					}
				}
				return ValidationError{}
			},
		},
		{
			ID:          "valid_status",
			Name:        "Valid Status",
			Description: "Validates call status is recognized",
			Category:    "reference",
			Severity:    "low",
			Validate: func(cdr models.CDR) ValidationError {
				if len(config.ValidStatuses) > 0 {
					valid := false
					for _, s := range config.ValidStatuses {
						if strings.EqualFold(cdr.Status, s) {
							valid = true
							break
						}
					}
					if !valid {
						return ValidationError{
							RuleID:   "valid_status",
							Field:    "status",
							Value:    cdr.Status,
							Message:  "Unrecognized call status",
							Severity: "low",
						}
					}
				}
				return ValidationError{}
			},
		},
		{
			ID:          "future_date",
			Name:        "Future Date Check",
			Description: "Validates call date is not in the future",
			Category:    "temporal",
			Severity:    "high",
			Validate: func(cdr models.CDR) ValidationError {
				if cdr.CallDate.After(time.Now().Add(time.Hour)) {
					return ValidationError{
						RuleID:   "future_date",
						Field:    "call_date",
						Value:    cdr.CallDate.String(),
						Message:  "Call date is in the future",
						Severity: "high",
					}
				}
				return ValidationError{}
			},
		},
		{
			ID:          "unique_id_format",
			Name:        "Unique ID Format",
			Description: "Validates unique ID is properly formatted",
			Category:    "format",
			Severity:    "high",
			Validate: func(cdr models.CDR) ValidationError {
				if cdr.UniqueID == "" {
					return ValidationError{
						RuleID:   "unique_id_format",
						Field:    "uniqueid",
						Value:    cdr.UniqueID,
						Message:  "Unique ID is empty",
						Severity: "high",
					}
				}
				return ValidationError{}
			},
		},
		{
			ID:          "caller_code_format",
			Name:        "Caller Code Format",
			Description: "Validates caller code is properly formatted",
			Category:    "format",
			Severity:    "medium",
			Validate: func(cdr models.CDR) ValidationError {
				if cdr.CallerCode == "" {
					return ValidationError{
						RuleID:   "caller_code_format",
						Field:    "caller_code",
						Value:    cdr.CallerCode,
						Message:  "Caller code is empty",
						Severity: "medium",
					}
				}
				return ValidationError{}
			},
		},
	}
}

// ValidateCDR validates a single CDR record
func (v *Validator) ValidateCDR(cdr models.CDR) ValidationResult {
	v.mu.Lock()
	defer v.mu.Unlock()

	result := ValidationResult{
		IsValid:     true,
		ValidatedAt: time.Now(),
		Score:       100.0,
	}

	for _, rule := range v.rules {
		error := rule.Validate(cdr)

		if error.Message != "" {
			v.updateRuleResult(rule.ID, false)

			if rule.Severity == "high" || rule.Severity == "critical" {
				result.Errors = append(result.Errors, error)
				result.IsValid = false
				result.Score -= 20
			} else {
				result.Warnings = append(result.Warnings, error)
				result.Score -= 5
			}
		} else {
			v.updateRuleResult(rule.ID, true)
		}
	}

	if result.Score < 0 {
		result.Score = 0
	}

	return result
}

// ValidateBatch validates a batch of CDR records
func (v *Validator) ValidateBatch(cdrs []models.CDR) []ValidationResult {
	results := make([]ValidationResult, len(cdrs))
	for i, cdr := range cdrs {
		results[i] = v.ValidateCDR(cdr)
	}
	return results
}

// updateRuleResult updates rule execution statistics
func (v *Validator) updateRuleResult(ruleID string, passed bool) {
	if _, exists := v.ruleResults[ruleID]; !exists {
		v.ruleResults[ruleID] = &RuleResult{RuleID: ruleID}
	}

	result := v.ruleResults[ruleID]
	result.TotalChecked++
	if passed {
		result.Passed++
	} else {
		result.Failed++
	}

	if result.TotalChecked > 0 {
		result.PassRate = float64(result.Passed) / float64(result.TotalChecked) * 100
	}
}

// GetRuleResults returns all rule execution statistics
func (v *Validator) GetRuleResults() map[string]*RuleResult {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.ruleResults
}

// GetRules returns all validation rules
func (v *Validator) GetRules() []ValidationRule {
	return v.rules
}

// AddRule adds a custom validation rule
func (v *Validator) AddRule(rule ValidationRule) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.rules = append(v.rules, rule)
}

// GetValidationSummary returns a summary of validation results
func (v *Validator) GetValidationSummary(results []ValidationResult) map[string]interface{} {
	total := len(results)
	valid := 0
	invalid := 0
	totalErrors := 0
	totalWarnings := 0

	for _, r := range results {
		if r.IsValid {
			valid++
		} else {
			invalid++
		}
		totalErrors += len(r.Errors)
		totalWarnings += len(r.Warnings)
	}

	avgScore := 0.0
	for _, r := range results {
		avgScore += r.Score
	}
	if total > 0 {
		avgScore /= float64(total)
	}

	return map[string]interface{}{
		"total":         total,
		"valid":         valid,
		"invalid":       invalid,
		"total_errors":  totalErrors,
		"total_warnings": totalWarnings,
		"avg_score":     avgScore,
		"pass_rate":     float64(valid) / float64(total) * 100,
	}
}
