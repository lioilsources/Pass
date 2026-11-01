// Package rcadmin checks and completes the RevenueCat project configuration
// through the REST API v2 (docs/CONTRACTS.md §5): every Pass product of every
// app in apps.json must exist in the matching RevenueCat app and be attached
// to the entitlement `all_apps`, and the offering `pass` must carry them.
//
// Endpoints follow https://www.revenuecat.com/docs/api-v2. Where the API
// changes, the error carries the status and body so the request can be fixed
// from the log.
package rcadmin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/lioilsources/Pass/internal/catalog"
)

// DefaultBaseURL is the production API.
const DefaultBaseURL = "https://api.revenuecat.com/v2"

// Client is a thin v2 client.
type Client struct {
	BaseURL   string
	SecretKey string
	ProjectID string
	HTTP      *http.Client
}

// NewFromEnv reads RC_SECRET_API_KEY (a v2 secret key) and RC_PROJECT_ID.
func NewFromEnv() (*Client, error) {
	key, proj := os.Getenv("RC_SECRET_API_KEY"), os.Getenv("RC_PROJECT_ID")
	if key == "" || proj == "" {
		return nil, errors.New("rcadmin: RC_SECRET_API_KEY and RC_PROJECT_ID are required")
	}
	return &Client{BaseURL: DefaultBaseURL, SecretKey: key, ProjectID: proj, HTTP: &http.Client{Timeout: 30 * time.Second}}, nil
}

// App is a RevenueCat app (one per store per Ol1n app).
type App struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"` // app_store | play_store | …
	// Store-specific blocks; only the identifiers are read.
	AppStore *struct {
		BundleID string `json:"bundle_id"`
	} `json:"app_store,omitempty"`
	PlayStore *struct {
		PackageName string `json:"package_name"`
	} `json:"play_store,omitempty"`
}

// StoreIdentifier returns the bundle id or package name.
func (a App) StoreIdentifier() string {
	switch {
	case a.AppStore != nil:
		return a.AppStore.BundleID
	case a.PlayStore != nil:
		return a.PlayStore.PackageName
	}
	return ""
}

// Product is a RevenueCat product.
type Product struct {
	ID              string `json:"id"`
	StoreIdentifier string `json:"store_identifier"`
	Type            string `json:"type"` // subscription | one_time | non_renewing | …
	AppID           string `json:"app_id"`
	DisplayName     string `json:"display_name"`
}

// Entitlement is a RevenueCat entitlement.
type Entitlement struct {
	ID          string `json:"id"`
	LookupKey   string `json:"lookup_key"`
	DisplayName string `json:"display_name"`
}

// Offering is a RevenueCat offering.
type Offering struct {
	ID          string `json:"id"`
	LookupKey   string `json:"lookup_key"`
	DisplayName string `json:"display_name"`
	IsCurrent   bool   `json:"is_current"`
}

// Package is a package inside an offering.
type Package struct {
	ID        string `json:"id"`
	LookupKey string `json:"lookup_key"`
	Products  *struct {
		Items []struct {
			Product Product `json:"product"`
		} `json:"items"`
	} `json:"products,omitempty"`
}

type page[T any] struct {
	Items    []T    `json:"items"`
	NextPage string `json:"next_page"`
}

func (c *Client) do(ctx context.Context, method, path string, q url.Values, body, out any) error {
	u := path
	if !strings.HasPrefix(path, "http") {
		u = strings.TrimRight(c.BaseURL, "/") + path
	}
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, rd)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.SecretKey)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("rcadmin: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("rcadmin: %s %s → %d: %s", method, path, resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	if out != nil && len(raw) > 0 {
		return json.Unmarshal(raw, out)
	}
	return nil
}

func (c *Client) project(path string) string {
	return "/projects/" + url.PathEscape(c.ProjectID) + path
}

func list[T any](ctx context.Context, c *Client, path string, q url.Values) ([]T, error) {
	if q == nil {
		q = url.Values{}
	}
	q.Set("limit", "100")
	var all []T
	next := path
	for next != "" {
		var p page[T]
		if err := c.do(ctx, http.MethodGet, next, q, nil, &p); err != nil {
			return nil, err
		}
		all = append(all, p.Items...)
		next = p.NextPage
		q = nil // next_page already carries its query
	}
	return all, nil
}

// Apps lists the project's apps.
func (c *Client) Apps(ctx context.Context) ([]App, error) {
	return list[App](ctx, c, c.project("/apps"), nil)
}

// Products lists all products.
func (c *Client) Products(ctx context.Context) ([]Product, error) {
	return list[Product](ctx, c, c.project("/products"), nil)
}

// Entitlements lists entitlements.
func (c *Client) Entitlements(ctx context.Context) ([]Entitlement, error) {
	return list[Entitlement](ctx, c, c.project("/entitlements"), nil)
}

