package gplay

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/lioilsources/Pass/internal/catalog"
	"google.golang.org/api/androidpublisher/v3"
)

func TestMoneyAndMicros(t *testing.T) {
	if Micros(49) != "49000000" || Micros(14.99) != "14990000" {
		t.Fatal(Micros(49), Micros(14.99))
	}
	m := Money(14.99, "USD")
	if m.Units != 14 || m.Nanos != 990_000_000 || m.CurrencyCode != "USD" {
		t.Fatalf("%+v", m)
	}
}

func TestPlanDeterministic(t *testing.T) {
	cat, _ := catalog.Default()
	app := *cat.Find("kirian")
	var a, b bytes.Buffer
	if err := Plan(app, cat.Pass, &a); err != nil {
		t.Fatal(err)
	}
	Plan(app, cat.Pass, &b)
	if a.String() != b.String() {
		t.Fatal("non-deterministic")
	}
	out := a.String()
	for _, want := range []string{
		"inappproducts.insert com.ab.kiran/pass_lifetime {managedUser, active, USD 49000000 micros",
		"monetization.subscriptions.create com.ab.kiran/pass_yearly {basePlan yearly P1Y, US 14.99 USD, other regions 14.99 USD / 13.79 EUR",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
}

type fake struct {
	inapps map[string]*androidpublisher.InAppProduct
	subs   map[string]*androidpublisher.Subscription
	writes int
}

func (f *fake) GetInApp(_ context.Context, pkg, sku string) (*androidpublisher.InAppProduct, error) {
	if p, ok := f.inapps[pkg+"/"+sku]; ok {
		return p, nil
	}
	return nil, errNotFound
}
func (f *fake) InsertInApp(_ context.Context, pkg string, p *androidpublisher.InAppProduct) error {
	f.inapps[pkg+"/"+p.Sku] = p
	f.writes++
	return nil
}
func (f *fake) GetSubscription(_ context.Context, pkg, id string) (*androidpublisher.Subscription, error) {
	if s, ok := f.subs[pkg+"/"+id]; ok {
		return s, nil
	}
	return nil, errNotFound
}
func (f *fake) CreateSubscription(_ context.Context, pkg, id string, s *androidpublisher.Subscription) error {
	f.subs[pkg+"/"+id] = s
	f.writes++
	return nil
}

func TestProvisionIdempotent(t *testing.T) {
	cat, _ := catalog.Default()
	app := *cat.Find("lexify")
	f := &fake{inapps: map[string]*androidpublisher.InAppProduct{}, subs: map[string]*androidpublisher.Subscription{}}
	var out bytes.Buffer
	if err := Provision(context.Background(), f, app, cat.Pass, &out); err != nil {
		t.Fatal(err)
	}
	if f.writes != 2 {
		t.Fatalf("writes %d", f.writes)
	}
	if err := Provision(context.Background(), f, app, cat.Pass, &out); err != nil {
		t.Fatal(err)
	}
	if f.writes != 2 {
		t.Fatalf("second run wrote: %d", f.writes)
	}
	if !strings.Contains(out.String(), "subscription pass_yearly exists in com.ol1n.duolingoCards") {
		t.Fatal(out.String())
	}
}
