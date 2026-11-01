package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// IAP is one existing in-app purchase of an app with its current US price.
type IAP struct {
	ProductID string  `json:"product_id"`
	Name      string  `json:"name"`
	Type      string  `json:"type"`
	State     string  `json:"state"`
	PriceUSD  float64 `json:"price_usd"`
	Priced    bool    `json:"priced"`
}

// Counts reports whether the IAP belongs in unlock_value_usd: an approved
// one-time unlock that is not the Pass itself. Consumables stay out because the
// Pass never unlocks them, and unapproved IAPs because nobody can buy them, so
// advertising their price would overstate the value.
func (i IAP) Counts() bool {
	return i.Type == "NON_CONSUMABLE" && i.State == "APPROVED" && !strings.HasPrefix(i.ProductID, "com.ol1n.pass.")
}

// UnlockValueUSD sums the priced IAPs that count.
func UnlockValueUSD(iaps []IAP) float64 {
	var sum float64
	for _, i := range iaps {
		if i.Counts() && i.Priced {
			sum += i.PriceUSD
		}
	}
	return sum
}

// ListIAPs reads the in-app purchases of an app. It only ever issues GETs.
func (l Live) ListIAPs(ctx context.Context, appID string) ([]IAP, error) {
	doc, err := l.C.Get(ctx, "/v1/apps/"+appID+"/inAppPurchasesV2", url.Values{"limit": {"200"}})
	if err != nil {
		return nil, err
	}
	rs, err := doc.Many()
	if err != nil {
		return nil, err
	}
	out := make([]IAP, 0, len(rs))
	for _, r := range rs {
		iap := IAP{
			ProductID: r.Str("productId"),
			Name:      r.Str("name"),
			Type:      r.Str("inAppPurchaseType"),
			State:     r.Str("state"),
		}
		price, ok, err := l.usdPrice(ctx, r.ID)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", iap.ProductID, err)
		}
		iap.PriceUSD, iap.Priced = price, ok
		out = append(out, iap)
	}
	return out, nil
}

// usdPrice returns the US customer price valid now. The US entry is manual when
// USA is the base territory and automatic otherwise, so both lists are read.
func (l Live) usdPrice(ctx context.Context, iapID string) (float64, bool, error) {
	sched, err := l.C.Get(ctx, "/v2/inAppPurchases/"+iapID+"/iapPriceSchedule", nil)
	if err != nil {
		if IsNotFound(err) {
			return 0, false, nil
		}
		return 0, false, err
	}
	s, err := sched.One()
	if err != nil {
		return 0, false, err
	}
	for _, list := range []string{"manualPrices", "automaticPrices"} {
		doc, err := l.C.Get(ctx, "/v1/inAppPurchasePriceSchedules/"+s.ID+"/"+list, url.Values{
			"filter[territory]": {"USA"},
			"include":           {"inAppPurchasePricePoint"},
			"limit":             {"50"},
		})
		if err != nil {
			return 0, false, err
		}
		if p, ok, err := currentPrice(doc); ok || err != nil {
			return p, ok, err
		}
	}
	return 0, false, nil
}

// currentPrice picks the open-ended entry of a price list and resolves its
// price point from the included resources.
func currentPrice(doc *Document) (float64, bool, error) {
	entries, err := doc.Many()
	if err != nil {
		return 0, false, err
	}
	for _, e := range entries {
		if e.Str("endDate") != "" {
			continue
		}
		var rel struct {
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(e.Relationships["inAppPurchasePricePoint"], &rel); err != nil {
			return 0, false, err
		}
		for _, inc := range doc.Included {
			if inc.ID != rel.Data.ID {
				continue
			}
			p, err := strconv.ParseFloat(inc.Str("customerPrice"), 64)
			if err != nil {
				return 0, false, fmt.Errorf("asc: customerPrice %q: %w", inc.Str("customerPrice"), err)
			}
			return p, true, nil
		}
	}
	return 0, false, nil
}
