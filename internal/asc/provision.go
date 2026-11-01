package asc

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"

	"github.com/lioilsources/Pass/internal/catalog"
)

// Text shared by every Pass product in every app.
const (
	LifetimeName       = "Ol1n Pass"
	YearlyName         = "Ol1n Pass Yearly"
	Locale             = "en-US"
	BaseTerritory      = "USA"
	ReviewNote         = "Ol1n Pass is a non-consumable purchased via In-App Purchase; the entitlement is shared across our own apps via our server (RevenueCat). The yearly subscription grants the same entitlement for one year. Test account: see App Review notes of the app."
	LifetimeDesc       = "Unlocks all premium features in every app by Ol1n, including apps released later. Current list: olin.now/pass"
	YearlyDesc         = "One year of all premium features in every app by Ol1n, including apps released later. Current list: olin.now/pass"
	SubscriptionPeriod = "ONE_YEAR"
)

// API is what the provisioner needs from App Store Connect. Every "Find"
// returns IsNotFound-errors for missing things; every "Create" returns the id.
type API interface {
	FindApp(ctx context.Context, bundleID string) (appID string, err error)
	Territories(ctx context.Context) ([]string, error)

	FindIAP(ctx context.Context, appID, productID string) (Resource, error)
	CreateIAP(ctx context.Context, appID, productID, name, reviewNote string) (Resource, error)
	HasIAPLocalization(ctx context.Context, iapID, locale string) (bool, error)
	CreateIAPLocalization(ctx context.Context, iapID, locale, name, desc string) error
	HasIAPPriceSchedule(ctx context.Context, iapID string) (bool, error)
	IAPPricePoint(ctx context.Context, iapID, territory string, price float64) (pricePointID string, err error)
	CreateIAPPriceSchedule(ctx context.Context, iapID, baseTerritory, pricePointID string) error
	HasIAPAvailability(ctx context.Context, iapID string) (bool, error)
	CreateIAPAvailability(ctx context.Context, iapID string, territories []string) error

	FindSubscriptionGroup(ctx context.Context, appID, referenceName string) (Resource, error)
	CreateSubscriptionGroup(ctx context.Context, appID, referenceName string) (Resource, error)
	HasGroupLocalization(ctx context.Context, groupID, locale string) (bool, error)
	CreateGroupLocalization(ctx context.Context, groupID, locale, name string) error
	FindSubscription(ctx context.Context, groupID, productID string) (Resource, error)
	CreateSubscription(ctx context.Context, groupID, productID, name, period, reviewNote string) (Resource, error)
	HasSubscriptionLocalization(ctx context.Context, subID, locale string) (bool, error)
	CreateSubscriptionLocalization(ctx context.Context, subID, locale, name, desc string) error
	HasSubscriptionPrice(ctx context.Context, subID string) (bool, error)
	SubscriptionPricePoint(ctx context.Context, subID, territory string, price float64) (string, error)
	CreateSubscriptionPrice(ctx context.Context, subID, pricePointID string) error
	HasSubscriptionAvailability(ctx context.Context, subID string) (bool, error)
	CreateSubscriptionAvailability(ctx context.Context, subID string, territories []string) error
}

