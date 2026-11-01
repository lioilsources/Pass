package asc

import (
	"context"
	"fmt"
	"io"

	"github.com/lioilsources/Pass/internal/catalog"
)

// Planner implements API without network: every lookup says "missing", so
// Provision walks the full create path and the recorded steps show exactly
// which requests `apply` would send. Existing-state branches are not shown,
// which is the point of a plan.
type Planner struct {
	Steps []string
}

func (p *Planner) step(format string, args ...any) {
	p.Steps = append(p.Steps, fmt.Sprintf(format, args...))
}

// Plan runs Provision against a Planner and writes the numbered steps to w.
func Plan(app catalog.App, pass catalog.Pass, w io.Writer) error {
	p := &Planner{}
	if err := Provision(context.Background(), p, app, pass, io.Discard); err != nil {
		return err
	}
	fmt.Fprintf(w, "App Store Connect plan for %s (%s):\n", app.Name, app.IOSBundleID)
	for i, s := range p.Steps {
		fmt.Fprintf(w, "%2d. %s\n", i+1, s)
	}
	return nil
}

func (p *Planner) FindApp(_ context.Context, bundleID string) (string, error) {
	p.step("GET /v1/apps?filter[bundleId]=%s → <app-id>", bundleID)
	return "<app-id>", nil
}

func (p *Planner) Territories(context.Context) ([]string, error) {
	p.step("GET /v1/territories → <all territories>")
	return []string{"<all territories>"}, nil
}

func (p *Planner) FindIAP(_ context.Context, appID, productID string) (Resource, error) {
	p.step("GET /v1/apps/%s/inAppPurchasesV2 (look for productId %s)", appID, productID)
	return Resource{}, errNotFound
}

func (p *Planner) CreateIAP(_ context.Context, appID, productID, name, _ string) (Resource, error) {
	p.step("POST /v2/inAppPurchases {NON_CONSUMABLE, productId %s, name %q, familySharable, reviewNote} → <iap-id>", productID, name)
	return Resource{ID: "<iap-id>", Type: "inAppPurchases"}, nil
}

func (p *Planner) HasIAPLocalization(_ context.Context, iapID, locale string) (bool, error) {
	p.step("GET /v2/inAppPurchases/%s/inAppPurchaseLocalizations (look for %s)", iapID, locale)
	return false, nil
}

func (p *Planner) CreateIAPLocalization(_ context.Context, iapID, locale, name, _ string) error {
	p.step("POST /v1/inAppPurchaseLocalizations {%s, name %q} for %s", locale, name, iapID)
	return nil
}

func (p *Planner) HasIAPPriceSchedule(_ context.Context, iapID string) (bool, error) {
	p.step("GET /v2/inAppPurchases/%s/iapPriceSchedule", iapID)
	return false, nil
}

func (p *Planner) IAPPricePoint(_ context.Context, iapID, territory string, price float64) (string, error) {
	p.step("GET /v2/inAppPurchases/%s/pricePoints?filter[territory]=%s → price point with customerPrice %.2f", iapID, territory, price)
	return "<price-point-id>", nil
}

func (p *Planner) CreateIAPPriceSchedule(_ context.Context, iapID, baseTerritory, pricePointID string) error {
	p.step("POST /v1/inAppPurchasePriceSchedules {baseTerritory %s, manualPrices → %s} for %s", baseTerritory, pricePointID, iapID)
	return nil
}

func (p *Planner) HasIAPAvailability(_ context.Context, iapID string) (bool, error) {
	p.step("GET /v2/inAppPurchases/%s/inAppPurchaseAvailability", iapID)
	return false, nil
}

func (p *Planner) CreateIAPAvailability(_ context.Context, iapID string, _ []string) error {
	p.step("POST /v1/inAppPurchaseAvailabilities {availableInNewTerritories, all territories} for %s", iapID)
	return nil
}

func (p *Planner) FindSubscriptionGroup(_ context.Context, appID, referenceName string) (Resource, error) {
	p.step("GET /v1/apps/%s/subscriptionGroups (look for referenceName %s)", appID, referenceName)
	return Resource{}, errNotFound
}

func (p *Planner) CreateSubscriptionGroup(_ context.Context, appID, referenceName string) (Resource, error) {
	p.step("POST /v1/subscriptionGroups {referenceName %s} → <group-id>", referenceName)
	return Resource{ID: "<group-id>"}, nil
}

func (p *Planner) HasGroupLocalization(_ context.Context, groupID, locale string) (bool, error) {
	p.step("GET /v1/subscriptionGroups/%s/subscriptionGroupLocalizations (look for %s)", groupID, locale)
	return false, nil
}

func (p *Planner) CreateGroupLocalization(_ context.Context, groupID, locale, name string) error {
	p.step("POST /v1/subscriptionGroupLocalizations {%s, name %q} for %s", locale, name, groupID)
	return nil
}

func (p *Planner) FindSubscription(_ context.Context, groupID, productID string) (Resource, error) {
	p.step("GET /v1/subscriptionGroups/%s/subscriptions (look for productId %s)", groupID, productID)
	return Resource{}, errNotFound
}

func (p *Planner) CreateSubscription(_ context.Context, groupID, productID, name, period, _ string) (Resource, error) {
	p.step("POST /v1/subscriptions {productId %s, name %q, %s, familySharable, reviewNote} → <sub-id>", productID, name, period)
	return Resource{ID: "<sub-id>"}, nil
}

func (p *Planner) HasSubscriptionLocalization(_ context.Context, subID, locale string) (bool, error) {
	p.step("GET /v1/subscriptions/%s/subscriptionLocalizations (look for %s)", subID, locale)
	return false, nil
}

func (p *Planner) CreateSubscriptionLocalization(_ context.Context, subID, locale, name, _ string) error {
	p.step("POST /v1/subscriptionLocalizations {%s, name %q} for %s", locale, name, subID)
	return nil
}

func (p *Planner) HasSubscriptionPrice(_ context.Context, subID string) (bool, error) {
	p.step("GET /v1/subscriptions/%s/prices", subID)
	return false, nil
}

func (p *Planner) SubscriptionPricePoint(_ context.Context, subID, territory string, price float64) (string, error) {
	p.step("GET /v1/subscriptions/%s/pricePoints?filter[territory]=%s → price point with customerPrice %.2f", subID, territory, price)
	return "<sub-price-point-id>", nil
}

func (p *Planner) CreateSubscriptionPrice(_ context.Context, subID, pricePointID string) error {
	p.step("POST /v1/subscriptionPrices {subscriptionPricePoint %s} for %s", pricePointID, subID)
	return nil
}

func (p *Planner) HasSubscriptionAvailability(_ context.Context, subID string) (bool, error) {
	p.step("GET /v1/subscriptions/%s/subscriptionAvailability", subID)
	return false, nil
}

func (p *Planner) CreateSubscriptionAvailability(_ context.Context, subID string, _ []string) error {
	p.step("POST /v1/subscriptionAvailabilities {availableInNewTerritories, all territories} for %s", subID)
	return nil
}
