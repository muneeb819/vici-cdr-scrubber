package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"

	"github.com/vici-cdr-scrubber/pkg/models"
)

// PostgresDB wraps the database connection
type PostgresDB struct {
	conn *sql.DB
}

// NewPostgresDB creates a new database connection
func NewPostgresDB(cfg models.DatabaseConfig) (*PostgresDB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode,
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &PostgresDB{conn: db}, nil
}

// Close closes the database connection
func (d *PostgresDB) Close() error {
	return d.conn.Close()
}

// FetchCDRs retrieves CDR records in batches
func (d *PostgresDB) FetchCDRs(ctx context.Context, offset, limit int) ([]models.CDR, error) {
	query := `
		SELECT uniqueid, call_date, caller_code, customer_phone_number,
		       phone_code, lead_id, campaign_id, list_id, closecallid,
		       park_time, call_duration, status, user, station,
		       called_count, last_local_call_time
		FROM vicidial_closer_log
		ORDER BY call_date DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := d.conn.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query CDRs: %w", err)
	}
	defer rows.Close()

	var cdrs []models.CDR
	for rows.Next() {
		var cdr models.CDR
		err := rows.Scan(
			&cdr.UniqueID, &cdr.CallDate, &cdr.CallerCode, &cdr.CustomerPhoneNumber,
			&cdr.PhoneCode, &cdr.LeadID, &cdr.CampaignID, &cdr.ListID, &cdr.CloseCallID,
			&cdr.ParkTime, &cdr.CallDuration, &cdr.Status, &cdr.User, &cdr.Station,
			&cdr.CalledCount, &cdr.LastLocalCallTime,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan CDR: %w", err)
		}
		cdrs = append(cdrs, cdr)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return cdrs, nil
}

// GetTotalCount returns the total number of CDR records
func (d *PostgresDB) GetTotalCount(ctx context.Context) (int64, error) {
	query := "SELECT COUNT(*) FROM vicidial_closer_log"

	var count int64
	err := d.conn.QueryRowContext(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get count: %w", err)
	}

	return count, nil
}

// InsertScrubbedCDRs inserts scrubbed CDR records
func (d *PostgresDB) InsertScrubbedCDRs(ctx context.Context, cdrs []models.ScrubbedCDR) error {
	tx, err := d.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	query := `
		INSERT INTO scrubbed_cdr (
			uniqueid, call_date, caller_code, customer_phone_number,
			phone_code, lead_id, campaign_id, list_id, closecallid,
			park_time, call_duration, status, user, station,
			called_count, last_local_call_time, scrubbed_at, scrub_reason,
			is_valid, normalized_phone, carrier_name, call_type
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
			$11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22
		)
		ON CONFLICT (uniqueid) DO UPDATE SET
			scrubbed_at = EXCLUDED.scrubbed_at,
			scrub_reason = EXCLUDED.scrub_reason,
			is_valid = EXCLUDED.is_valid,
			normalized_phone = EXCLUDED.normalized_phone,
			carrier_name = EXCLUDED.carrier_name,
			call_type = EXCLUDED.call_type
	`

	for _, cdr := range cdrs {
		_, err := tx.ExecContext(ctx, query,
			cdr.UniqueID, cdr.CallDate, cdr.CallerCode, cdr.CustomerPhoneNumber,
			cdr.PhoneCode, cdr.LeadID, cdr.CampaignID, cdr.ListID, cdr.CloseCallID,
			cdr.ParkTime, cdr.CallDuration, cdr.Status, cdr.User, cdr.Station,
			cdr.CalledCount, cdr.LastLocalCallTime, cdr.ScrubbedAt, cdr.ScrubReason,
			cdr.IsValid, cdr.NormalizedPhone, cdr.CarrierName, cdr.CallType,
		)
		if err != nil {
			return fmt.Errorf("failed to insert CDR: %w", err)
		}
	}

	return tx.Commit()
}
