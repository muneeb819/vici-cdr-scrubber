package models

import (
	"time"
)

// CDR represents a Call Detail Record from Vicidial
type CDR struct {
	UniqueID           string     `json:"unique_id" db:"uniqueid"`
	CallDate           time.Time  `json:"call_date" db:"call_date"`
	CallerCode         string     `json:"caller_code" db:"caller_code"`
	CustomerPhoneNumber string   `json:"customer_phone_number" db:"customer_phone_number"`
	PhoneCode          string     `json:"phone_code" db:"phone_code"`
	LeadID             int64      `json:"lead_id" db:"lead_id"`
	CampaignID         string     `json:"campaign_id" db:"campaign_id"`
	ListID             int64      `json:"list_id" db:"list_id"`
	CloseCallID        string     `json:"close_call_id" db:"closecallid"`
	ParkTime           int        `json:"park_time" db:"park_time"`
	CallDuration       int        `json:"call_duration" db:"call_duration"`
	Status             string     `json:"status" db:"status"`
	User               string     `json:"user" db:"user"`
	Station            string     `json:"station" db:"station"`
	CalledCount        int        `json:"called_count" db:"called_count"`
	LastLocalCallTime  time.Time  `json:"last_local_call_time" db:"last_local_call_time"`
	CustomMetadata     string     `json:"custom_metadata" db:"custom_metadata"`
}

// ScrubbedCDR represents a CDR after scrubbing/processing
type ScrubbedCDR struct {
	CDR
	ScrubbedAt        time.Time  `json:"scrubbed_at"`
	ScrubReason       string     `json:"scrub_reason,omitempty"`
	IsValid           bool       `json:"is_valid"`
	NormalizedPhone   string     `json:"normalized_phone,omitempty"`
	CarrierName       string     `json:"carrier_name,omitempty"`
	CallType          string     `json:"call_type,omitempty"`
	FraudScore        float64    `json:"fraud_score,omitempty"`
	FraudAlerts       []string   `json:"fraud_alerts,omitempty"`
	EnrichedCountry   string     `json:"enriched_country,omitempty"`
	EnrichedTimezone  string     `json:"enriched_timezone,omitempty"`
	EnrichedLineType  string     `json:"enriched_line_type,omitempty"`
	ValidationScore   float64    `json:"validation_score,omitempty"`
	IsInternational   bool       `json:"is_international"`
	IsTollFree        bool       `json:"is_toll_free"`
	IsMobile          bool       `json:"is_mobile"`
}

// ScrubStats holds statistics about the scrubbing process
type ScrubStats struct {
	TotalRecords      int64      `json:"total_records"`
	ProcessedRecords  int64      `json:"processed_records"`
	ValidRecords      int64      `json:"valid_records"`
	InvalidRecords    int64      `json:"invalid_records"`
	DuplicateRecords  int64      `json:"duplicate_records"`
	FilteredRecords   int64      `json:"filtered_records"`
	FraudDetected     int        `json:"fraud_detected"`
	EnrichedRecords   int        `json:"enriched_records"`
	ValidationErrors  int        `json:"validation_errors"`
	QualityScore      float64    `json:"quality_score"`
	StartTime         time.Time  `json:"start_time"`
	EndTime           time.Time  `json:"end_time"`
	Duration          float64    `json:"duration_seconds"`
	Errors            []string   `json:"errors,omitempty"`
}

// FilterCriteria defines filtering rules for CDR scrubbing
type FilterCriteria struct {
	MinDuration       int        `json:"min_duration"`
	MaxDuration       int        `json:"max_duration"`
	ExcludeStatuses   []string   `json:"exclude_statuses"`
	IncludeCampaigns  []string   `json:"include_campaigns"`
	ExcludeCampaigns  []string   `json:"exclude_campaigns"`
	StartDate         *time.Time `json:"start_date,omitempty"`
	EndDate           *time.Time `json:"end_date,omitempty"`
	RemoveInternal    bool       `json:"remove_internal"`
	RemoveDuplicates  bool       `json:"remove_duplicates"`
}
