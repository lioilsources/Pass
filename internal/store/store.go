// Package store is the SQLite persistence of pass-api: link codes and the
// RevenueCat purchase events received over the webhook. Pure Go
// (modernc.org/sqlite), one connection, embedded migrations.
package store

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	"github.com/lioilsources/Pass/internal/linkcode"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrations embed.FS

// TimeLayout is how every timestamp is stored: UTC, whole seconds, so string
// comparison in SQL is chronological.
const TimeLayout = "2006-01-02T15:04:05Z"

// FormatTime renders t for storage.
func FormatTime(t time.Time) string { return t.UTC().Truncate(time.Second).Format(TimeLayout) }

// ParseTime reads a stored timestamp.
func ParseTime(s string) (time.Time, error) { return time.Parse(TimeLayout, s) }

// Store wraps the database.
type Store struct {
	db *sql.DB
}

// Open opens (and creates) the SQLite file at path. Use ":memory:" for tests.
func Open(path string) (*Store, error) {
	dsn := path
	if path != ":memory:" {
		dsn = "file:" + path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open: %w", err)
	}
	// A single connection: SQLite writers serialize anyway and in-memory
	// databases are per connection.
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: ping: %w", err)
	}
	return &Store{db: db}, nil
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

// Migrate applies every migrations/NNN_*.sql not yet recorded in
// schema_migrations, in file-name order.
func (s *Store) Migrate() error {
	if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (name TEXT PRIMARY KEY, applied_at TEXT NOT NULL)`); err != nil {
		return fmt.Errorf("store: migrate: %w", err)
	}
	names, err := fs.Glob(migrations, "migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(names)
	for _, name := range names {
		var n int
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE name = ?`, name).Scan(&n); err != nil {
			return fmt.Errorf("store: migrate: %w", err)
		}
		if n > 0 {
			continue
		}
		body, err := migrations.ReadFile(name)
		if err != nil {
			return err
		}
		tx, err := s.db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(string(body)); err != nil {
			tx.Rollback()
			return fmt.Errorf("store: migrate %s: %w", name, err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations (name, applied_at) VALUES (?, ?)`, name, FormatTime(time.Now())); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

// InsertLinkCode implements linkcode.Store.
func (s *Store) InsertLinkCode(hash, appUserID string, createdAt, expiresAt time.Time) error {
	_, err := s.db.Exec(`INSERT INTO link_codes (hash, app_user_id, created_at, expires_at) VALUES (?, ?, ?, ?)`,
		hash, appUserID, FormatTime(createdAt), FormatTime(expiresAt))
	if err != nil {
		return fmt.Errorf("store: insert link code: %w", err)
	}
	return nil
}

// RedeemLinkCode implements linkcode.Store: one UPDATE that only hits a live,
// unredeemed row, so two concurrent redeems cannot both succeed.
func (s *Store) RedeemLinkCode(hash string, now time.Time) (string, error) {
	nowS := FormatTime(now)
	res, err := s.db.Exec(`UPDATE link_codes SET redeemed_at = ? WHERE hash = ? AND redeemed_at IS NULL AND expires_at > ?`,
		nowS, hash, nowS)
	if err != nil {
		return "", fmt.Errorf("store: redeem: %w", err)
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return "", linkcode.ErrInvalid
	}
	var user string
	if err := s.db.QueryRow(`SELECT app_user_id FROM link_codes WHERE hash = ?`, hash).Scan(&user); err != nil {
		return "", fmt.Errorf("store: redeem: %w", err)
	}
	return user, nil
}

// PurgeExpiredLinkCodes deletes codes that expired before cutoff.
func (s *Store) PurgeExpiredLinkCodes(cutoff time.Time) (int64, error) {
	res, err := s.db.Exec(`DELETE FROM link_codes WHERE expires_at < ?`, FormatTime(cutoff))
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// PurchaseEvent is one RevenueCat webhook event.
type PurchaseEvent struct {
	ID          string
	Type        string
	AppUserID   string
	ProductID   string
	Store       string
	Price       float64
	Currency    string
	Environment string
	ReceivedAt  time.Time
	Raw         string
}

// ErrDuplicate is returned by InsertPurchaseEvent for an event id already seen.
var ErrDuplicate = errors.New("store: duplicate event")

// InsertPurchaseEvent stores an event; the webhook is deduplicated on event id.
func (s *Store) InsertPurchaseEvent(e PurchaseEvent) error {
	_, err := s.db.Exec(`INSERT INTO purchase_events (id, type, app_user_id, product_id, store, price, currency, environment, received_at, raw)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.Type, e.AppUserID, e.ProductID, e.Store, e.Price, e.Currency, e.Environment, FormatTime(e.ReceivedAt), e.Raw)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") || strings.Contains(err.Error(), "constraint failed") {
			return ErrDuplicate
		}
		return fmt.Errorf("store: insert event: %w", err)
	}
	return nil
}

// MarkNotified records the outcome of the notification for an event:
// status is "notified" or "failed".
// TODO: a retry loop for status=failed (e.g. a 5-minute ticker re-sending
// events with notified_at IS NULL and status='failed') is deliberately left
// for a later version; today a failed Discord post is logged and visible in
// the table.
func (s *Store) MarkNotified(id, status string, at time.Time) error {
	_, err := s.db.Exec(`UPDATE purchase_events SET notify_status = ?, notified_at = ? WHERE id = ?`, status, FormatTime(at), id)
	return err
}

// CountEvents returns the number of stored events (used by tests and `/healthz`-style checks).
func (s *Store) CountEvents() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM purchase_events`).Scan(&n)
	return n, err
}