// Provision creates whatever is missing for one app and reports each step to w.
// It never deletes or changes prices of existing products.
func Provision(ctx context.Context, api API, app catalog.App, pass catalog.Pass, w io.Writer) error {
	ids := catalog.ProductIDsFor(app.Slug)
	say := func(format string, args ...any) { fmt.Fprintf(w, format+"\n", args...) }

	appID, err := api.FindApp(ctx, app.IOSBundleID)
	if err != nil {
		return fmt.Errorf("app %s: %w", app.IOSBundleID, err)
	}
	say("app %s (%s) → id %s", app.Name, app.IOSBundleID, appID)

	territories, err := api.Territories(ctx)
	if err != nil {
		return err
	}

	// ---- lifetime non-consumable ----
	iap, err := api.FindIAP(ctx, appID, ids.IOSLifetime)
	switch {
	case IsNotFound(err):
		iap, err = api.CreateIAP(ctx, appID, ids.IOSLifetime, LifetimeName, ReviewNote)
		if err != nil {
			return fmt.Errorf("create IAP %s: %w", ids.IOSLifetime, err)
		}
		say("created IAP %s → %s", ids.IOSLifetime, iap.ID)
	case err != nil:
		return err
	default:
		say("IAP %s exists → %s (state %s)", ids.IOSLifetime, iap.ID, iap.Str("state"))
	}
	if err := ensure(ctx, w, "IAP localization "+Locale,
		func() (bool, error) { return api.HasIAPLocalization(ctx, iap.ID, Locale) },
		func() error { return api.CreateIAPLocalization(ctx, iap.ID, Locale, LifetimeName, LifetimeDesc) }); err != nil {
		return err
	}
	if err := ensure(ctx, w, fmt.Sprintf("IAP price schedule %s %.2f USD", BaseTerritory, pass.LifetimeUSD),
		func() (bool, error) { return api.HasIAPPriceSchedule(ctx, iap.ID) },
		func() error {
			pp, err := api.IAPPricePoint(ctx, iap.ID, BaseTerritory, pass.LifetimeUSD)
			if err != nil {
				return err
			}
			return api.CreateIAPPriceSchedule(ctx, iap.ID, BaseTerritory, pp)
		}); err != nil {
		return err
	}
	if err := ensure(ctx, w, fmt.Sprintf("IAP availability (%d territories)", len(territories)),
		func() (bool, error) { return api.HasIAPAvailability(ctx, iap.ID) },
		func() error { return api.CreateIAPAvailability(ctx, iap.ID, territories) }); err != nil {
		return err
	}

	// ---- yearly subscription ----
	group, err := api.FindSubscriptionGroup(ctx, appID, pass.IOSSubscriptionGroup)
	switch {
	case IsNotFound(err):
		group, err = api.CreateSubscriptionGroup(ctx, appID, pass.IOSSubscriptionGroup)
		if err != nil {
			return fmt.Errorf("create subscription group: %w", err)
		}
		say("created subscription group %s → %s", pass.IOSSubscriptionGroup, group.ID)
	case err != nil:
		return err
	default:
		say("subscription group %s exists → %s", pass.IOSSubscriptionGroup, group.ID)
	}
	if err := ensure(ctx, w, "group localization "+Locale,
		func() (bool, error) { return api.HasGroupLocalization(ctx, group.ID, Locale) },
		func() error { return api.CreateGroupLocalization(ctx, group.ID, Locale, LifetimeName) }); err != nil {
		return err
	}
	sub, err := api.FindSubscription(ctx, group.ID, ids.IOSYearly)
	switch {
	case IsNotFound(err):
		sub, err = api.CreateSubscription(ctx, group.ID, ids.IOSYearly, YearlyName, SubscriptionPeriod, ReviewNote)
		if err != nil {
			return fmt.Errorf("create subscription %s: %w", ids.IOSYearly, err)
		}
		say("created subscription %s → %s", ids.IOSYearly, sub.ID)
	case err != nil:
		return err
	default:
		say("subscription %s exists → %s (state %s)", ids.IOSYearly, sub.ID, sub.Str("state"))
	}
	if err := ensure(ctx, w, "subscription localization "+Locale,
		func() (bool, error) { return api.HasSubscriptionLocalization(ctx, sub.ID, Locale) },
		func() error { return api.CreateSubscriptionLocalization(ctx, sub.ID, Locale, YearlyName, YearlyDesc) }); err != nil {
		return err
	}
	if err := ensure(ctx, w, fmt.Sprintf("subscription price %s %.2f USD", BaseTerritory, pass.YearlyUSD),
		func() (bool, error) { return api.HasSubscriptionPrice(ctx, sub.ID) },
		func() error {
			pp, err := api.SubscriptionPricePoint(ctx, sub.ID, BaseTerritory, pass.YearlyUSD)
			if err != nil {
				return err
			}
			return api.CreateSubscriptionPrice(ctx, sub.ID, pp)
		}); err != nil {
		return err
	}
	if err := ensure(ctx, w, fmt.Sprintf("subscription availability (%d territories)", len(territories)),
		func() (bool, error) { return api.HasSubscriptionAvailability(ctx, sub.ID) },
		func() error { return api.CreateSubscriptionAvailability(ctx, sub.ID, territories) }); err != nil {
		return err
	}

	say("manual: upload the review screenshot (paywall with the app list) for both products in App Store Connect; it cannot be set through the API without an asset upload flow.")
	say("manual: Family Sharing is requested at creation (familySharable=true); verify it shows as enabled.")
	return nil
}

