package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lioilsources/Pass/internal/catalog"
	"github.com/lioilsources/Pass/internal/notify"
	"github.com/lioilsources/Pass/internal/revenuecat"
	"github.com/lioilsources/Pass/internal/store"
)

type fakeEnt struct {
	holders     map[string]bool
	invalidated []string
	err         error
}

func (f *fakeEnt) HasEntitlement(_ context.Context, user, ent string) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	return ent == "all_apps" && f.holders[user], nil
}
func (f *fakeEnt) Invalidate(user string) { f.invalidated = append(f.invalidated, user) }

type harness struct {
	srv   *httptest.Server
	ent   *fakeEnt
	now   time.Time
	st    *store.Store
	posts *[]string
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Migrate(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })

	posts := &[]string{}
	discord := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		*posts = append(*posts, string(b))
		w.WriteHeader(204)
	}))
	t.Cleanup(discord.Close)

	cat, _ := catalog.Default()
	n := notify.New(cat)
	n.DiscordWebhookURL = discord.URL

	h := &harness{ent: &fakeEnt{holders: map[string]bool{"holder": true}}, now: time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC), st: st, posts: posts}
	s := New(Config{
		Catalog: cat, CatalogJSON: catalog.DefaultJSON(), Store: st, Entitlements: h.ent, Notifier: n,
		WebhookSecret: "whsec", Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now: func() time.Time { return h.now },
	})
	h.srv = httptest.NewServer(s)
	t.Cleanup(h.srv.Close)
	return h
}

