package fraud

import (
	"math"
	"sort"
	"sync"
	"time"

	"github.com/vici-cdr-scrubber/internal/models"
)

// DetectionEngine performs fraud detection on CDR data
type DetectionEngine struct {
	mu              sync.RWMutex
	config          FraudConfig
	profiles        map[string]*CallerProfile
	anomalyRules    []AnomalyRule
	alertChan       chan FraudAlert
}

// FraudConfig holds fraud detection configuration
type FraudConfig struct {
	Enabled                bool    `yaml:"enabled"`
	HighVolumeThreshold    int     `yaml:"high_volume_threshold"`
	ShortCallThreshold     int     `yaml:"short_call_threshold"`
	ShortCallMaxCount      int     `yaml:"short_call_max_count"`
	NightCallStartHour     int     `yaml:"night_call_start_hour"`
	NightCallEndHour       int     `yaml:"night_call_end_hour"`
	GeoVelocityKmh         float64 `yaml:"geo_velocity_kmh"`
	SimultaneousCallWindow int     `yaml:"simultaneous_call_window"`
	FrequentDialerCount    int     `yaml:"frequent_dialer_count"`
	AnomalyScoreThreshold  float64 `yaml:"anomaly_score_threshold"`
}

// CallerProfile tracks calling patterns for a number
type CallerProfile struct {
	PhoneNumber     string
	TotalCalls      int
	TotalDuration   int
	UniqueTargets   map[string]bool
	CallTimes       []time.Time
	Durations       []int
	Targets         map[string]int
	HourlyDist      [24]int
	GeoLocations    []GeoPoint
	LastUpdated     time.Time
}

// GeoPoint represents a geographic location
type GeoPoint struct {
	Latitude  float64
	Longitude float64
	Timestamp time.Time
}

// FraudAlert represents a detected fraud indicator
type FraudAlert struct {
	AlertID      string    `json:"alert_id"`
	PhoneNumber  string    `json:"phone_number"`
	AlertType    string    `json:"alert_type"`
	Severity     string    `json:"severity"`
	Score        float64   `json:"score"`
	Description  string    `json:"description"`
	DetectedAt   time.Time `json:"detected_at"`
	RelatedCDRs  []string  `json:"related_cdrs,omitempty"`
}

// AnomalyRule defines a rule for anomaly detection
type AnomalyRule struct {
	Name        string
	Description string
	Weight      float64
	Evaluate    func(profile *CallerProfile, cdr models.CDR) float64
}

// NewDetectionEngine creates a new fraud detection engine
func NewDetectionEngine(config FraudConfig) *DetectionEngine {
	engine := &DetectionEngine{
		config:     config,
		profiles:   make(map[string]*CallerProfile),
		alertChan:  make(chan FraudAlert, 1000),
	}
	engine.initDefaultRules()
	return engine
}

// initDefaultRules sets up default anomaly detection rules
func (e *DetectionEngine) initDefaultRules() {
	e.anomalyRules = []AnomalyRule{
		{
			Name:        "high_volume",
			Description: "Unusually high call volume in short period",
			Weight:      0.3,
			Evaluate:    e.checkHighVolume,
		},
		{
			Name:        "short_call_burst",
			Description: "Many consecutive short-duration calls",
			Weight:      0.25,
			Evaluate:    e.checkShortCallBurst,
		},
		{
			Name:        "night_activity",
			Description: "Unusual calling pattern during night hours",
			Weight:      0.15,
			Evaluate:    e.checkNightActivity,
		},
		{
			Name:        "geo_velocity",
			Description: "Impossible travel speed between call locations",
			Weight:      0.2,
			Evaluate:    e.checkGeoVelocity,
		},
		{
			Name:        "sequential_dialing",
			Description: "Rapid sequential dialing pattern",
			Weight:      0.1,
			Evaluate:    e.checkSequentialDialing,
		},
	}
}

// AnalyzeCDR processes a single CDR for fraud indicators
func (e *DetectionEngine) AnalyzeCDR(cdr models.CDR) []FraudAlert {
	e.mu.Lock()
	defer e.mu.Unlock()

	profile := e.getOrCreateProfile(cdr.CallerCode)
	e.updateProfile(profile, cdr)

	var alerts []FraudAlert
	totalScore := 0.0

	for _, rule := range e.anomalyRules {
		score := rule.Evaluate(profile, cdr)
		if score > 0 {
			totalScore += score * rule.Weight
			if score > 0.7 {
				alert := FraudAlert{
					AlertID:     generateAlertID(),
					PhoneNumber: cdr.CallerCode,
					AlertType:   rule.Name,
					Severity:    e.calculateSeverity(score),
					Score:       score,
					Description: rule.Description,
					DetectedAt:  time.Now(),
					RelatedCDRs: []string{cdr.UniqueID},
				}
				alerts = append(alerts, alert)
			}
		}
	}

	if totalScore > e.config.AnomalyScoreThreshold {
		combinedAlert := FraudAlert{
			AlertID:     generateAlertID(),
			PhoneNumber: cdr.CallerCode,
			AlertType:   "combined_anomaly",
			Severity:    e.calculateSeverity(totalScore),
			Score:       totalScore,
			Description: "Multiple fraud indicators detected",
			DetectedAt:  time.Now(),
			RelatedCDRs: []string{cdr.UniqueID},
		}
		alerts = append(alerts, combinedAlert)
	}

	return alerts
}

// getOrCreateProfile retrieves or creates a caller profile
func (e *DetectionEngine) getOrCreateProfile(phoneNumber string) *CallerProfile {
	if profile, exists := e.profiles[phoneNumber]; exists {
		return profile
	}
	profile := &CallerProfile{
		PhoneNumber:  phoneNumber,
		UniqueTargets: make(map[string]bool),
		Targets:      make(map[string]int),
		LastUpdated:  time.Now(),
	}
	e.profiles[phoneNumber] = profile
	return profile
}