func ensure(_ context.Context, w io.Writer, what string, has func() (bool, error), create func() error) error {
	ok, err := has()
	if err != nil {
		return fmt.Errorf("%s: check: %w", what, err)
	}
	if ok {
		fmt.Fprintf(w, "%s exists\n", what)
		return nil
	}
	if err := create(); err != nil {
		return fmt.Errorf("%s: create: %w", what, err)
	}
	fmt.Fprintf(w, "created %s\n", what)
	return nil
}

// ---------------------------------------------------------------------------
// HTTP implementation of API.

// Live adapts Client to API.
type Live struct{ C *Client }

func rel(typ, id string) map[string]any {
	return map[string]any{"data": map[string]any{"type": typ, "id": id}}
}

func relMany(typ string, ids []string) map[string]any {
	data := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		data = append(data, map[string]any{"type": typ, "id": id})
	}
	return map[string]any{"data": data}
}

func (l Live) FindApp(ctx context.Context, bundleID string) (string, error) {
	doc, err := l.C.Get(ctx, "/v1/apps", url.Values{"filter[bundleId]": {bundleID}, "limit": {"2"}})
	if err != nil {
		return "", err
	}
	apps, err := doc.Many()
	if err != nil {
		return "", err
	}
	if len(apps) != 1 {
		return "", fmt.Errorf("asc: %d apps match bundle id %s", len(apps), bundleID)
	}
	return apps[0].ID, nil
}

func (l Live) Territories(ctx context.Context) ([]string, error) {
	doc, err := l.C.Get(ctx, "/v1/territories", url.Values{"limit": {"200"}})
	if err != nil {
		return nil, err
	}
	ts, err := doc.Many()
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(ts))
	for _, t := range ts {
		out = append(out, t.ID)
	}
	return out, nil
}

func findByAttr(doc *Document, attr, want string) (Resource, error) {
	rs, err := doc.Many()
	if err != nil {
		return Resource{}, err
	}
	for _, r := range rs {
		if r.Str(attr) == want {
			return r, nil
		}
	}
	return Resource{}, errNotFound
}

func (l Live) FindIAP(ctx context.Context, appID, productID string) (Resource, error) {
	doc, err := l.C.Get(ctx, "/v1/apps/"+appID+"/inAppPurchasesV2", url.Values{"limit": {"200"}})
	if err != nil {
		return Resource{}, err
	}
	return findByAttr(doc, "productId", productID)
}

func (l Live) CreateIAP(ctx context.Context, appID, productID, name, reviewNote string) (Resource, error) {
	// https://developer.apple.com/documentation/appstoreconnectapi/inapppurchasev2create
	body := map[string]any{"data": map[string]any{
		"type": "inAppPurchases",
		"attributes": map[string]any{
			"name": name, "productId": productID, "inAppPurchaseType": "NON_CONSUMABLE",
			"reviewNote": reviewNote, "familySharable": true,
		},
		"relationships": map[string]any{"app": rel("apps", appID)},
	}}
	doc, err := l.C.Post(ctx, "/v2/inAppPurchases", body)
	if err != nil {
		return Resource{}, err
	}
	return doc.One()
}

func (l Live) HasIAPLocalization(ctx context.Context, iapID, locale string) (bool, error) {
	doc, err := l.C.Get(ctx, "/v2/inAppPurchases/"+iapID+"/inAppPurchaseLocalizations", nil)
	if err != nil {
		return false, err
	}
	_, err = findByAttr(doc, "locale", locale)
	return err == nil, nil
}

func (l Live) CreateIAPLocalization(ctx context.Context, iapID, locale, name, desc string) error {
	body := map[string]any{"data": map[string]any{
		"type":          "inAppPurchaseLocalizations",
		"attributes":    map[string]any{"locale": locale, "name": name, "description": desc},
		"relationships": map[string]any{"inAppPurchaseV2": rel("inAppPurchases", iapID)},
	}}
	_, err := l.C.Post(ctx, "/v1/inAppPurchaseLocalizations", body)
	return err
}

func (l Live) HasIAPPriceSchedule(ctx context.Context, iapID string) (bool, error) {
	doc, err := l.C.Get(ctx, "/v2/inAppPurchases/"+iapID+"/iapPriceSchedule", nil)
	if IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	_, err = doc.One()
	return err == nil, nil
}

