package asc

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/lioilsources/Pass/internal/catalog"
)

func testKeyPEM(t *testing.T) ([]byte, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}), key
}

func TestTokenClaims(t *testing.T) {
	pemBytes, key := testKeyPEM(t)
	c, err := New("KEYID123", "issuer-uuid", pemBytes)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	c.now = func() time.Time { return now }
	tok, err := c.Token()
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := jwt.ParseWithClaims(tok, &jwt.RegisteredClaims{}, func(t *jwt.Token) (any, error) {
		return &key.PublicKey, nil
	}, jwt.WithTimeFunc(func() time.Time { return now }))
	if err != nil {
		t.Fatal(err)
	}
	claims := parsed.Claims.(*jwt.RegisteredClaims)
	if claims.Issuer != "issuer-uuid" || claims.Audience[0] != "appstoreconnect-v1" {
		t.Fatalf("%+v", claims)
	}
	if exp := claims.ExpiresAt.Time.Sub(now); exp <= 0 || exp > 20*time.Minute {
		t.Fatalf("exp %v", exp)
	}
	if parsed.Header["kid"] != "KEYID123" || parsed.Header["alg"] != "ES256" {
		t.Fatalf("%v", parsed.Header)
	}
}

func TestPlanDeterministic(t *testing.T) {
	cat, _ := catalog.Default()
	app := *cat.Find("kirian")
	var a, b bytes.Buffer
	if err := Plan(app, cat.Pass, &a); err != nil {
		t.Fatal(err)
	}
	if err := Plan(app, cat.Pass, &b); err != nil {
		t.Fatal(err)
	}
	if a.String() != b.String() {
		t.Fatal("plan output differs between runs")
	}
	out := a.String()
	for _, want := range []string{
		"filter[bundleId]=com.ol1n.kiran",
		"POST /v2/inAppPurchases {NON_CONSUMABLE, productId com.ol1n.pass.lifetime.kirian",
		"customerPrice 49.00",
		"POST /v1/subscriptionGroups {referenceName ol1n_pass}",
		"productId com.ol1n.pass.yearly.kirian",
		"customerPrice 14.99",
		"POST /v1/subscriptionAvailabilities",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("plan missing %q\n%s", want, out)
		}
	}
	if n := strings.Count(out, "\n"); n != 25 {
		t.Errorf("expected 25 lines (header + 24 steps), got %d:\n%s", n, out)
	}
}