func (h *harness) do(t *testing.T, method, path, body string, hdr ...string) (int, map[string]any) {
	t.Helper()
	req, _ := http.NewRequest(method, h.srv.URL+path, strings.NewReader(body))
	for i := 0; i+1 < len(hdr); i += 2 {
		req.Header.Set(hdr[i], hdr[i+1])
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	var out map[string]any
	if strings.HasPrefix(resp.Header.Get("Content-Type"), "application/json") {
		json.Unmarshal(b, &out)
	} else {
		out = map[string]any{"_raw": string(b)}
	}
	return resp.StatusCode, out
}

func TestHealthAndCatalog(t *testing.T) {
	h := newHarness(t)
	if code, out := h.do(t, "GET", "/healthz", ""); code != 200 || out["_raw"] != "ok" {
		t.Fatal(code, out)
	}
	code, out := h.do(t, "GET", "/pass/apps.json", "")
	if code != 200 || out["entitlement"] != "all_apps" {
		t.Fatal(code, out)
	}
	if code, out := h.do(t, "GET", "/nope", ""); code != 404 || out["error"] != "not_found" {
		t.Fatal(code, out)
	}
}

func TestEntitlements(t *testing.T) {
	h := newHarness(t)
	code, out := h.do(t, "GET", "/v1/entitlements?user=holder", "")
	if code != 200 || out["all_apps"] != true || out["source"] != "revenuecat" {
		t.Fatal(code, out)
	}
	if code, out := h.do(t, "GET", "/v1/entitlements?user=nobody", ""); code != 200 || out["all_apps"] != false {
		t.Fatal(code, out)
	}
	if code, _ := h.do(t, "GET", "/v1/entitlements", ""); code != 400 {
		t.Fatal(code)
	}
}

func TestWithoutRevenueCatKey(t *testing.T) {
	h := newHarness(t)
	_, err := revenuecat.Unconfigured{}.HasEntitlement(context.Background(), "holder", "all_apps")
	h.ent.err = err
	if code, out := h.do(t, "GET", "/v1/entitlements?user=holder", ""); code != 503 || out["error"] != "not_configured" {
		t.Fatal(code, out)
	}
	if code, out := h.do(t, "POST", "/v1/link-code", `{"app_user_id":"holder"}`); code != 503 || out["error"] != "not_configured" {
		t.Fatal(code, out)
	}
	if code, _ := h.do(t, "GET", "/pass/apps.json", ""); code != 200 {
		t.Fatal(code)
	}
}

func TestLinkCodeFlow(t *testing.T) {
	h := newHarness(t)
	// No pass → 403.
	if code, out := h.do(t, "POST", "/v1/link-code", `{"app_user_id":"nobody"}`); code != 403 || out["error"] != "no_pass" {
		t.Fatal(code, out)
	}
	if code, _ := h.do(t, "POST", "/v1/link-code", `{}`); code != 400 {
		t.Fatal(code)
	}
	if code, _ := h.do(t, "POST", "/v1/link-code", `nope`); code != 400 {
		t.Fatal(code)
	}
	// Happy path.
	code, out := h.do(t, "POST", "/v1/link-code", `{"app_user_id":"holder"}`)
	if code != 200 {
		t.Fatal(code, out)
	}
	c := out["code"].(string)
	if len(c) != 8 || out["display"] != c[:4]+"-"+c[4:] || out["expires_at"] != "2026-11-01T12:15:00Z" {
		t.Fatal(out)
	}
	// Redeem with the display form.
	code, out = h.do(t, "POST", "/v1/redeem", `{"code":"`+strings.ToLower(out["display"].(string))+`"}`)
	if code != 200 || out["app_user_id"] != "holder" {
		t.Fatal(code, out)
	}
	// Used → 404.
	if code, out := h.do(t, "POST", "/v1/redeem", `{"code":"`+c+`"}`); code != 404 || out["error"] != "invalid_code" {
		t.Fatal(code, out)
	}
	// Expired → 404.
	_, out = h.do(t, "POST", "/v1/link-code", `{"app_user_id":"holder"}`)
	h.now = h.now.Add(16 * time.Minute)
	if code, _ := h.do(t, "POST", "/v1/redeem", `{"code":"`+out["code"].(string)+`"}`); code != 404 {
		t.Fatal(code)
	}
	// Garbage → 404 too (not 400: never leak format hints beyond "invalid").
	if code, _ := h.do(t, "POST", "/v1/redeem", `{"code":"xx"}`); code != 404 {
		t.Fatal(code)
	}
}

func TestRateLimits(t *testing.T) {
	h := newHarness(t)
	var last int
	for i := 0; i < 11; i++ {
		last, _ = h.do(t, "POST", "/v1/redeem", `{"code":"ZZZZZZZZ"}`)
	}
	if last != 429 {
		t.Fatalf("redeem 11th = %d", last)
	}
	h.now = h.now.Add(time.Minute) // refill
	if code, _ := h.do(t, "POST", "/v1/redeem", `{"code":"ZZZZZZZZ"}`); code != 404 {
		t.Fatalf("after refill = %d", code)
	}
	for i := 0; i < 6; i++ {
		last, _ = h.do(t, "POST", "/v1/link-code", `{"app_user_id":"holder"}`)
	}
	if last != 429 {
		t.Fatalf("link-code 6th = %d", last)
	}
	// Another user has its own bucket.
	if code, _ := h.do(t, "POST", "/v1/link-code", `{"app_user_id":"nobody"}`); code != 403 {
		t.Fatal(code)
	}
}

const webhookBody = `{"api_version":"1.0","event":{"id":"evt_1","type":"INITIAL_PURCHASE","app_user_id":"holder","product_id":"com.ol1n.pass.lifetime.kirian","store":"APP_STORE","price":49.0,"currency":"USD","environment":"PRODUCTION","event_timestamp_ms":1761998400000}}`

func TestWebhook(t *testing.T) {
	h := newHarness(t)
	if code, out := h.do(t, "POST", "/webhooks/revenuecat", webhookBody); code != 401 || out["error"] != "unauthorized" {
		t.Fatal(code, out)
	}
	if code, _ := h.do(t, "POST", "/webhooks/revenuecat", webhookBody, "Authorization", "Bearer wrong"); code != 401 {
		t.Fatal(code)
	}
	if code, _ := h.do(t, "POST", "/webhooks/revenuecat", webhookBody, "Authorization", "wrong"); code != 401 {
		t.Fatal(code)
	}
	// RevenueCat sends the header value verbatim as configured: the bare secret must work too.
	if code, out := h.do(t, "POST", "/webhooks/revenuecat", `{"event":{"id":"bare-1","type":"TEST"}}`, "Authorization", "whsec"); code != 200 {
		t.Fatal(code, out)
	}
	code, out := h.do(t, "POST", "/webhooks/revenuecat", webhookBody, "Authorization", "Bearer whsec")
	if code != 200 || out["status"] != "ok" || out["notify"] != "notified" {
		t.Fatal(code, out)
	}
	if len(*h.posts) != 1 || !strings.Contains((*h.posts)[0], "kirian") {
		t.Fatalf("discord posts: %v", *h.posts)
	}
	if len(h.ent.invalidated) != 1 || h.ent.invalidated[0] != "holder" {
		t.Fatal(h.ent.invalidated)
	}
	// Duplicate id → no second post.
	code, out = h.do(t, "POST", "/webhooks/revenuecat", webhookBody, "Authorization", "Bearer whsec")
	if code != 200 || out["status"] != "duplicate" || len(*h.posts) != 1 {
		t.Fatal(code, out, len(*h.posts))
	}
	// TEST events are acknowledged, not stored.
	code, out = h.do(t, "POST", "/webhooks/revenuecat", `{"event":{"id":"t1","type":"TEST"}}`, "Authorization", "Bearer whsec")
	if code != 200 || out["status"] != "test" {
		t.Fatal(code, out)
	}
	if n, _ := h.st.CountEvents(); n != 1 {
		t.Fatal(n)
	}
	if code, _ := h.do(t, "POST", "/webhooks/revenuecat", `{"event":{}}`, "Authorization", "Bearer whsec"); code != 400 {
		t.Fatal(code)
	}
}

func TestClientIP(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "127.0.0.1:1234"
	r.Header.Set("X-Forwarded-For", "203.0.113.9, 10.0.0.1")
	if ip := clientIP(r); ip != "203.0.113.9" {
		t.Fatal(ip)
	}
	r.RemoteAddr = "198.51.100.7:555" // public peer: header is not trusted
	if ip := clientIP(r); ip != "198.51.100.7" {
		t.Fatal(ip)
	}
}
