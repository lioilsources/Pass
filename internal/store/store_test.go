package store

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/lioilsources/Pass/internal/linkcode"
)

func open(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "pass.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Migrate(); err != nil {
		t.Fatal(err)
	}
	if err := s.Migrate(); err != nil { // idempotent
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestLinkCodeLifecycle(t *testing.T) {
	s := open(t)
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)

	code, exp, err := linkcode.Issue(s, "user-1", now)
	if err != nil {
		t.Fatal(err)
	}
	if !exp.Equal(now.Add(linkcode.TTL)) {
		t.Fatalf("expires %v", exp)
	}
	// Unknown code.
	if _, err := linkcode.Redeem(s, "ZZZZ-ZZZZ", now); !errors.Is(err, linkcode.ErrInvalid) {
		t.Fatalf("unknown: %v", err)
	}
	// Happy path with display formatting and lower case.
	user, err := linkcode.Redeem(s, linkcode.Display(code), now.Add(time.Minute))
	if err != nil || user != "user-1" {
		t.Fatalf("redeem: %q %v", user, err)
	}
	// Second use fails.
	if _, err := linkcode.Redeem(s, code, now.Add(2*time.Minute)); !errors.Is(err, linkcode.ErrInvalid) {
		t.Fatalf("reuse: %v", err)
	}
	// Expired code fails.
	code2, _, _ := linkcode.Issue(s, "user-2", now)
	if _, err := linkcode.Redeem(s, code2, now.Add(linkcode.TTL)); !errors.Is(err, linkcode.ErrInvalid) {
		t.Fatalf("expired: %v", err)
	}
	n, err := s.PurgeExpiredLinkCodes(now.Add(time.Hour))
	if err != nil || n != 2 {
		t.Fatalf("purge: %d %v", n, err)
	}
}

func TestPurchaseEventDedup(t *testing.T) {
	s := open(t)
	e := PurchaseEvent{ID: "evt1", Type: "INITIAL_PURCHASE", AppUserID: "u", ProductID: "p", Store: "APP_STORE", ReceivedAt: time.Now(), Raw: "{}"}
	if err := s.InsertPurchaseEvent(e); err != nil {
		t.Fatal(err)
	}
	if err := s.InsertPurchaseEvent(e); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("dup: %v", err)
	}
	if err := s.MarkNotified("evt1", "notified", time.Now()); err != nil {
		t.Fatal(err)
	}
	if n, _ := s.CountEvents(); n != 1 {
		t.Fatalf("count %d", n)
	}
}

func TestTimeRoundTrip(t *testing.T) {
	in := time.Date(2026, 1, 2, 3, 4, 5, 999, time.FixedZone("x", 3600))
	s := FormatTime(in)
	if s != "2026-01-02T02:04:05Z" {
		t.Fatal(s)
	}
	out, err := ParseTime(s)
	if err != nil || !out.Equal(in.Truncate(time.Second)) {
		t.Fatal(out, err)
	}
}
