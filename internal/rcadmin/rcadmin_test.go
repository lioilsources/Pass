package rcadmin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lioilsources/Pass/internal/catalog"
)

func TestExpectedProducts(t *testing.T) {
	cat, _ := catalog.Default()
	exp := ExpectedProducts(cat)
	if len(exp) != 4*len(cat.Apps) {
		t.Fatal(len(exp))
	}
	if exp[3].StoreIdentifier != "pass_yearly:yearly" || exp[3].Store != "play_store" {
		t.Fatalf("%+v", exp[3])
	}
}

func TestCheckAndApply(t *testing.T) {
	cat, _ := catalog.Default()
	cat.Apps = cat.Apps[:1] // kirian only
	products := []Product{
		{ID: "p1", StoreIdentifier: "com.ol1n.pass.lifetime.kirian", Type: "one_time", AppID: "ios1"},
	}
	attached := []string{}
	var created []map[string]any
	mux := http.NewServeMux()
	page := func(w http.ResponseWriter, items any) {
		json.NewEncoder(w).Encode(map[string]any{"items": items})
	}
	mux.HandleFunc("GET /v2/projects/proj/apps", func(w http.ResponseWriter, r *http.Request) {
		page(w, []map[string]any{
			{"id": "ios1", "name": "Kirian iOS", "type": "app_store", "app_store": map[string]any{"bundle_id": "com.ol1n.kiran"}},
			{"id": "and1", "name": "Kirian Android", "type": "play_store", "play_store": map[string]any{"package_name": "com.ab.kiran"}},
		})
	})
	mux.HandleFunc("GET /v2/projects/proj/products", func(w http.ResponseWriter, r *http.Request) { page(w, products) })
	mux.HandleFunc("POST /v2/projects/proj/products", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		created = append(created, body)
		p := Product{ID: "new" + body["store_identifier"].(string), StoreIdentifier: body["store_identifier"].(string), AppID: body["app_id"].(string)}
		products = append(products, p)
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(p)
	})
	mux.HandleFunc("GET /v2/projects/proj/entitlements", func(w http.ResponseWriter, r *http.Request) {
		page(w, []Entitlement{{ID: "e1", LookupKey: "all_apps"}})
	})
	mux.HandleFunc("GET /v2/projects/proj/entitlements/e1/products", func(w http.ResponseWriter, r *http.Request) {
		var ps []Product
		for _, id := range attached {
			ps = append(ps, Product{ID: id})
		}
		page(w, ps)
	})
	mux.HandleFunc("POST /v2/projects/proj/entitlements/e1/actions/attach_products", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ProductIDs []string `json:"product_ids"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		attached = append(attached, body.ProductIDs...)
		w.Write([]byte(`{}`))
	})
	mux.HandleFunc("GET /v2/projects/proj/offerings", func(w http.ResponseWriter, r *http.Request) {
		page(w, []Offering{{ID: "o1", LookupKey: "pass"}})
	})
	mux.HandleFunc("GET /v2/projects/proj/offerings/o1/packages", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("expand") != "items.product" {
			t.Error("expand missing")
		}
		w.Write([]byte(`{"items":[{"id":"pk1","lookup_key":"$rc_lifetime","products":{"items":[{"product":{"id":"p1"}}]}}]}`))
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk" {
			w.WriteHeader(401)
			return
		}
		mux.ServeHTTP(w, r)
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL + "/v2", SecretKey: "sk", ProjectID: "proj", HTTP: srv.Client()}
	var out bytes.Buffer
	rep, err := Check(context.Background(), c, cat, &out)
	if err != nil {
		t.Fatal(err)
	}
	if rep.OK || len(rep.MissingProducts) != 3 || len(rep.Unattached) != 1 {
		t.Fatalf("%+v\n%s", rep, out.String())
	}
	if !strings.Contains(out.String(), "pass_yearly:yearly") {
		t.Fatal(out.String())
	}

	out.Reset()
	if err := Apply(context.Background(), c, cat, &out); err != nil {
		t.Fatal(err)
	}
	if len(created) != 3 {
		t.Fatalf("created %d", len(created))
	}
	if created[0]["app_id"] != "ios1" || created[0]["type"] != "subscription" {
		t.Fatalf("%v", created[0])
	}
	if len(attached) != 4 {
		t.Fatalf("attached %v", attached)
	}
	if !strings.Contains(out.String(), "not in offering") {
		t.Fatal(out.String())
	}
	rep, _ = Check(context.Background(), c, cat, &out)
	if len(rep.MissingProducts) != 0 || len(rep.Unattached) != 0 || len(rep.NotInOffering) != 3 {
		t.Fatalf("%+v", rep)
	}
}
