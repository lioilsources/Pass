// passctl is the operator CLI for Ol1n Pass (docs/CONTRACTS.md §5): it checks
// the catalog and creates the store and RevenueCat products that the catalog
// implies. It runs on the Mac with the keys from the environment; nothing here
// ever deletes or re-prices an existing product.
//
//	passctl catalog check|value [--catalog path] [--json]
//	passctl asc products plan|apply --app <slug> [--catalog path]
//	passctl play products plan|apply --app <slug> [--catalog path]
//	passctl rc check|products apply [--catalog path]
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/lioilsources/Pass/internal/asc"
	"github.com/lioilsources/Pass/internal/catalog"
	"github.com/lioilsources/Pass/internal/gplay"
	"github.com/lioilsources/Pass/internal/rcadmin"
)

const usage = `passctl — Ol1n Pass operator CLI

  passctl catalog check                      validate catalog/apps.json
  passctl catalog value                      total unlock value in USD
  passctl asc  products plan  --app <slug>   App Store Connect: what apply would create (no network)
  passctl asc  products apply --app <slug>   App Store Connect: create missing IAP + subscription
  passctl asc  iaps --app <slug>             App Store Connect: list existing IAPs and their US prices (read-only)
  passctl play products plan  --app <slug>   Google Play: what apply would create (no network)
  passctl play products apply --app <slug>   Google Play: create missing one-time product + subscription
  passctl rc   check                         RevenueCat: products ↔ entitlement all_apps ↔ offering pass
  passctl rc   products apply                RevenueCat: create missing products, attach to all_apps

Common flags: --catalog <path> (default catalog/apps.json), --json (catalog commands), --all (asc/play: every app).
Env: ASC_KEY_ID ASC_ISSUER_ID ASC_PRIVATE_KEY_PATH · GOOGLE_PLAY_SERVICE_ACCOUNT_JSON · RC_SECRET_API_KEY RC_PROJECT_ID
`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

type common struct {
	catalogPath string
	app         string
	all         bool
	jsonOut     bool
}

func parse(args []string) (verbs []string, c common, err error) {
	fs := flag.NewFlagSet("passctl", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&c.catalogPath, "catalog", "catalog/apps.json", "path to apps.json")
	fs.StringVar(&c.app, "app", "", "app slug")
	fs.BoolVar(&c.all, "all", false, "every app in the catalog")
	fs.BoolVar(&c.jsonOut, "json", false, "JSON output")
	// Allow flags after the verbs: split them out first.
	var flagArgs []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "-") {
			flagArgs = append(flagArgs, a)
			if !strings.Contains(a, "=") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") && a != "--all" && a != "-all" && a != "--json" && a != "-json" {
				flagArgs = append(flagArgs, args[i+1])
				i++
			}
			continue
		}
		verbs = append(verbs, a)
	}
	if err := fs.Parse(flagArgs); err != nil {
		return nil, c, err
	}
	return verbs, c, nil
}

