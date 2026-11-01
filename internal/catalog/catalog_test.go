package catalog

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestDefaultIsValid(t *testing.T) {
	c, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	if c.Find("kirian") == nil {
		t.Fatal("kirian missing from default catalog")
	}
}

// The embedded copy must be the committed source; `make sync-catalog` copies it.
func TestEmbeddedMatchesSource(t *testing.T) {
	src, err := os.ReadFile("../../catalog/apps.json")
	if err != nil {
		t.Fatal(err)
	}
	var a, b any
	if err := json.Unmarshal(src, &a); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(defaultJSON, &b); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatal("internal/catalog/default_apps.json differs from catalog/apps.json — run `make sync-catalog`")
	}
}

func TestProductIDs(t *testing.T) {
	c, _ := Default()
	ids, err := c.ProductIDs("kirian")
	if err != nil {
		t.Fatal(err)
	}
	want := ProductIDs{
		IOSLifetime:     "com.ol1n.pass.lifetime.kirian",
		IOSYearly:       "com.ol1n.pass.yearly.kirian",
		AndroidLifetime: "pass_lifetime",
		AndroidYearly:   "pass_yearly",
	}
	if ids != want {
		t.Fatalf("got %+v want %+v", ids, want)
	}
	if _, err := c.ProductIDs("nope"); err == nil {
		t.Fatal("unknown slug must error")
	}
	if s, ok := c.SlugForProduct("com.ol1n.pass.yearly.lexify"); !ok || s != "lexify" {
		t.Fatalf("SlugForProduct: %q %v", s, ok)
	}
	if _, ok := c.SlugForProduct("com.ol1n.pass.lifetime.unknown"); ok {
		t.Fatal("unknown product must not map")
	}
	if s, ok := c.SlugForPackage("com.ab.kiran"); !ok || s != "kirian" {
		t.Fatalf("SlugForPackage: %q %v", s, ok)
	}
}

func TestTotalUnlockValue(t *testing.T) {
	ten, twenty := 10.0, 20.0
	c := Catalog{Apps: []App{{UnlockValueUSD: &ten}, {UnlockValueUSD: nil}, {UnlockValueUSD: &twenty}}}
	// No app has pass_since yet, so the Pass unlocks nothing today; the
	// listed value is what the catalog will be worth once rolled out.
	if got := c.TotalUnlockValueUSD(); got != 0 {
		t.Fatalf("TotalUnlockValueUSD = %v, want 0", got)
	}
	if got := c.TotalListedValueUSD(); got != 30 {
		t.Fatalf("got %v", got)
	}
}

func TestValidateErrors(t *testing.T) {
	c, _ := Default()
	c.Version = 2
	c.Apps = append(c.Apps, c.Apps[0]) // duplicate slug
	c.Apps[1].IOSBundleID = ""
	bad := "http://insecure"
	c.Apps[1].AppStoreURL = &bad
	err := c.Validate()
	if err == nil {
		t.Fatal("expected errors")
	}
	for _, want := range []string{"version", "duplicate slug", "ios_bundle_id is empty", "not an https URL"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("missing %q in:\n%v", want, err)
		}
	}
}

func TestParseRejectsUnknownFields(t *testing.T) {
	if _, err := Parse([]byte(`{"version":1,"bogus":true}`)); err == nil {
		t.Fatal("unknown field accepted")
	}
}