// A fake ASC that remembers what was created, so Provision can be exercised
// twice: the second run must only GET.
func TestLiveProvisionIdempotent(t *testing.T) {
	pemBytes, _ := testKeyPEM(t)
	var posts []string
	iaps := []Resource{}
	groups := []Resource{}
	subs := []Resource{}
	locs := map[string]bool{}
	sched := map[string]bool{}
	mux := http.NewServeMux()
	list := func(w http.ResponseWriter, rs []Resource) {
		json.NewEncoder(w).Encode(map[string]any{"data": rs})
	}
	mux.HandleFunc("GET /v1/apps", func(w http.ResponseWriter, r *http.Request) {
		list(w, []Resource{{Type: "apps", ID: "APP1"}})
	})
	mux.HandleFunc("GET /v1/territories", func(w http.ResponseWriter, r *http.Request) {
		list(w, []Resource{{Type: "territories", ID: "USA"}, {Type: "territories", ID: "CZE"}})
	})
	mux.HandleFunc("GET /v1/apps/APP1/inAppPurchasesV2", func(w http.ResponseWriter, r *http.Request) { list(w, iaps) })
	mux.HandleFunc("POST /v2/inAppPurchases", func(w http.ResponseWriter, r *http.Request) {
		var body Document
		json.NewDecoder(r.Body).Decode(&body)
		res, _ := body.One()
		res.ID = "IAP1"
		iaps = append(iaps, res)
		posts = append(posts, r.URL.Path)
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(map[string]any{"data": res})
	})
	mux.HandleFunc("GET /v2/inAppPurchases/IAP1/inAppPurchaseLocalizations", func(w http.ResponseWriter, r *http.Request) {
		if locs["iap"] {
			list(w, []Resource{{Type: "inAppPurchaseLocalizations", Attributes: map[string]any{"locale": "en-US"}}})
			return
		}
		list(w, nil)
	})
	mux.HandleFunc("POST /v1/inAppPurchaseLocalizations", func(w http.ResponseWriter, r *http.Request) {
		locs["iap"] = true
		posts = append(posts, r.URL.Path)
		w.WriteHeader(201)
		w.Write([]byte(`{"data":{"type":"inAppPurchaseLocalizations","id":"L1"}}`))
	})
	mux.HandleFunc("GET /v2/inAppPurchases/IAP1/iapPriceSchedule", func(w http.ResponseWriter, r *http.Request) {
		if sched["iap"] {
			w.Write([]byte(`{"data":{"type":"inAppPurchasePriceSchedules","id":"S1"}}`))
			return
		}
		w.WriteHeader(404)
		w.Write([]byte(`{"errors":[{"status":"404","code":"NOT_FOUND","title":"not found","detail":"no schedule"}]}`))
	})
	mux.HandleFunc("GET /v2/inAppPurchases/IAP1/pricePoints", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("filter[territory]") != "USA" {
			t.Errorf("territory filter missing")
		}
		list(w, []Resource{
			{Type: "inAppPurchasePricePoints", ID: "PP0", Attributes: map[string]any{"customerPrice": "0.99"}},
			{Type: "inAppPurchasePricePoints", ID: "PP49", Attributes: map[string]any{"customerPrice": "49.0"}},
		})
	})
	mux.HandleFunc("POST /v1/inAppPurchasePriceSchedules", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		inc := body["included"].([]any)[0].(map[string]any)
		pp := inc["relationships"].(map[string]any)["inAppPurchasePricePoint"].(map[string]any)["data"].(map[string]any)["id"]
		if pp != "PP49" {
			t.Errorf("price point %v", pp)
		}
		sched["iap"] = true
		posts = append(posts, r.URL.Path)
		w.WriteHeader(201)
		w.Write([]byte(`{"data":{"type":"inAppPurchasePriceSchedules","id":"S1"}}`))
	})
	mux.HandleFunc("GET /v2/inAppPurchases/IAP1/inAppPurchaseAvailability", func(w http.ResponseWriter, r *http.Request) {
		if sched["iapAvail"] {
			w.Write([]byte(`{"data":{"type":"inAppPurchaseAvailabilities","id":"A1"}}`))
			return
		}
		w.WriteHeader(404)
	})
	mux.HandleFunc("POST /v1/inAppPurchaseAvailabilities", func(w http.ResponseWriter, r *http.Request) {
		sched["iapAvail"] = true
		posts = append(posts, r.URL.Path)
		w.WriteHeader(201)
		w.Write([]byte(`{"data":{"type":"inAppPurchaseAvailabilities","id":"A1"}}`))
	})
	mux.HandleFunc("GET /v1/apps/APP1/subscriptionGroups", func(w http.ResponseWriter, r *http.Request) { list(w, groups) })
	mux.HandleFunc("POST /v1/subscriptionGroups", func(w http.ResponseWriter, r *http.Request) {
		var body Document
		json.NewDecoder(r.Body).Decode(&body)
		res, _ := body.One()
		res.ID = "G1"
		groups = append(groups, res)
		posts = append(posts, r.URL.Path)
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(map[string]any{"data": res})
	})
	mux.HandleFunc("GET /v1/subscriptionGroups/G1/subscriptionGroupLocalizations", func(w http.ResponseWriter, r *http.Request) {
		if locs["group"] {
			list(w, []Resource{{Attributes: map[string]any{"locale": "en-US"}}})
			return
		}
		list(w, nil)
	})
	mux.HandleFunc("POST /v1/subscriptionGroupLocalizations", func(w http.ResponseWriter, r *http.Request) {
		locs["group"] = true
		posts = append(posts, r.URL.Path)
		w.WriteHeader(201)
		w.Write([]byte(`{"data":{"type":"subscriptionGroupLocalizations","id":"GL1"}}`))
	})
	mux.HandleFunc("GET /v1/subscriptionGroups/G1/subscriptions", func(w http.ResponseWriter, r *http.Request) { list(w, subs) })
	mux.HandleFunc("POST /v1/subscriptions", func(w http.ResponseWriter, r *http.Request) {
		var body Document
		json.NewDecoder(r.Body).Decode(&body)
		res, _ := body.One()
		if res.Str("subscriptionPeriod") != "ONE_YEAR" {
			t.Errorf("period %q", res.Str("subscriptionPeriod"))
		}
		res.ID = "SUB1"
		subs = append(subs, res)
		posts = append(posts, r.URL.Path)
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(map[string]any{"data": res})
	})
	mux.HandleFunc("GET /v1/subscriptions/SUB1/subscriptionLocalizations", func(w http.ResponseWriter, r *http.Request) {
		if locs["sub"] {
			list(w, []Resource{{Attributes: map[string]any{"locale": "en-US"}}})
			return
		}
		list(w, nil)
	})
	mux.HandleFunc("POST /v1/subscriptionLocalizations", func(w http.ResponseWriter, r *http.Request) {
		locs["sub"] = true
		posts = append(posts, r.URL.Path)
		w.WriteHeader(201)
		w.Write([]byte(`{"data":{"type":"subscriptionLocalizations","id":"SL1"}}`))
	})
	mux.HandleFunc("GET /v1/subscriptions/SUB1/prices", func(w http.ResponseWriter, r *http.Request) {
		if sched["sub"] {
			list(w, []Resource{{Type: "subscriptionPrices", ID: "SP1"}})
			return
		}
		list(w, nil)
	})
	mux.HandleFunc("GET /v1/subscriptions/SUB1/pricePoints", func(w http.ResponseWriter, r *http.Request) {
		list(w, []Resource{{Type: "subscriptionPricePoints", ID: "SPP1499", Attributes: map[string]any{"customerPrice": "14.99"}}})
	})
	mux.HandleFunc("POST /v1/subscriptionPrices", func(w http.ResponseWriter, r *http.Request) {
		sched["sub"] = true
		posts = append(posts, r.URL.Path)
		w.WriteHeader(201)
		w.Write([]byte(`{"data":{"type":"subscriptionPrices","id":"SP1"}}`))
	})
	mux.HandleFunc("GET /v1/subscriptions/SUB1/subscriptionAvailability", func(w http.ResponseWriter, r *http.Request) {
		if sched["subAvail"] {
			w.Write([]byte(`{"data":{"type":"subscriptionAvailabilities","id":"SA1"}}`))
			return
		}
		w.WriteHeader(404)
	})
	mux.HandleFunc("POST /v1/subscriptionAvailabilities", func(w http.ResponseWriter, r *http.Request) {
		sched["subAvail"] = true
		posts = append(posts, r.URL.Path)
		w.WriteHeader(201)
		w.Write([]byte(`{"data":{"type":"subscriptionAvailabilities","id":"SA1"}}`))
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			t.Errorf("no bearer on %s", r.URL.Path)
		}
		mux.ServeHTTP(w, r)
	}))
	defer srv.Close()

	c, _ := New("K", "I", pemBytes)
	c.BaseURL = srv.URL
	cat, _ := catalog.Default()
	app := *cat.Find("kirian")

	var out bytes.Buffer
	if err := Provision(context.Background(), Live{C: c}, app, cat.Pass, &out); err != nil {
		t.Fatalf("first run: %v\n%s", err, out.String())
	}
	if len(posts) != 10 {
		t.Fatalf("first run posted %d times: %v", len(posts), posts)
	}
	posts = nil
	out.Reset()
	if err := Provision(context.Background(), Live{C: c}, app, cat.Pass, &out); err != nil {
		t.Fatalf("second run: %v\n%s", err, out.String())
	}
	if len(posts) != 0 {
		t.Fatalf("second run must not POST, got %v", posts)
	}
	if !strings.Contains(out.String(), "IAP com.ol1n.pass.lifetime.kirian exists") {
		t.Fatal(out.String())
	}
}

func TestHTTPErrorIsReadable(t *testing.T) {
	pemBytes, _ := testKeyPEM(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(409)
		w.Write([]byte(`{"errors":[{"status":"409","code":"ENTITY_ERROR.ATTRIBUTE.INVALID","title":"An attribute value is invalid.","detail":"productId is taken"}]}`))
	}))
	defer srv.Close()
	c, _ := New("K", "I", pemBytes)
	c.BaseURL = srv.URL
	_, err := c.Post(context.Background(), "/v2/inAppPurchases", map[string]any{})
	if err == nil || !strings.Contains(err.Error(), "productId is taken") || !strings.Contains(err.Error(), "409") {
		t.Fatalf("%v", err)
	}
	if IsNotFound(err) {
		t.Fatal("409 is not 404")
	}
}
