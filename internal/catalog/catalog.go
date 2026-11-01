// Package catalog loads and validates apps.json, the single source of truth for
// which Ol1n apps take part in Ol1n Pass and what the Pass is worth
// (docs/CONTRACTS.md §3). Store product IDs are derived from the slug (§2) and
// never written by hand.
package catalog

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
)

// SchemaVersion is the major version of apps.json this code understands.
const SchemaVersion = 1

// Entitlement is the one RevenueCat entitlement every Pass product grants.
const Entitlement = "all_apps"

//go:embed default_apps.json
var defaultJSON []byte

// Catalog is the parsed apps.json.
type Catalog struct {
	Version     int    `json:"version"`
	Updated     string `json:"updated"`
	Entitlement string `json:"entitlement"`
	Pass        Pass   `json:"pass"`
	Apps        []App  `json:"apps"`
}

// Pass describes the product itself.
type Pass struct {
	Name                  string  `json:"name"`
	LifetimeUSD           float64 `json:"lifetime_usd"`
	YearlyUSD             float64 `json:"yearly_usd"`
	LandingURL            string  `json:"landing_url"`
	CatalogURL            string  `json:"catalog_url"`
	APIURL                string  `json:"api_url"`
	IOSSubscriptionGroup  string  `json:"ios_subscription_group"`
	AndroidYearlyBasePlan string  `json:"android_yearly_base_plan"`
}

// App is one participating app.
type App struct {
	Slug           string   `json:"slug"`
	Name           string   `json:"name"`
	IOSBundleID    string   `json:"ios_bundle_id"`
	AndroidPackage string   `json:"android_package"`
	AppStoreID     *string  `json:"appstore_id"`
	AppStoreURL    *string  `json:"appstore_url"`
	PlayStoreURL   *string  `json:"playstore_url"`
	UnlockValueUSD *float64 `json:"unlock_value_usd"`
	Unlocks        []string `json:"unlocks"`
	Kids           bool     `json:"kids"`
	PassSince      *string  `json:"pass_since"`
	Notes          string   `json:"notes,omitempty"`
}

// ProductIDs are the store identifiers for one app (CONTRACTS §2).
type ProductIDs struct {
	IOSLifetime     string
	IOSYearly       string
	AndroidLifetime string
	AndroidYearly   string
}

// Android product IDs are per package, so they are the same in every app.
const (
	AndroidLifetimeSKU = "pass_lifetime"
	AndroidYearlySKU   = "pass_yearly"
	iosProductPrefix   = "com.ol1n.pass."
)

// ProductIDsFor derives the store product IDs for a slug.
func ProductIDsFor(slug string) ProductIDs {
	return ProductIDs{
		IOSLifetime:     iosProductPrefix + "lifetime." + slug,
		IOSYearly:       iosProductPrefix + "yearly." + slug,
		AndroidLifetime: AndroidLifetimeSKU,
		AndroidYearly:   AndroidYearlySKU,
	}
}

// ProductIDs returns the store product IDs of an app, or an error when the slug
// is not in the catalog.
func (c *Catalog) ProductIDs(slug string) (ProductIDs, error) {
	if c.Find(slug) == nil {
		return ProductIDs{}, fmt.Errorf("catalog: unknown app %q", slug)
	}
	return ProductIDsFor(slug), nil
}

// Find returns the app with the slug, or nil.
func (c *Catalog) Find(slug string) *App {
	for i := range c.Apps {
		if c.Apps[i].Slug == slug {
			return &c.Apps[i]
		}
	}
	return nil
}

// SlugForProduct maps an iOS product ID back to its slug. Android SKUs are
// shared, so they cannot be mapped without the package; see SlugForPackage.
func (c *Catalog) SlugForProduct(productID string) (string, bool) {
	for _, kind := range []string{"lifetime.", "yearly."} {
		if rest, ok := strings.CutPrefix(productID, iosProductPrefix+kind); ok && c.Find(rest) != nil {
			return rest, true
		}
	}
	return "", false
}

// SlugForPackage maps an Android package or iOS bundle id back to its slug.
func (c *Catalog) SlugForPackage(id string) (string, bool) {
	for _, a := range c.Apps {
		if a.AndroidPackage == id || a.IOSBundleID == id {
			return a.Slug, true
		}
	}
	return "", false
}