func pickPricePoint(doc *Document, price float64) (string, error) {
	pps, err := doc.Many()
	if err != nil {
		return "", err
	}
	for _, pp := range pps {
		if f, err := strconv.ParseFloat(pp.Str("customerPrice"), 64); err == nil && f == price {
			return pp.ID, nil
		}
	}
	var seen []string
	for _, pp := range pps {
		seen = append(seen, pp.Str("customerPrice"))
	}
	return "", fmt.Errorf("asc: no price point for %.2f; available: %s", price, strings.Join(seen, " "))
}

func (l Live) IAPPricePoint(ctx context.Context, iapID, territory string, price float64) (string, error) {
	doc, err := l.C.Get(ctx, "/v2/inAppPurchases/"+iapID+"/pricePoints", url.Values{"filter[territory]": {territory}, "limit": {"200"}})
	if err != nil {
		return "", err
	}
	return pickPricePoint(doc, price)
}

func (l Live) CreateIAPPriceSchedule(ctx context.Context, iapID, baseTerritory, pricePointID string) error {
	// https://developer.apple.com/documentation/appstoreconnectapi/inapppurchasepriceschedulecreaterequest
	// manualPrices is an inline create: the price lives in `included` under a
	// temporary id and is referenced from the schedule's relationships.
	const tmp = "${price-1}"
	body := map[string]any{
		"data": map[string]any{
			"type": "inAppPurchasePriceSchedules",
			"relationships": map[string]any{
				"inAppPurchase": rel("inAppPurchases", iapID),
				"baseTerritory": rel("territories", baseTerritory),
				"manualPrices":  relMany("inAppPurchasePrices", []string{tmp}),
			},
		},
		"included": []map[string]any{{
			"type":       "inAppPurchasePrices",
			"id":         tmp,
			"attributes": map[string]any{"startDate": nil},
			"relationships": map[string]any{
				"inAppPurchaseV2":         rel("inAppPurchases", iapID),
				"inAppPurchasePricePoint": rel("inAppPurchasePricePoints", pricePointID),
			},
		}},
	}
	_, err := l.C.Post(ctx, "/v1/inAppPurchasePriceSchedules", body)
	return err
}

func (l Live) HasIAPAvailability(ctx context.Context, iapID string) (bool, error) {
	doc, err := l.C.Get(ctx, "/v2/inAppPurchases/"+iapID+"/inAppPurchaseAvailability", nil)
	if IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	_, err = doc.One()
	return err == nil, nil
}

func (l Live) CreateIAPAvailability(ctx context.Context, iapID string, territories []string) error {
	body := map[string]any{"data": map[string]any{
		"type":       "inAppPurchaseAvailabilities",
		"attributes": map[string]any{"availableInNewTerritories": true},
		"relationships": map[string]any{
			"inAppPurchase":        rel("inAppPurchases", iapID),
			"availableTerritories": relMany("territories", territories),
		},
	}}
	_, err := l.C.Post(ctx, "/v1/inAppPurchaseAvailabilities", body)
	return err
}

func (l Live) FindSubscriptionGroup(ctx context.Context, appID, referenceName string) (Resource, error) {
	doc, err := l.C.Get(ctx, "/v1/apps/"+appID+"/subscriptionGroups", url.Values{"limit": {"50"}})
	if err != nil {
		return Resource{}, err
	}
	return findByAttr(doc, "referenceName", referenceName)
}

func (l Live) CreateSubscriptionGroup(ctx context.Context, appID, referenceName string) (Resource, error) {
	body := map[string]any{"data": map[string]any{
		"type":          "subscriptionGroups",
		"attributes":    map[string]any{"referenceName": referenceName},
		"relationships": map[string]any{"app": rel("apps", appID)},
	}}
	doc, err := l.C.Post(ctx, "/v1/subscriptionGroups", body)
	if err != nil {
		return Resource{}, err
	}
	return doc.One()
}

func (l Live) HasGroupLocalization(ctx context.Context, groupID, locale string) (bool, error) {
	doc, err := l.C.Get(ctx, "/v1/subscriptionGroups/"+groupID+"/subscriptionGroupLocalizations", nil)
	if err != nil {
		return false, err
	}
	_, err = findByAttr(doc, "locale", locale)
	return err == nil, nil
}

