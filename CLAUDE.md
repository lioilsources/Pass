# Pass

<!-- Max 80 řádků. Co se nevejde, jde do docs/ a sem jeden odkaz. Veřejné repo: žádné LAN IP, domovské cesty, ID. -->

## Účel
Ol1n Pass: jeden nákup (lifetime 49 USD, volitelně roční), který odemkne všechny jednorázové
unlocky ve všech appkách Ol1n na App Storu i Google Play, včetně budoucích. Nákup jde vždy přes
StoreKit / Play Billing; sdílení nároku mezi appkami dělá RevenueCat (entitlement `all_apps`)
a malé Go API. Implementuje `Prompts/00-PASS_PLAN.md`; rozhraní a identifikátory jsou v
`docs/CONTRACTS.md` — to čti první, když měníš cokoli, co přechází hranici balíček ↔ server ↔ store.

## Build a test
```
make test              # go test ./... + (cd ol1n_pass && flutter test)
make build             # bin/pass-api, bin/passctl (CGO_ENABLED=0)
make check-catalog     # passctl catalog check — schéma, unikátní slugy, součet hodnoty
make sync-catalog      # catalog/apps.json → ol1n_pass/assets/ a internal/catalog/ (kopie, test to hlídá)
make sync-site         # site/ + catalog → ../ol1n.now (pages/pass*.html, assets/css/pass.css, static/pass.json)
make deploy-pass-api   # image → ghcr, `ssh joda` compose pull + up
```

## Mapa
```
catalog/apps.json      jediný zdroj pravdy: appky, hodnoty unlocků, pass_since, store odkazy
ol1n_pass/             Flutter balíček: PassService, identita (keychain / ContentProvider / Google), paywall
cmd/pass-api           Go server: link code, entitlements, RC webhook → Discord/Telegram, /pass/apps.json
cmd/passctl            Go CLI: katalog, ASC + Play IAP plan/apply, RevenueCat kontrola
internal/              sdílený Go kód (catalog, revenuecat, asc, play, linkcode, notify, …)
deploy/                compose + Caddy snippet pro JODA
site/                  zdroj landing page olin.now/pass (cs + en) a jejího CSS
docs/                  CONTRACTS, RUNBOOK (ruční kroky v konzolích), INTEGRATION, PROMO_ACCOUNTS, PROMO_WIRING
Prompts/               původní plán
```

## Invarianty
- Odemykání vychází jen ze store transakce. Link code nikdy nevyrobí nárok: jen přepne klienta na
  `app_user_id`, který už v RevenueCat aktivní `all_apps` má, a server to před vydáním kódu ověří.
  Žádné licenční klíče mimo store (Apple 3.1.1).
- `catalog/apps.json` je jediný zdroj pravdy. `ol1n_pass/assets/apps.json` a
  `internal/catalog/default_apps.json` jsou kopie z `make sync-catalog`; test selže, když se liší.
- Product ID se odvozují ze `slug` (CONTRACTS §2). Nikdy je nepiš ručně do kódu ani do konzolí.
- `passctl … apply` je idempotentní: čte, vytváří chybějící, nikdy nemaže a nemění ceny existujících.
- V repu není žádný klíč, Team ID, service account ani webhook URL; všechno jde z env / Bitwardenu
  (`Distribution/SKILL.md`). Repo je veřejné.
- `pass-api` nikdy neodpoví „má Pass“ bez čerstvého ověření v RevenueCat (cache max 60 s).
- Dětské appky (`kids: true`): žádný login bez rodičovské brány; balíček bránu nevnucuje, appka ji dodá.

## Odchylky
- Katalog je na `https://olin.now/pass.json`, ne `/pass/apps.json` z plánu: adresář `pass/` by se
  na GitHub Pages tloukl se stránkou `/pass` (viz CONTRACTS §3). `pass-api` ho zrcadlí na `/pass/apps.json`.
- `pass-api` poslouchá na 8096 (další volný port v registru JODA), ne na portu z plánu, který žádný neurčil.
- Yearly je v repu připravené (produkty, package, paywall), ale zapnutí je rozhodnutí z plánu §6.

## Kam dál
`docs/RUNBOOK.md` (RevenueCat, ASC, Play, JODA, pilot), `docs/INTEGRATION.md` (appka krok za krokem),
`docs/PROMO_ACCOUNTS.md` (promo účty portfolia), `docs/PROMO_WIRING.md` (PromoClown/PromoHub/story),
`ol1n_pass/README.md`, `Prompts/00-PASS_PLAN.md`.