// TotalUnlockValueUSD sums unlock_value_usd over the apps the Pass unlocks
// today (pass_since set). This is the "value of all packs today" number the
// paywall and the landing page anchor the price with; it must never count an
// app where the Pass does not work yet (Apple 2.3 metadata accuracy).
func (c *Catalog) TotalUnlockValueUSD() float64 {
	var sum float64
	for _, a := range c.Apps {
		if a.UnlockValueUSD != nil && a.PassSince != nil && *a.PassSince != "" {
			sum += *a.UnlockValueUSD
		}
	}
	return sum
}

// TotalListedValueUSD sums unlock_value_usd over every app with a known
// value, wired in or not — what the catalog will be worth once rolled out.
func (c *Catalog) TotalListedValueUSD() float64 {
	var sum float64
	for _, a := range c.Apps {
		if a.UnlockValueUSD != nil {
			sum += *a.UnlockValueUSD
		}
	}
	return sum
}

var slugRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// Validate checks the structural rules from CONTRACTS §3 and returns every
// problem found, joined.
func (c *Catalog) Validate() error {
	var errs []error
	add := func(format string, args ...any) { errs = append(errs, fmt.Errorf(format, args...)) }

	if c.Version != SchemaVersion {
		add("version: got %d, this build understands %d", c.Version, SchemaVersion)
	}
	if c.Entitlement != Entitlement {
		add("entitlement: got %q, want %q", c.Entitlement, Entitlement)
	}
	if c.Pass.LifetimeUSD <= 0 {
		add("pass.lifetime_usd must be positive")
	}
	if c.Pass.YearlyUSD <= 0 {
		add("pass.yearly_usd must be positive")
	}
	for _, u := range []struct{ name, val string }{
		{"pass.landing_url", c.Pass.LandingURL},
		{"pass.catalog_url", c.Pass.CatalogURL},
		{"pass.api_url", c.Pass.APIURL},
	} {
		if err := checkURL(u.val); err != nil {
			add("%s: %v", u.name, err)
		}
	}
	if c.Pass.IOSSubscriptionGroup == "" {
		add("pass.ios_subscription_group is empty")
	}
	if c.Pass.AndroidYearlyBasePlan == "" {
		add("pass.android_yearly_base_plan is empty")
	}
	if len(c.Apps) == 0 {
		add("apps: empty")
	}
	seen := map[string]bool{}
	for i, a := range c.Apps {
		where := fmt.Sprintf("apps[%d] (%q)", i, a.Slug)
		if !slugRe.MatchString(a.Slug) {
			add("%s: slug must match %s", where, slugRe)
		}
		if seen[a.Slug] {
			add("%s: duplicate slug", where)
		}
		seen[a.Slug] = true
		if a.Name == "" {
			add("%s: name is empty", where)
		}
		if a.IOSBundleID == "" {
			add("%s: ios_bundle_id is empty", where)
		}
		if a.AndroidPackage == "" {
			add("%s: android_package is empty", where)
		}
		if a.AppStoreURL != nil {
			if err := checkURL(*a.AppStoreURL); err != nil {
				add("%s: appstore_url: %v", where, err)
			}
		}
		if a.PlayStoreURL != nil {
			if err := checkURL(*a.PlayStoreURL); err != nil {
				add("%s: playstore_url: %v", where, err)
			}
		}
		if a.UnlockValueUSD != nil && *a.UnlockValueUSD < 0 {
			add("%s: unlock_value_usd is negative", where)
		}
	}
	return errors.Join(errs...)
}

func checkURL(s string) error {
	u, err := url.Parse(s)
	if err != nil {
		return err
	}
	if u.Scheme != "https" || u.Host == "" {
		return fmt.Errorf("%q is not an https URL", s)
	}
	return nil
}

// Parse decodes and validates apps.json bytes.
func Parse(data []byte) (*Catalog, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var c Catalog
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("catalog: parse: %w", err)
	}
	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("catalog: invalid:\n%w", err)
	}
	return &c, nil
}

// Load reads apps.json from a path.
func Load(path string) (*Catalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("catalog: %w", err)
	}
	return Parse(data)
}

// Default returns the catalog embedded at build time (a copy of
// catalog/apps.json kept in sync by `make sync-catalog`).
func Default() (*Catalog, error) { return Parse(defaultJSON) }

// DefaultJSON returns the embedded apps.json bytes, served verbatim by pass-api.
func DefaultJSON() []byte { return bytes.Clone(defaultJSON) }
