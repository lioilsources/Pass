// Package revenuecat talks to the RevenueCat REST API v1 for entitlement checks
// and parses the v1 webhook payload. The base URL is injectable so tests run
// against httptest.
package revenuecat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// DefaultBaseURL is the production API.
const DefaultBaseURL = "https://api.revenuecat.com"

// ErrNotFound is returned when RevenueCat does not know the app user id.
var ErrNotFound = errors.New("revenuecat: subscriber not found")

// ErrNotConfigured is returned by Unconfigured: the server runs without a
// RevenueCat key, so it can serve the catalog but never answer "has Pass".
var ErrNotConfigured = errors.New("revenuecat: no secret API key configured")

// Unconfigured stands in for Client while RC_SECRET_API_KEY is unset. Every
// entitlement check fails, so no request can ever be granted a Pass.
type Unconfigured struct{}

// HasEntitlement always fails with ErrNotConfigured.
func (Unconfigured) HasEntitlement(context.Context, string, string) (bool, error) {
	return false, ErrNotConfigured
}

// Invalidate does nothing; there is no cache.
func (Unconfigured) Invalidate(string) {}

// Client checks entitlements with the project's secret API key (v1).
type Client struct {
	BaseURL   string
	SecretKey string
	HTTP      *http.Client
	CacheTTL  time.Duration
	now       func() time.Time

	mu    sync.Mutex
	cache map[string]cacheEntry
}

type cacheEntry struct {
	active  map[string]bool
	expires time.Time
}

// New builds a client with a 10 s HTTP timeout and a 60 s cache.
func New(secretKey string) *Client {
	return &Client{
		BaseURL:   DefaultBaseURL,
		SecretKey: secretKey,
		HTTP:      &http.Client{Timeout: 10 * time.Second},
		CacheTTL:  60 * time.Second,
		now:       time.Now,
		cache:     map[string]cacheEntry{},
	}
}

// subscriber is the subset of GET /v1/subscribers/{id} we read.
type subscriber struct {
	Subscriber struct {
		Entitlements map[string]struct {
			ExpiresDate       *string `json:"expires_date"`
			ProductIdentifier string  `json:"product_identifier"`
		} `json:"entitlements"`
	} `json:"subscriber"`
}

// ActiveEntitlements returns the set of active entitlement ids for appUserID.
// Results are cached for CacheTTL. A subscriber RevenueCat has never seen is
// created by this call (RevenueCat semantics), so unknown ids yield an empty set
// rather than ErrNotFound; ErrNotFound covers a hard 404.
func (c *Client) ActiveEntitlements(ctx context.Context, appUserID string) (map[string]bool, error) {
	if appUserID == "" {
		return nil, errors.New("revenuecat: empty app user id")
	}
	c.mu.Lock()
	if e, ok := c.cache[appUserID]; ok && c.now().Before(e.expires) {
		c.mu.Unlock()
		return e.active, nil
	}
	c.mu.Unlock()

	u := strings.TrimRight(c.BaseURL, "/") + "/v1/subscribers/" + url.PathEscape(appUserID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.SecretKey)
	req.Header.Set("Accept", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("revenuecat: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return nil, ErrNotFound
	case resp.StatusCode/100 != 2:
		return nil, fmt.Errorf("revenuecat: GET subscriber: %s: %s", resp.Status, truncate(body, 300))
	}
	var s subscriber
	if err := json.Unmarshal(body, &s); err != nil {
		return nil, fmt.Errorf("revenuecat: decode: %w", err)
	}
	now := c.now()
	active := map[string]bool{}
	for id, ent := range s.Subscriber.Entitlements {
		if ent.ExpiresDate == nil {
			active[id] = true // lifetime
			continue
		}
		exp, err := time.Parse(time.RFC3339Nano, *ent.ExpiresDate)
		if err == nil && exp.After(now) {
			active[id] = true
		}
	}
	c.mu.Lock()
	c.cache[appUserID] = cacheEntry{active: active, expires: now.Add(c.CacheTTL)}
	c.mu.Unlock()
	return active, nil
}

// HasEntitlement reports whether appUserID currently holds entitlement.
func (c *Client) HasEntitlement(ctx context.Context, appUserID, entitlement string) (bool, error) {
	active, err := c.ActiveEntitlements(ctx, appUserID)
	if err != nil {
		return false, err
	}
	return active[entitlement], nil
}

// Invalidate drops a cached subscriber (called after a webhook for that user).
func (c *Client) Invalidate(appUserID string) {
	c.mu.Lock()
	delete(c.cache, appUserID)
	c.mu.Unlock()
}

// Event is the part of a RevenueCat webhook (v1) that pass-api acts on.
type Event struct {
	ID                string
	Type              string
	AppUserID         string
	OriginalAppUserID string
	ProductID         string
	Store             string
	Price             float64
	Currency          string
	Environment       string
	AppID             string
	EventTimestamp    time.Time
}

// Notified lists the event types that reach Discord/Telegram. TEST goes to the log only.
var Notified = map[string]bool{
	"INITIAL_PURCHASE":      true,
	"NON_RENEWING_PURCHASE": true,
	"RENEWAL":               true,
	"CANCELLATION":          true,
	"EXPIRATION":            true,
	"BILLING_ISSUE":         true,
	"TRANSFER":              true,
	"PRODUCT_CHANGE":        true,
}

type webhookPayload struct {
	APIVersion string `json:"api_version"`
	Event      struct {
		ID                string   `json:"id"`
		Type              string   `json:"type"`
		AppUserID         string   `json:"app_user_id"`
		OriginalAppUserID string   `json:"original_app_user_id"`
		ProductID         string   `json:"product_id"`
		Store             string   `json:"store"`
		Price             *float64 `json:"price"`
		Currency          string   `json:"currency"`
		Environment       string   `json:"environment"`
		AppID             string   `json:"app_id"`
		EventTimestampMs  int64    `json:"event_timestamp_ms"`
		// TRANSFER events carry the user ids in these arrays instead.
		TransferredFrom []string `json:"transferred_from"`
		TransferredTo   []string `json:"transferred_to"`
	} `json:"event"`
}

// ParseWebhook decodes a webhook body.
func ParseWebhook(body []byte) (Event, error) {
	var p webhookPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return Event{}, fmt.Errorf("revenuecat: webhook decode: %w", err)
	}
	e := p.Event
	if e.ID == "" || e.Type == "" {
		return Event{}, errors.New("revenuecat: webhook without event id or type")
	}
	out := Event{
		ID:                e.ID,
		Type:              e.Type,
		AppUserID:         e.AppUserID,
		OriginalAppUserID: e.OriginalAppUserID,
		ProductID:         e.ProductID,
		Store:             e.Store,
		Currency:          e.Currency,
		Environment:       e.Environment,
		AppID:             e.AppID,
	}
	if e.Price != nil {
		out.Price = *e.Price
	}
	if e.EventTimestampMs > 0 {
		out.EventTimestamp = time.UnixMilli(e.EventTimestampMs).UTC()
	}
	if out.AppUserID == "" && len(e.TransferredTo) > 0 {
		out.AppUserID = e.TransferredTo[0]
	}
	if out.OriginalAppUserID == "" && len(e.TransferredFrom) > 0 {
		out.OriginalAppUserID = e.TransferredFrom[0]
	}
	return out, nil
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "…"
}