// EntitlementProducts lists the products attached to an entitlement.
func (c *Client) EntitlementProducts(ctx context.Context, entitlementID string) ([]Product, error) {
	return list[Product](ctx, c, c.project("/entitlements/"+url.PathEscape(entitlementID)+"/products"), nil)
}

// Offerings lists offerings.
func (c *Client) Offerings(ctx context.Context) ([]Offering, error) {
	return list[Offering](ctx, c, c.project("/offerings"), nil)
}

// Packages lists an offering's packages with their products expanded.
func (c *Client) Packages(ctx context.Context, offeringID string) ([]Package, error) {
	return list[Package](ctx, c, c.project("/offerings/"+url.PathEscape(offeringID)+"/packages"), url.Values{"expand": {"items.product"}})
}

// CreateProduct creates a product in a RevenueCat app.
// https://www.revenuecat.com/docs/api-v2#tag/Product/operation/create-product
func (c *Client) CreateProduct(ctx context.Context, appID, storeIdentifier, typ, displayName string) (Product, error) {
	var p Product
	body := map[string]any{"store_identifier": storeIdentifier, "app_id": appID, "type": typ, "display_name": displayName}
	err := c.do(ctx, http.MethodPost, c.project("/products"), nil, body, &p)
	return p, err
}

// AttachProducts attaches products to an entitlement.
func (c *Client) AttachProducts(ctx context.Context, entitlementID string, productIDs []string) error {
	return c.do(ctx, http.MethodPost, c.project("/entitlements/"+url.PathEscape(entitlementID)+"/actions/attach_products"), nil,
		map[string]any{"product_ids": productIDs}, nil)
}

// CreateEntitlement creates an entitlement.
func (c *Client) CreateEntitlement(ctx context.Context, lookupKey, displayName string) (Entitlement, error) {
	var e Entitlement
	err := c.do(ctx, http.MethodPost, c.project("/entitlements"), nil, map[string]any{"lookup_key": lookupKey, "display_name": displayName}, &e)
	return e, err
}

// ---------------------------------------------------------------------------

// Expected is one product the catalog says must exist in RevenueCat.
type Expected struct {
	Slug            string
	Store           string // app_store | play_store
	StoreAppID      string // bundle id / package
	StoreIdentifier string // RC store_identifier
	Type            string // one_time | subscription
	DisplayName     string
}

// ExpectedProducts derives the RC product list from the catalog. Play
// subscriptions are identified as `productId:basePlanId`.
func ExpectedProducts(cat *catalog.Catalog) []Expected {
	var out []Expected
	for _, a := range cat.Apps {
		ids := catalog.ProductIDsFor(a.Slug)
		out = append(out,
			Expected{a.Slug, "app_store", a.IOSBundleID, ids.IOSLifetime, "one_time", "Ol1n Pass"},
			Expected{a.Slug, "app_store", a.IOSBundleID, ids.IOSYearly, "subscription", "Ol1n Pass Yearly"},
			Expected{a.Slug, "play_store", a.AndroidPackage, ids.AndroidLifetime, "one_time", "Ol1n Pass"},
			Expected{a.Slug, "play_store", a.AndroidPackage, ids.AndroidYearly + ":" + cat.Pass.AndroidYearlyBasePlan, "subscription", "Ol1n Pass Yearly"},
		)
	}
	return out
}

// Report is the outcome of Check.
type Report struct {
	Lines           []string
	MissingApps     []Expected // no RC app for that store identifier
	MissingProducts []Expected
	Unattached      []Product // exist, not attached to all_apps
	MissingOffering bool
	NotInOffering   []Product
	EntitlementID   string
	OK              bool
}

