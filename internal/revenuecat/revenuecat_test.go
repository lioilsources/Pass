package revenuecat

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestHasEntitlementAndCache(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Authorization") != "Bearer sk_test" {
			w.WriteHeader(401)
			return
		}
		switch r.URL.Path {
		case "/v1/subscribers/lifetime":
			w.Write([]byte(`{"subscriber":{"entitlements":{"all_apps":{"expires_date":null,"product_identifier":"com.ol1n.pass.lifetime.kirian"}}}}`))
		case "/v1/subscribers/expired":
			w.Write([]byte(`{"subscriber":{"entitlements":{"all_apps":{"expires_date":"2020-01-01T00:00:00Z","product_identifier":"x"}}}}`))
		case "/v1/subscribers/none":
			w.Write([]byte(`{"subscriber":{"entitlements":{}}}`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	c := New("sk_test")
	c.BaseURL = srv.URL
	ctx := context.Background()

	ok, err := c.HasEntitlement(ctx, "lifetime", "all_apps")
	if err != nil || !ok {
		t.Fatalf("lifetime: %v %v", ok, err)
	}
	ok, _ = c.HasEntitlement(ctx, "lifetime", "all_apps") // cached
	if !ok || calls.Load() != 1 {
		t.Fatalf("cache miss, calls=%d", calls.Load())
	}
	c.Invalidate("lifetime")
	c.HasEntitlement(ctx, "lifetime", "all_apps")
	if calls.Load() != 2 {
		t.Fatal("invalidate did not refetch")
	}
	if ok, _ := c.HasEntitlement(ctx, "expired", "all_apps"); ok {
		t.Fatal("expired must be inactive")
	}
	if ok, _ := c.HasEntitlement(ctx, "none", "all_apps"); ok {
		t.Fatal("none must be inactive")
	}
	if _, err := c.HasEntitlement(ctx, "missing", "all_apps"); err != ErrNotFound {
		t.Fatalf("404: %v", err)
	}
}

func TestParseWebhook(t *testing.T) {
	body := []byte(`{"api_version":"1.0","event":{"id":"evt_1","type":"INITIAL_PURCHASE","app_user_id":"u1","original_app_user_id":"u1","product_id":"com.ol1n.pass.lifetime.kirian","store":"APP_STORE","price":49.0,"currency":"USD","environment":"PRODUCTION","app_id":"app1","event_timestamp_ms":1761998400000}}`)
	e, err := ParseWebhook(body)
	if err != nil {
		t.Fatal(err)
	}
	if e.ID != "evt_1" || e.Type != "INITIAL_PURCHASE" || e.Price != 49 || e.Currency != "USD" || e.Store != "APP_STORE" {
		t.Fatalf("%+v", e)
	}
	if e.EventTimestamp != time.UnixMilli(1761998400000).UTC() {
		t.Fatal(e.EventTimestamp)
	}
	tr, err := ParseWebhook([]byte(`{"event":{"id":"evt_2","type":"TRANSFER","transferred_from":["a"],"transferred_to":["b"]}}`))
	if err != nil || tr.AppUserID != "b" || tr.OriginalAppUserID != "a" {
		t.Fatalf("transfer: %+v %v", tr, err)
	}
	if _, err := ParseWebhook([]byte(`{"event":{}}`)); err == nil {
		t.Fatal("missing id accepted")
	}
}
