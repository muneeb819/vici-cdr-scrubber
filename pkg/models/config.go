package models

// Config holds the application configuration
type Config struct {
	Database   DatabaseConfig   `yaml:"database"`
	Scrubber   ScrubberConfig   `yaml:"scrubber"`
	Fraud      FraudConfig      `yaml:"fraud"`
	Validation ValidationConfig `yaml:"validation"`
	Enrichment EnrichmentConfig `yaml:"enrichment"`
	Output     OutputConfig     `yaml:"output"`
	Logging    LoggingConfig    `yaml:"logging"`
	Scheduler  SchedulerConfig  `yaml:"scheduler"`
}

// FraudConfig holds fraud detection settings
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

// ValidationConfig holds validation settings
type ValidationConfig struct {
	StrictMode     bool     `yaml:"strict_mode"`
	PhoneRegex     string   `yaml:"phone_regex"`
	MaxDuration    int      `yaml:"max_duration"`
	MinDuration    int      `yaml:"min_duration"`
	ValidStatuses  []string `yaml:"valid_statuses"`
	ValidCampaigns []string `yaml:"valid_campaigns"`
}

// EnrichmentConfig holds enrichment settings
type EnrichmentConfig struct {
	Enabled        bool   `yaml:"enabled"`
	CarrierLookup  bool   `yaml:"carrier_lookup"`
	TimezoneLookup bool   `yaml:"timezone_lookup"`
	GeoLookup      bool   `yaml:"geo_lookup"`
}

// SchedulerConfig holds scheduler settings
type SchedulerConfig struct {
	Enabled     bool   `yaml:"enabled"`
	Interval    int    `yaml:"interval_seconds"`
	MaxRetries  int    `yaml:"max_retries"`
}

// DatabaseConfig holds database connection settings
type DatabaseConfig struct {
	Host           string `yaml:"host"`
	Port           int    `yaml:"port"`
	User           string `yaml:"user"`
	Password       string `yaml:"password"`
	DBName         string `yaml:"dbname"`
	SSLMode        string `yaml:"sslmode"`
	MaxOpenConns   int    `yaml:"max_open_conns"`
	MaxIdleConns   int    `yaml:"max_idle_conns"`
	ConnMaxLifetime int   `yaml:"conn_max_lifetime"`
}

// ScrubberConfig holds scrubbing behavior settings
type ScrubberConfig struct {
	BatchSize        int      `yaml:"batch_size"`
	MaxRetries       int      `yaml:"max_retries"`
	Deduplication    bool     `yaml:"deduplication"`
	NormalizePhones  bool     `yaml:"normalize_phones"`
	RemoveInternal   bool     `yaml:"remove_internal"`
	MinDuration      int      `yaml:"min_duration"`
	MaxDuration      int      `yaml:"max_duration"`
	ExcludeStatuses  []string `yaml:"exclude_statuses"`
	IncludeCampaigns []string `yaml:"include_campaigns"`
	ExcludeCampaigns []string `yaml:"exclude_campaigns"`
}

// OutputConfig holds output settings
type OutputConfig struct {
	Format         string `yaml:"format"`
	Directory      string `yaml:"directory"`
	FilenamePrefix string `yaml:"filename_prefix"`
	IncludeHeaders bool   `yaml:"include_headers"`
}

// LoggingConfig holds logging settings
type LoggingConfig struct {
	Level        string `yaml:"level"`
	File         string `yaml:"file"`
	MaxSize      int    `yaml:"max_size"`
	MaxBackups   int    `yaml:"max_backups"`
	Compress     bool   `yaml:"compress"`
}