// Check compares RevenueCat with the catalog.
func Check(ctx context.Context, c *Client, cat *catalog.Catalog, w io.Writer) (*Report, error) {
	rep := &Report{}
	apps, err := c.Apps(ctx)
	if err != nil {
		return nil, err
	}
	products, err := c.Products(ctx)
	if err != nil {
		return nil, err
	}
	ents, err := c.Entitlements(ctx)
	if err != nil {
		return nil, err
	}
	var allApps *Entitlement
	for i := range ents {
		if ents[i].LookupKey == catalog.Entitlement {
			allApps = &ents[i]
		}
	}
	attached := map[string]bool{}
	if allApps != nil {
		rep.EntitlementID = allApps.ID
		ps, err := c.EntitlementProducts(ctx, allApps.ID)
		if err != nil {
			return nil, err
		}
		for _, p := range ps {
			attached[p.ID] = true
		}
	}
	offerings, err := c.Offerings(ctx)
	if err != nil {
		return nil, err
	}
	inOffering := map[string]bool{}
	rep.MissingOffering = true
	for _, o := range offerings {
		if o.LookupKey != "pass" {
			continue
		}
		rep.MissingOffering = false
		pkgs, err := c.Packages(ctx, o.ID)
		if err != nil {
			return nil, err
		}
		for _, pk := range pkgs {
			if pk.Products == nil {
				continue
			}
			for _, it := range pk.Products.Items {
				inOffering[it.Product.ID] = true
			}
		}
	}

	appByStore := map[string]App{}
	for _, a := range apps {
		if id := a.StoreIdentifier(); id != "" {
			appByStore[a.Type+"/"+id] = a
		}
	}
	prodByKey := map[string]Product{}
	for _, p := range products {
		prodByKey[p.AppID+"/"+p.StoreIdentifier] = p
	}

	fmt.Fprintf(w, "%-20s %-11s %-42s %-8s %-9s %-9s\n", "app", "store", "store_identifier", "exists", "all_apps", "offering")
	for _, e := range ExpectedProducts(cat) {
		app, ok := appByStore[e.Store+"/"+e.StoreAppID]
		if !ok {
			rep.MissingApps = append(rep.MissingApps, e)
			fmt.Fprintf(w, "%-20s %-11s %-42s %-8s\n", e.Slug, e.Store, e.StoreIdentifier, "NO APP")
			continue
		}
		p, ok := prodByKey[app.ID+"/"+e.StoreIdentifier]
		if !ok {
			rep.MissingProducts = append(rep.MissingProducts, e)
			fmt.Fprintf(w, "%-20s %-11s %-42s %-8s\n", e.Slug, e.Store, e.StoreIdentifier, "missing")
			continue
		}
		att, off := attached[p.ID], inOffering[p.ID]
		if !att {
			rep.Unattached = append(rep.Unattached, p)
		}
		if !off {
			rep.NotInOffering = append(rep.NotInOffering, p)
		}
		fmt.Fprintf(w, "%-20s %-11s %-42s %-8s %-9s %-9s\n", e.Slug, e.Store, e.StoreIdentifier, "yes", yn(att), yn(off))
	}
	if allApps == nil {
		fmt.Fprintln(w, "entitlement all_apps: MISSING")
	}
	if rep.MissingOffering {
		fmt.Fprintln(w, "offering pass: MISSING (create it in the dashboard with packages $rc_lifetime and $rc_annual; the API v2 creates offerings but package/product wiring is easier by hand)")
	}
	rep.OK = allApps != nil && !rep.MissingOffering && len(rep.MissingApps)+len(rep.MissingProducts)+len(rep.Unattached)+len(rep.NotInOffering) == 0
	return rep, nil
}

func yn(b bool) string {
	if b {
		return "yes"
	}
	return "NO"
}

// Apply creates missing products and attaches every Pass product to all_apps.
// RevenueCat apps and the offering are created by hand (they need store
// credentials); missing apps are reported, not created.
func Apply(ctx context.Context, c *Client, cat *catalog.Catalog, w io.Writer) error {
	rep, err := Check(ctx, c, cat, io.Discard)
	if err != nil {
		return err
	}
	if rep.EntitlementID == "" {
		e, err := c.CreateEntitlement(ctx, catalog.Entitlement, "All Ol1n apps")
		if err != nil {
			return err
		}
		rep.EntitlementID = e.ID
		fmt.Fprintf(w, "created entitlement %s → %s\n", catalog.Entitlement, e.ID)
	}
	for _, e := range rep.MissingApps {
		fmt.Fprintf(w, "manual: add RevenueCat app for %s (%s %s) in the dashboard, then re-run\n", e.Slug, e.Store, e.StoreAppID)
	}
	apps, err := c.Apps(ctx)
	if err != nil {
		return err
	}
	appByStore := map[string]string{}
	for _, a := range apps {
		appByStore[a.Type+"/"+a.StoreIdentifier()] = a.ID
	}
	var toAttach []string
	created := 0
	for _, e := range rep.MissingProducts {
		appID := appByStore[e.Store+"/"+e.StoreAppID]
		if appID == "" {
			continue
		}
		p, err := c.CreateProduct(ctx, appID, e.StoreIdentifier, e.Type, e.DisplayName)
		if err != nil {
			return fmt.Errorf("create product %s: %w", e.StoreIdentifier, err)
		}
		fmt.Fprintf(w, "created product %s (%s) → %s\n", e.StoreIdentifier, e.Store, p.ID)
		toAttach = append(toAttach, p.ID)
		created++
	}
	for _, p := range rep.Unattached {
		toAttach = append(toAttach, p.ID)
	}
	sort.Strings(toAttach)
	if len(toAttach) > 0 {
		if err := c.AttachProducts(ctx, rep.EntitlementID, toAttach); err != nil {
			return err
		}
		fmt.Fprintf(w, "attached %d products to %s\n", len(toAttach), catalog.Entitlement)
	}
	if rep.MissingOffering {
		fmt.Fprintln(w, "manual: create offering `pass` with packages $rc_lifetime / $rc_annual and add the products (dashboard)")
	} else if n := len(rep.NotInOffering) + created; n > 0 { // freshly created products are never in the offering yet
		fmt.Fprintf(w, "manual: %d products are not in offering `pass` yet — add them to the packages in the dashboard\n", n)
	}
	return nil
}