func (l Live) CreateGroupLocalization(ctx context.Context, groupID, locale, name string) error {
	body := map[string]any{"data": map[string]any{
		"type":          "subscriptionGroupLocalizations",
		"attributes":    map[string]any{"locale": locale, "name": name},
		"relationships": map[string]any{"subscriptionGroup": rel("subscriptionGroups", groupID)},
	}}
	_, err := l.C.Post(ctx, "/v1/subscriptionGroupLocalizations", body)
	return err
}

func (l Live) FindSubscription(ctx context.Context, groupID, productID string) (Resource, error) {
	doc, err := l.C.Get(ctx, "/v1/subscriptionGroups/"+groupID+"/subscriptions", url.Values{"limit": {"50"}})
	if err != nil {
		return Resource{}, err
	}
	return findByAttr(doc, "productId", productID)
}

func (l Live) CreateSubscription(ctx context.Context, groupID, productID, name, period, reviewNote string) (Resource, error) {
	// https://developer.apple.com/documentation/appstoreconnectapi/subscriptioncreaterequest
	body := map[string]any{"data": map[string]any{
		"type": "subscriptions",
		"attributes": map[string]any{
			"name": name, "productId": productID, "subscriptionPeriod": period,
			"familySharable": true, "reviewNote": reviewNote, "groupLevel": 1,
		},
		"relationships": map[string]any{"group": rel("subscriptionGroups", groupID)},
	}}
	doc, err := l.C.Post(ctx, "/v1/subscriptions", body)
	if err != nil {
		return Resource{}, err
	}
	return doc.One()
}

func (l Live) HasSubscriptionLocalization(ctx context.Context, subID, locale string) (bool, error) {
	doc, err := l.C.Get(ctx, "/v1/subscriptions/"+subID+"/subscriptionLocalizations", nil)
	if err != nil {
		return false, err
	}
	_, err = findByAttr(doc, "locale", locale)
	return err == nil, nil
}

func (l Live) CreateSubscriptionLocalization(ctx context.Context, subID, locale, name, desc string) error {
	body := map[string]any{"data": map[string]any{
		"type":          "subscriptionLocalizations",
		"attributes":    map[string]any{"locale": locale, "name": name, "description": desc},
		"relationships": map[string]any{"subscription": rel("subscriptions", subID)},
	}}
	_, err := l.C.Post(ctx, "/v1/subscriptionLocalizations", body)
	return err
}

func (l Live) HasSubscriptionPrice(ctx context.Context, subID string) (bool, error) {
	doc, err := l.C.Get(ctx, "/v1/subscriptions/"+subID+"/prices", url.Values{"limit": {"1"}})
	if err != nil {
		return false, err
	}
	rs, err := doc.Many()
	return len(rs) > 0, err
}

func (l Live) SubscriptionPricePoint(ctx context.Context, subID, territory string, price float64) (string, error) {
	doc, err := l.C.Get(ctx, "/v1/subscriptions/"+subID+"/pricePoints", url.Values{"filter[territory]": {territory}, "limit": {"200"}})
	if err != nil {
		return "", err
	}
	return pickPricePoint(doc, price)
}

func (l Live) CreateSubscriptionPrice(ctx context.Context, subID, pricePointID string) error {
	// https://developer.apple.com/documentation/appstoreconnectapi/subscriptionpricecreaterequest
	body := map[string]any{"data": map[string]any{
		"type":       "subscriptionPrices",
		"attributes": map[string]any{"preserveCurrentPrice": false},
		"relationships": map[string]any{
			"subscription":           rel("subscriptions", subID),
			"subscriptionPricePoint": rel("subscriptionPricePoints", pricePointID),
		},
	}}
	_, err := l.C.Post(ctx, "/v1/subscriptionPrices", body)
	return err
}

func (l Live) HasSubscriptionAvailability(ctx context.Context, subID string) (bool, error) {
	doc, err := l.C.Get(ctx, "/v1/subscriptions/"+subID+"/subscriptionAvailability", nil)
	if IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	_, err = doc.One()
	return err == nil, nil
}

func (l Live) CreateSubscriptionAvailability(ctx context.Context, subID string, territories []string) error {
	body := map[string]any{"data": map[string]any{
		"type":       "subscriptionAvailabilities",
		"attributes": map[string]any{"availableInNewTerritories": true},
		"relationships": map[string]any{
			"subscription":         rel("subscriptions", subID),
			"availableTerritories": relMany("territories", territories),
		},
	}}
	_, err := l.C.Post(ctx, "/v1/subscriptionAvailabilities", body)
	return err
}
