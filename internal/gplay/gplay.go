// Package gplay creates the Ol1n Pass products on Google Play
// (docs/CONTRACTS.md §2, §5) through the official Android Publisher v3 client:
// the one-time product `pass_lifetime` and the subscription `pass_yearly` with
// base plan `yearly`. Like asc, the provisioner talks to an API interface so a
// plan can run without network.
package gplay

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"

	"github.com/lioilsources/Pass/internal/catalog"
	"google.golang.org/api/androidpublisher/v3"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

// Text shared by the products.
const (
	Language     = "en-US"
	LifetimeName = "Ol1n Pass"
	YearlyName   = "Ol1n Pass Yearly"
	LifetimeDesc = "Unlocks all premium features in every app by Ol1n, including apps released later. Current list: olin.now/pass"
	YearlyDesc   = "One year of all premium features in every app by Ol1n, including apps released later. Current list: olin.now/pass"
	// Google asks for an explicit EUR price for the "other regions" bucket; we
	// derive it from USD at a fixed, rounded rate so the plan stays deterministic.
	// Adjust in Play Console if the rate drifts a lot.
	eurPerUSD = 0.92
)

var errNotFound = errors.New("gplay: not found")

// IsNotFound reports a missing product.
func IsNotFound(err error) bool {
	if errors.Is(err, errNotFound) {
		return true
	}
	var ge *googleapi.Error
	return errors.As(err, &ge) && ge.Code == 404
}

// API is what the provisioner needs.
type API interface {
	GetInApp(ctx context.Context, pkg, sku string) (*androidpublisher.InAppProduct, error)
	InsertInApp(ctx context.Context, pkg string, p *androidpublisher.InAppProduct) error
	GetSubscription(ctx context.Context, pkg, productID string) (*androidpublisher.Subscription, error)
	CreateSubscription(ctx context.Context, pkg, productID string, s *androidpublisher.Subscription) error
}

// Micros converts a USD amount to Play's priceMicros string.
func Micros(usd float64) string { return fmt.Sprintf("%d", int64(math.Round(usd*1_000_000))) }

// Money converts an amount to the Money type (units + nanos).
func Money(amount float64, currency string) *androidpublisher.Money {
	units := math.Floor(amount)
	nanos := int64(math.Round((amount - units) * 1e9))
	return &androidpublisher.Money{CurrencyCode: currency, Units: int64(units), Nanos: nanos}
}

// LifetimeProduct builds the one-time product.
func LifetimeProduct(pkg string, pass catalog.Pass) *androidpublisher.InAppProduct {
	return &androidpublisher.InAppProduct{
		PackageName:     pkg,
		Sku:             catalog.AndroidLifetimeSKU,
		PurchaseType:    "managedUser",
		Status:          "active",
		DefaultLanguage: Language,
		DefaultPrice:    &androidpublisher.Price{Currency: "USD", PriceMicros: Micros(pass.LifetimeUSD)},
		Listings: map[string]androidpublisher.InAppProductListing{
			Language: {Title: LifetimeName, Description: LifetimeDesc},
		},
	}
}

// YearlySubscription builds the subscription with its single base plan.
func YearlySubscription(pkg string, pass catalog.Pass) *androidpublisher.Subscription {
	eur := math.Round(pass.YearlyUSD*eurPerUSD*100) / 100
	return &androidpublisher.Subscription{
		PackageName: pkg,
		ProductId:   catalog.AndroidYearlySKU,
		Listings: []*androidpublisher.SubscriptionListing{
			{LanguageCode: Language, Title: YearlyName, Description: YearlyDesc},
		},
		BasePlans: []*androidpublisher.BasePlan{{
			BasePlanId: pass.AndroidYearlyBasePlan,
			AutoRenewingBasePlanType: &androidpublisher.AutoRenewingBasePlanType{
				BillingPeriodDuration: "P1Y",
				GracePeriodDuration:   "P30D",
				ResubscribeState:      "RESUBSCRIBE_STATE_ACTIVE",
				ProrationMode:         "SUBSCRIPTION_PRORATION_MODE_CHARGE_ON_NEXT_BILLING_DATE",
				LegacyCompatible:      true,
			},
			RegionalConfigs: []*androidpublisher.RegionalBasePlanConfig{
				{RegionCode: "US", NewSubscriberAvailability: true, Price: Money(pass.YearlyUSD, "USD")},
			},
			OtherRegionsConfig: &androidpublisher.OtherRegionsBasePlanConfig{
				NewSubscriberAvailability: true,
				UsdPrice:                  Money(pass.YearlyUSD, "USD"),
				EurPrice:                  Money(eur, "EUR"),
			},
		}},
	}
}

