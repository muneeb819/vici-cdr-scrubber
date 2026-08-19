package enrichment

import (
	"fmt"
	"strings"
	"sync"

	"github.com/vici-cdr-scrubber/pkg/models"
)

// Enricher enriches CDR data with additional information
type Enricher struct {
	mu            sync.RWMutex
	carrierDB     map[string]*CarrierInfo
	timezoneDB    map[string]string
	prefixDB      map[string]*PhonePrefix
	rateCenterDB  map[string]*RateCenter
}

// CarrierInfo holds carrier identification data
type CarrierInfo struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Country   string `json:"country"`
	MCC       string `json:"mcc"`
	MNC       string `json:"mnc"`
	LineType  string `json:"line_type"`
}

// PhonePrefix holds phone number prefix data
type PhonePrefix struct {
	Country     string `json:"country"`
	CountryCode string `json:"country_code"`
	Prefix      string `json:"prefix"`
	Region      string `json:"region"`
	Carrier     string `json:"carrier"`
}

// RateCenter holds rate center information
type RateCenter struct {
	Name      string  `json:"name"`
	State     string  `json:"state"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Timezone  string  `json:"timezone"`
}

// EnrichedCDR holds enriched CDR data
type EnrichedCDR struct {
 models.ScrubbedCDR
	CarrierName     string  `json:"carrier_name"`
	CarrierType     string  `json:"carrier_type"`
	Country         string  `json:"country"`
	CountryCode     string  `json:"country_code"`
	Region          string  `json:"region"`
	Timezone        string  `json:"timezone"`
	LineType        string  `json:"line_type"`
	RateCenter      string  `json:"rate_center"`
	IsInternational bool    `json:"is_international"`
	IsTollFree      bool    `json:"is_toll_free"`
	IsMobile        bool    `json:"is_mobile"`
}

// EnrichmentResult holds enrichment results
type EnrichmentResult struct {
	EnrichedCDRs   []EnrichedCDR `json:"enriched_cdrs"`
	TotalProcessed int           `json:"total_processed"`
	TotalEnriched  int           `json:"total_enriched"`
	EnrichmentRate float64       `json:"enrichment_rate"`
}

// NewEnricher creates a new data enricher
func NewEnricher() *Enricher {
	e := &Enricher{
		carrierDB:    make(map[string]*CarrierInfo),
		timezoneDB:   make(map[string]string),
		prefixDB:     make(map[string]*PhonePrefix),
		rateCenterDB: make(map[string]*RateCenter),
	}
	e.loadDefaultData()
	return e
}

// loadDefaultData loads default enrichment data
func (e *Enricher) loadDefaultData() {
	e.prefixDB["1"] = &PhonePrefix{Country: "United States", CountryCode: "1", Region: "North America"}
	e.prefixDB["44"] = &PhonePrefix{Country: "United Kingdom", CountryCode: "44", Region: "Europe"}
	e.prefixDB["91"] = &PhonePrefix{Country: "India", CountryCode: "91", Region: "Asia"}
	e.prefixDB["86"] = &PhonePrefix{Country: "China", CountryCode: "86", Region: "Asia"}
	e.prefixDB["81"] = &PhonePrefix{Country: "Japan", CountryCode: "81", Region: "Asia"}
	e.prefixDB["49"] = &PhonePrefix{Country: "Germany", CountryCode: "49", Region: "Europe"}
	e.prefixDB["33"] = &PhonePrefix{Country: "France", CountryCode: "33", Region: "Europe"}
	e.prefixDB["61"] = &PhonePrefix{Country: "Australia", CountryCode: "61", Region: "Oceania"}
	e.prefixDB["55"] = &PhonePrefix{Country: "Brazil", CountryCode: "55", Region: "South America"}
	e.prefixDB["52"] = &PhonePrefix{Country: "Mexico", CountryCode: "52", Region: "North America"}

	e.timezoneDB["1"] = "America/New_York"
	e.timezoneDB["44"] = "Europe/London"
	e.timezoneDB["91"] = "Asia/Kolkata"
	e.timezoneDB["86"] = "Asia/Shanghai"
	e.timezoneDB["81"] = "Asia/Tokyo"
	e.timezoneDB["49"] = "Europe/Berlin"
	e.timezoneDB["33"] = "Europe/Paris"
	e.timezoneDB["61"] = "Australia/Sydney"
	e.timezoneDB["55"] = "America/Sao_Paulo"
	e.timezoneDB["52"] = "America/Mexico_City"

	e.carrierDB["TMOBILE"] = &CarrierInfo{Name: "T-Mobile", Type: "MNO", Country: "US", LineType: "mobile"}
	e.carrierDB["ATT"] = &CarrierInfo{Name: "AT&T", Type: "MNO", Country: "US", LineType: "mobile"}
	e.carrierDB["VERIZON"] = &CarrierInfo{Name: "Verizon", Type: "MNO", Country: "US", LineType: "mobile"}
}

// EnrichCDR enriches a single CDR record
func (e *Enricher) EnrichCDR(cdr models.ScrubbedCDR) EnrichedCDR {
	e.mu.RLock()
	defer e.mu.RUnlock()

	enriched := EnrichedCDR{
		ScrubbedCDR: cdr,
	}

	phone := strings.TrimPrefix(cdr.NormalizedPhone, "+")

	// Determine country from prefix
	for prefix, info := range e.prefixDB {
		if strings.HasPrefix(phone, prefix) {
			enriched.Country = info.Country
			enriched.CountryCode = info.CountryCode
			enriched.Region = info.Region
			break
		}
	}

	// Determine timezone
	if enriched.CountryCode != "" {
		if tz, exists := e.timezoneDB[enriched.CountryCode]; exists {
			enriched.Timezone = tz
		}
	}

	// Determine line type
	enriched.LineType = e.determineLineType(phone)
	enriched.IsMobile = enriched.LineType == "mobile"
	enriched.IsTollFree = e.isTollFree(phone)
	enriched.IsInternational = enriched.CountryCode != "1" && enriched.CountryCode != ""

	// Look up carrier
	if carrier, exists := e.carrierDB[strings.ToUpper(cdr.User)]; exists {
		enriched.CarrierName = carrier.Name
		enriched.CarrierType = carrier.Type
	}

	return enriched
}

// EnrichBatch enriches a batch of CDR records
func (e *Enricher) EnrichBatch(cdrs []models.ScrubbedCDR) *EnrichmentResult {
	result := &EnrichmentResult{
		TotalProcessed: len(cdrs),
	}

	for _, cdr := range cdrs {
		enriched := e.EnrichCDR(cdr)
		result.EnrichedCDRs = append(result.EnrichedCDRs, enriched)
		if enriched.Country != "" || enriched.CarrierName != "" {
			result.TotalEnriched++
		}
	}

	if result.TotalProcessed > 0 {
		result.EnrichmentRate = float64(result.TotalEnriched) / float64(result.TotalProcessed) * 100
	}

	return result
}

// determineLineType determines the line type from phone number
func (e *Enricher) determineLineType(phone string) string {
	if strings.HasPrefix(phone, "800") || strings.HasPrefix(phone, "888") ||
		strings.HasPrefix(phone, "877") || strings.HasPrefix(phone, "866") {
		return "toll_free"
	}

	if len(phone) == 10 && !strings.HasPrefix(phone, "8") {
		return "landline"
	}

	if len(phone) >= 10 {
		return "mobile"
	}

	return "unknown"
}

// isTollFree checks if a phone number is toll-free
func (e *Enricher) isTollFree(phone string) bool {
	tollFreePrefixes := []string{"800", "888", "877", "866", "855", "844", "833"}
	for _, prefix := range tollFreePrefixes {
		if strings.HasPrefix(phone, prefix) {
			return true
		}
	}
	return false
}

// AddCarrier adds carrier information to the database
func (e *Enricher) AddCarrier(code string, carrier CarrierInfo) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.carrierDB[code] = &carrier
}

// AddPrefix adds phone prefix information to the database
func (e *Enricher) AddPrefix(prefix string, info PhonePrefix) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.prefixDB[prefix] = &info
}

// AddTimezone adds timezone mapping
func (e *Enricher) AddTimezone(countryCode, timezone string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.timezoneDB[countryCode] = timezone
}

// GetCarrierInfo returns carrier information
func (e *Enricher) GetCarrierInfo(code string) *CarrierInfo {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.carrierDB[code]
}

// GetPrefixInfo returns prefix information
func (e *Enricher) GetPrefixInfo(prefix string) *PhonePrefix {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.prefixDB[prefix]
}

// FormatEnrichedCDR formats enriched CDR for display
func FormatEnrichedCDR(cdr EnrichedCDR) string {
	return fmt.Sprintf("Phone: %s | Country: %s | Carrier: %s | Type: %s | TZ: %s",
		cdr.NormalizedPhone, cdr.Country, cdr.CarrierName, cdr.LineType, cdr.Timezone)
}
