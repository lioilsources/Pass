# Ol1n Pass

Jeden nákup, který odemkne všechny prémiové funkce ve všech appkách Ol1n na App Storu
i Google Play — včetně appek, které teprve vyjdou. Lifetime za 49,99 USD napevno, doplňkově
roční předplatné se stejným nárokem. Landing: [olin.now/pass](https://olin.now/pass).

Ani App Store ani Google Play produkt „vše od vývojáře navždy“ nemají. Nákup proto vždy
projde StoreKit / Play Billing jako běžný non-consumable (nebo předplatné) v dané appce
a sdílení nároku mezi appkami dělá vlastní vrstva: RevenueCat s jedním entitlementem
`all_apps` a malé Go API pro věci, které RevenueCat sám neumí (link code, notifikace).

## Jak to funguje

```
┌──────────────┐  purchase (StoreKit2 / Play Billing)   ┌──────────────┐
│  Appka A     │ ─────────────────────────────────────▶ │  RevenueCat  │
│  (Flutter)   │ ◀───────── entitlements[all_apps] ──── │  1 projekt   │
└──────────────┘                                        │  N apps      │
       │ app_user_id                                     └──────┬───────┘
       ▼                                                        │ webhook
 iOS: shared keychain (App Group)                        ┌──────▼───────┐
 Android: ContentProvider mezi appkami,                  │  pass-api    │ Go, JODA
          Google Sign-In uid nebo link code              │  + Discord   │ pass.ol1n.com
                                                         └──────────────┘
┌──────────────┐
│  Appka B     │  stejné app_user_id → stejné entitlements
└──────────────┘
```

Každá appka používá **stejné `app_user_id`** v jednom RevenueCat projektu. Na iOS si ho appky
předávají přes sdílenou klíčenku bez přihlašování; na Androidu přes `ContentProvider` chráněný
signature permission, případně přes Google účet nebo osmimístný kód z `pass-api`. Entitlement
`all_apps` je pak vidět všude, kde je ten uživatel identifikovaný. Detaily a všechny
identifikátory: [`docs/CONTRACTS.md`](docs/CONTRACTS.md). Původní plán:
[`Prompts/00-PASS_PLAN.md`](Prompts/00-PASS_PLAN.md).

## Co je v repu

| Část | Co to je |
|---|---|
| [`catalog/apps.json`](catalog/apps.json) | Zdroj pravdy: appky, hodnota jejich unlocků, od které verze mají Pass, store odkazy. Publikuje se na `olin.now/pass.json`. |
| [`ol1n_pass/`](ol1n_pass/) | Flutter balíček: `PassService` (RevenueCat), identita per platforma, paywall a badge widgety. |
| `cmd/pass-api` | Go server: `/v1/link-code`, `/v1/redeem`, `/v1/entitlements`, RevenueCat webhook → Discord/Telegram, zrcadlo katalogu. |
| `cmd/passctl` | Go CLI: kontrola katalogu, založení IAP v App Store Connect a Google Play z katalogu, kontrola RevenueCat. |
| [`site/`](site/) | Landing `olin.now/pass` (cs + en) — kopíruje se do repa `ol1n.now`. |
| [`docs/`](docs/) | Runbook ručních kroků, integrace do appky, promo účty, napojení na PromoClown/PromoHub. |

## Stav implementace (fáze z plánu §5)

| # | Krok | Stav |
|---|---|---|
| 1 | RevenueCat projekt, appky, entitlement `all_apps`, offering `pass`, webhook | ruční krok v konzoli — postup v [RUNBOOK §1](docs/RUNBOOK.md); `passctl rc check` to ověří |
| 2 | Store produkty lifetime + yearly v každé appce (ASC + Play) | `passctl asc/play products plan|apply` v repu; Paid Apps agreement, review screenshot a submit zůstávají ruční — [RUNBOOK §2–3](docs/RUNBOOK.md) |
| 3 | Balíček `ol1n_pass` | v repu: PassService, iOS keychain, Android provider, Google Sign-In, link code, paywall, testy |
| 4 | Pilot ve 2 appkách (jedna dětská, jedna ne) | čeká na kroky 1–2; checklist v [RUNBOOK §5](docs/RUNBOOK.md), integrace v [INTEGRATION](docs/INTEGRATION.md) |
| 5 | `olin.now/pass` landing + `pass.json` | v repu (`site/`), do `ol1n.now` přes `make sync-site`; deploy tam |
| 6 | Rollout do zbylých appek | per appka podle [INTEGRATION](docs/INTEGRATION.md) |
| 7 | Pravidlo pro nové appky | checklist v [RUNBOOK §6](docs/RUNBOOK.md) |

`pass-api` (plán fáze 3, „volitelné“) je v repu od začátku, protože link code bez serveru
nejde a RevenueCat webhook potřebuje příjemce; deploy na JODA je v [RUNBOOK §4](docs/RUNBOOK.md).

## Rychlý start pro appku

```yaml
dependencies:
  ol1n_pass:
    git: { url: https://github.com/lioilsources/Pass.git, ref: v0.1.0, path: ol1n_pass }
```

```dart
await PassService.instance.init(PassConfig(appSlug: 'kirian', revenueCatApiKeyIos: ..., revenueCatApiKeyAndroid: ...));
final unlocked = hasAppUnlock || PassService.instance.hasPass.value;
```

Celý postup včetně entitlementů, paywallu a zápisu do katalogu: [`docs/INTEGRATION.md`](docs/INTEGRATION.md)
a [`ol1n_pass/README.md`](ol1n_pass/README.md).

## Dokumenty

- [`docs/CONTRACTS.md`](docs/CONTRACTS.md) — identifikátory, API, schéma katalogu. Čti první.
- [`docs/RUNBOOK.md`](docs/RUNBOOK.md) — co udělat ručně v RevenueCat, App Store Connect, Google Play a na JODĚ; pilot.
- [`docs/INTEGRATION.md`](docs/INTEGRATION.md) — integrace balíčku do existující appky (Kirian, Lexify).
- [`docs/PROMO_ACCOUNTS.md`](docs/PROMO_ACCOUNTS.md) — guideline pro založení promo účtů portfolia.
- [`docs/PROMO_WIRING.md`](docs/PROMO_WIRING.md) — napojení na PromoClown, PromoHub, Ol1nDevBot a story.

## Vývoj

```bash
make test            # Go + Flutter testy
make build           # bin/pass-api, bin/passctl
make check-catalog   # validace catalog/apps.json
```

Konvence a invarianty pro práci v repu: [`CLAUDE.md`](CLAUDE.md).