// Provision creates whatever is missing for one app's package.
func Provision(ctx context.Context, api API, app catalog.App, pass catalog.Pass, w io.Writer) error {
	pkg := app.AndroidPackage
	say := func(format string, args ...any) { fmt.Fprintf(w, format+"\n", args...) }

	if _, err := api.GetInApp(ctx, pkg, catalog.AndroidLifetimeSKU); IsNotFound(err) {
		if err := api.InsertInApp(ctx, pkg, LifetimeProduct(pkg, pass)); err != nil {
			return fmt.Errorf("insert %s: %w", catalog.AndroidLifetimeSKU, err)
		}
		say("created one-time product %s in %s (%.2f USD, auto-converted to other currencies)", catalog.AndroidLifetimeSKU, pkg, pass.LifetimeUSD)
	} else if err != nil {
		return fmt.Errorf("get %s: %w", catalog.AndroidLifetimeSKU, err)
	} else {
		say("one-time product %s exists in %s", catalog.AndroidLifetimeSKU, pkg)
	}

	if _, err := api.GetSubscription(ctx, pkg, catalog.AndroidYearlySKU); IsNotFound(err) {
		if err := api.CreateSubscription(ctx, pkg, catalog.AndroidYearlySKU, YearlySubscription(pkg, pass)); err != nil {
			return fmt.Errorf("create %s: %w", catalog.AndroidYearlySKU, err)
		}
		say("created subscription %s with base plan %s in %s (%.2f USD / year)", catalog.AndroidYearlySKU, pass.AndroidYearlyBasePlan, pkg, pass.YearlyUSD)
		say("manual: the base plan is created in DRAFT; activate it in Play Console → Monetize → Subscriptions (the API has basePlans.activate, run it once the listing is reviewed).")
	} else if err != nil {
		return fmt.Errorf("get %s: %w", catalog.AndroidYearlySKU, err)
	} else {
		say("subscription %s exists in %s", catalog.AndroidYearlySKU, pkg)
	}
	say("manual: RevenueCat needs the Play service account with 'View financial data' + 'Manage orders' on this app; see docs/RUNBOOK.md.")
	return nil
}

// ---------------------------------------------------------------------------

// Live calls the real API.
type Live struct{ S *androidpublisher.Service }

// NewFromEnv reads GOOGLE_PLAY_SERVICE_ACCOUNT_JSON (a path, same name as
// PromoClown) and builds the client.
func NewFromEnv(ctx context.Context) (*Live, error) {
	path := os.Getenv("GOOGLE_PLAY_SERVICE_ACCOUNT_JSON")
	if path == "" {
		return nil, errors.New("gplay: GOOGLE_PLAY_SERVICE_ACCOUNT_JSON (path to the service account key) is required")
	}
	svc, err := androidpublisher.NewService(ctx, option.WithCredentialsFile(path), option.WithScopes(androidpublisher.AndroidpublisherScope))
	if err != nil {
		return nil, fmt.Errorf("gplay: %w", err)
	}
	return &Live{S: svc}, nil
}

func (l *Live) GetInApp(ctx context.Context, pkg, sku string) (*androidpublisher.InAppProduct, error) {
	return l.S.Inappproducts.Get(pkg, sku).Context(ctx).Do()
}

func (l *Live) InsertInApp(ctx context.Context, pkg string, p *androidpublisher.InAppProduct) error {
	_, err := l.S.Inappproducts.Insert(pkg, p).AutoConvertMissingPrices(true).Context(ctx).Do()
	return err
}

func (l *Live) GetSubscription(ctx context.Context, pkg, productID string) (*androidpublisher.Subscription, error) {
	return l.S.Monetization.Subscriptions.Get(pkg, productID).Context(ctx).Do()
}

func (l *Live) CreateSubscription(ctx context.Context, pkg, productID string, s *androidpublisher.Subscription) error {
	// regionsVersion "2022/02" is the value the v3 API documents for new
	// subscriptions; a newer version string is accepted as well.
	_, err := l.S.Monetization.Subscriptions.Create(pkg, s).ProductId(productID).RegionsVersionVersion("2022/02").Context(ctx).Do()
	return err
}

// ---------------------------------------------------------------------------

// Planner records what apply would do.
type Planner struct{ Steps []string }

func (p *Planner) GetInApp(_ context.Context, pkg, sku string) (*androidpublisher.InAppProduct, error) {
	p.Steps = append(p.Steps, fmt.Sprintf("inappproducts.get %s/%s", pkg, sku))
	return nil, errNotFound
}

func (p *Planner) InsertInApp(_ context.Context, pkg string, ip *androidpublisher.InAppProduct) error {
	p.Steps = append(p.Steps, fmt.Sprintf("inappproducts.insert %s/%s {%s, %s, %s %s micros, listing %q}",
		pkg, ip.Sku, ip.PurchaseType, ip.Status, ip.DefaultPrice.Currency, ip.DefaultPrice.PriceMicros, ip.Listings[Language].Title))
	return nil
}

func (p *Planner) GetSubscription(_ context.Context, pkg, id string) (*androidpublisher.Subscription, error) {
	p.Steps = append(p.Steps, fmt.Sprintf("monetization.subscriptions.get %s/%s", pkg, id))
	return nil, errNotFound
}

func (p *Planner) CreateSubscription(_ context.Context, pkg, id string, s *androidpublisher.Subscription) error {
	bp := s.BasePlans[0]
	p.Steps = append(p.Steps, fmt.Sprintf("monetization.subscriptions.create %s/%s {basePlan %s %s, US %d.%02d USD, other regions %d.%02d USD / %d.%02d EUR, listing %q}",
		pkg, id, bp.BasePlanId, bp.AutoRenewingBasePlanType.BillingPeriodDuration,
		bp.RegionalConfigs[0].Price.Units, bp.RegionalConfigs[0].Price.Nanos/10_000_000,
		bp.OtherRegionsConfig.UsdPrice.Units, bp.OtherRegionsConfig.UsdPrice.Nanos/10_000_000,
		bp.OtherRegionsConfig.EurPrice.Units, bp.OtherRegionsConfig.EurPrice.Nanos/10_000_000,
		s.Listings[0].Title))
	return nil
}

// Plan prints the numbered steps for one app.
func Plan(app catalog.App, pass catalog.Pass, w io.Writer) error {
	p := &Planner{}
	if err := Provision(context.Background(), p, app, pass, io.Discard); err != nil {
		return err
	}
	fmt.Fprintf(w, "Google Play plan for %s (%s):\n", app.Name, app.AndroidPackage)
	for i, s := range p.Steps {
		fmt.Fprintf(w, "%2d. %s\n", i+1, s)
	}
	return nil
}