// updateProfile updates caller profile with new CDR data
func (e *DetectionEngine) updateProfile(profile *CallerProfile, cdr models.CDR) {
	profile.TotalCalls++
	profile.TotalDuration += cdr.CallDuration
	profile.UniqueTargets[cdr.CustomerPhoneNumber] = true
	profile.Targets[cdr.CustomerPhoneNumber]++
	profile.CallTimes = append(profile.CallTimes, cdr.CallDate)
	profile.Durations = append(profile.Durations, cdr.CallDuration)
	profile.LastUpdated = cdr.CallDate

	hour := cdr.CallDate.Hour()
	profile.HourlyDist[hour]++
}

// checkHighVolume detects unusually high call volume
func (e *DetectionEngine) checkHighVolume(profile *CallerProfile, cdr models.CDR) float64 {
	if len(profile.CallTimes) < 2 {
		return 0
	}

	recentCalls := 0
	cutoff := time.Now().Add(-1 * time.Hour)
	for _, t := range profile.CallTimes {
		if t.After(cutoff) {
			recentCalls++
		}
	}

	if recentCalls > e.config.HighVolumeThreshold {
		return math.Min(float64(recentCalls)/float64(e.config.HighVolumeThreshold), 1.0)
	}
	return 0
}

// checkShortCallBurst detects bursts of very short calls
func (e *DetectionEngine) checkShortCallBurst(profile *CallerProfile, cdr models.CDR) float64 {
	if len(profile.Durations) < e.config.ShortCallMaxCount {
		return 0
	}

	shortCount := 0
	for _, d := range profile.Durations[len(profile.Durations)-e.config.ShortCallMaxCount:] {
		if d <= e.config.ShortCallThreshold {
			shortCount++
		}
	}

	ratio := float64(shortCount) / float64(e.config.ShortCallMaxCount)
	if ratio > 0.7 {
		return ratio
	}
	return 0
}

// checkNightActivity detects unusual night-time calling
func (e *DetectionEngine) checkNightActivity(profile *CallerProfile, cdr models.CDR) float64 {
	hour := cdr.CallDate.Hour()
	isNight := hour >= e.config.NightCallStartHour || hour < e.config.NightCallEndHour

	if !isNight {
		return 0
	}

	totalNightCalls := 0
	for _, t := range profile.CallTimes {
		h := t.Hour()
		if h >= e.config.NightCallStartHour || h < e.config.NightCallEndHour {
			totalNightCalls++
		}
	}

	nightRatio := float64(totalNightCalls) / float64(profile.TotalCalls)
	if nightRatio > 0.3 && profile.TotalCalls > 10 {
		return nightRatio
	}
	return 0
}

// checkGeoVelocity detects impossible travel speeds
func (e *DetectionEngine) checkGeoVelocity(profile *CallerProfile, cdr models.CDR) float64 {
	if len(profile.GeoLocations) < 2 {
		return 0
	}

	last := profile.GeoLocations[len(profile.GeoLocations)-1]
	prev := profile.GeoLocations[len(profile.GeoLocations)-2]

	timeDiff := last.Timestamp.Sub(prev.Timestamp).Hours()
	if timeDiff <= 0 {
		return 0
	}

	distance := haversineDistance(prev.Latitude, prev.Longitude, last.Latitude, last.Longitude)
	velocity := distance / timeDiff

	if velocity > e.config.GeoVelocityKmh {
		return math.Min(velocity/e.config.GeoVelocityKmh, 1.0)
	}
	return 0
}

// checkSequentialDialing detects rapid sequential dialing
func (e *DetectionEngine) checkSequentialDialing(profile *CallerProfile, cdr models.CDR) float64 {
	if len(profile.CallTimes) < 3 {
		return 0
	}

	rapidCount := 0
	for i := 1; i < len(profile.CallTimes); i++ {
		diff := profile.CallTimes[i].Sub(profile.CallTimes[i-1]).Seconds()
		if diff < 10 {
			rapidCount++
		}
	}

	if rapidCount > 5 {
		return math.Min(float64(rapidCount)/10.0, 1.0)
	}
	return 0
}

// calculateSeverity determines alert severity based on score
func (e *DetectionEngine) calculateSeverity(score float64) string {
	switch {
	case score >= 0.8:
		return "critical"
	case score >= 0.6:
		return "high"
	case score >= 0.4:
		return "medium"
	default:
		return "low"
	}
}

// haversineDistance calculates distance between two geo points in km
func haversineDistance(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371
	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return R * c
}

// GetAlerts returns the alert channel
func (e *DetectionEngine) GetAlerts() <-chan FraudAlert {
	return e.alertChan
}

// GetProfiles returns all caller profiles
func (e *DetectionEngine) GetProfiles() map[string]*CallerProfile {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.profiles
}

// GetTopCallers returns callers sorted by call count
func (e *DetectionEngine) GetTopCallers(limit int) []*CallerProfile {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var profiles []*CallerProfile
	for _, p := range e.profiles {
		profiles = append(profiles, p)
	}

	sort.Slice(profiles, func(i, j int) bool {
		return profiles[i].TotalCalls > profiles[j].TotalCalls
	})

	if limit > len(profiles) {
		limit = len(profiles)
	}
	return profiles[:limit]
}

func generateAlertID() string {
	return time.Now().Format("20060102150405") + "-" + randomHex(6)
}

func randomHex(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = "0123456789abcdef"[time.Now().UnixNano()%16]
		time.Sleep(1)
	}
	return string(b)
}