func run(args []string) error {
	verbs, c, err := parse(args)
	if err != nil {
		return err
	}
	if len(verbs) == 0 || verbs[0] == "help" || verbs[0] == "-h" {
		fmt.Print(usage)
		return nil
	}
	cat, err := catalog.Load(c.catalogPath)
	if err != nil && !(verbs[0] == "catalog") {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	switch strings.Join(verbs, " ") {
	case "catalog check":
		if err != nil {
			return err
		}
		if c.jsonOut {
			return json.NewEncoder(os.Stdout).Encode(map[string]any{"ok": true, "apps": len(cat.Apps), "total_unlock_value_usd": cat.TotalUnlockValueUSD(), "total_listed_value_usd": cat.TotalListedValueUSD()})
		}
		fmt.Printf("ok: %d apps, unlock value today %.2f USD (listed %.2f USD), updated %s\n", len(cat.Apps), cat.TotalUnlockValueUSD(), cat.TotalListedValueUSD(), cat.Updated)
		for _, a := range cat.Apps {
			v := "   —  "
			if a.UnlockValueUSD != nil {
				v = fmt.Sprintf("%6.2f", *a.UnlockValueUSD)
			}
			since := "not yet"
			if a.PassSince != nil {
				since = "since " + *a.PassSince
			}
			fmt.Printf("  %-20s %s USD  %-12s %s\n", a.Slug, v, since, a.IOSBundleID)
		}
		return nil
	case "catalog value":
		if err != nil {
			return err
		}
		if c.jsonOut {
			return json.NewEncoder(os.Stdout).Encode(map[string]any{"total_unlock_value_usd": cat.TotalUnlockValueUSD(), "total_listed_value_usd": cat.TotalListedValueUSD()})
		}
		fmt.Printf("today %.2f USD, listed %.2f USD\n", cat.TotalUnlockValueUSD(), cat.TotalListedValueUSD())
		return nil

	case "asc products plan":
		return forApps(cat, c, func(a catalog.App) error { return asc.Plan(a, cat.Pass, os.Stdout) })
	case "asc products apply":
		cl, err := asc.NewFromEnv()
		if err != nil {
			return err
		}
		return forApps(cat, c, func(a catalog.App) error {
			fmt.Printf("== %s (%s)\n", a.Name, a.IOSBundleID)
			return asc.Provision(ctx, asc.Live{C: cl}, a, cat.Pass, os.Stdout)
		})

	case "asc iaps":
		cl, err := asc.NewFromEnv()
		if err != nil {
			return err
		}
		live := asc.Live{C: cl}
		return forApps(cat, c, func(a catalog.App) error {
			id, err := live.FindApp(ctx, a.IOSBundleID)
			if err != nil {
				fmt.Printf("== %s: %v\n", a.Slug, err)
				return nil
			}
			iaps, err := live.ListIAPs(ctx, id)
			if err != nil {
				return err
			}
			fmt.Printf("== %s: unlock value %.2f USD from %d IAPs\n", a.Slug, asc.UnlockValueUSD(iaps), len(iaps))
			for _, i := range iaps {
				price := "no USD price"
				if i.Priced {
					price = fmt.Sprintf("%.2f USD", i.PriceUSD)
				}
				fmt.Printf("  %-44s %-16s %-24s %s  %s\n", i.ProductID, i.Type, i.State, price, i.Name)
			}
			return nil
		})

	case "play products plan":
		return forApps(cat, c, func(a catalog.App) error { return gplay.Plan(a, cat.Pass, os.Stdout) })
	case "play products apply":
		cl, err := gplay.NewFromEnv(ctx)
		if err != nil {
			return err
		}
		return forApps(cat, c, func(a catalog.App) error {
			fmt.Printf("== %s (%s)\n", a.Name, a.AndroidPackage)
			return gplay.Provision(ctx, cl, a, cat.Pass, os.Stdout)
		})

	case "rc check":
		cl, err := rcadmin.NewFromEnv()
		if err != nil {
			return err
		}
		rep, err := rcadmin.Check(ctx, cl, cat, os.Stdout)
		if err != nil {
			return err
		}
		if !rep.OK {
			return fmt.Errorf("RevenueCat configuration incomplete: %d apps missing, %d products missing, %d unattached, %d not in offering",
				len(rep.MissingApps), len(rep.MissingProducts), len(rep.Unattached), len(rep.NotInOffering))
		}
		fmt.Println("ok: every Pass product exists, is attached to all_apps and is in offering pass")
		return nil
	case "rc products apply":
		cl, err := rcadmin.NewFromEnv()
		if err != nil {
			return err
		}
		return rcadmin.Apply(ctx, cl, cat, os.Stdout)
	}
	fmt.Print(usage)
	return fmt.Errorf("unknown command %q", strings.Join(verbs, " "))
}

func forApps(cat *catalog.Catalog, c common, fn func(catalog.App) error) error {
	switch {
	case c.all:
		for _, a := range cat.Apps {
			if err := fn(a); err != nil {
				return fmt.Errorf("%s: %w", a.Slug, err)
			}
		}
		return nil
	case c.app != "":
		a := cat.Find(c.app)
		if a == nil {
			return fmt.Errorf("unknown app %q (slugs: %s)", c.app, slugs(cat))
		}
		return fn(*a)
	default:
		return fmt.Errorf("--app <slug> or --all is required (slugs: %s)", slugs(cat))
	}
}

func slugs(cat *catalog.Catalog) string {
	var s []string
	for _, a := range cat.Apps {
		s = append(s, a.Slug)
	}
	return strings.Join(s, ", ")
}
