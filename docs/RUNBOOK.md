# RUNBOOK — Ol1n Pass: co udělat ručně v konzolích a na JODĚ

Co musí udělat člověk, aby fungoval tok **appka → StoreKit/Play Billing → RevenueCat →
všechny appky**, a k tomu **RevenueCat webhook → pass-api → Discord**. Kód nic z toho nezakládá
sám kromě store produktů (`passctl`). Identifikátory a názvy jsou v [`CONTRACTS.md`](CONTRACTS.md);
tady se jen vyplňují do formulářů.

Klíče a jejich zdroje (Bitwarden, `Distribution/`) popisuje `Distribution/SKILL.md` v privátním repu
Distribution. Sem patří jen názvy env proměnných:

| Env | Co to je | Kdo ho používá |
|---|---|---|
| `ASC_KEY_ID`, `ASC_ISSUER_ID`, `ASC_PRIVATE_KEY_PATH` | App Store Connect API klíč (stejné názvy jako PromoClown `store-reviews`) | `passctl asc` |
| `GOOGLE_PLAY_SERVICE_ACCOUNT_JSON` | cesta k JSON service accountu s právem *Manage store presence → in-app products* | `passctl play` |
| `RC_SECRET_API_KEY` | RevenueCat **secret** API key (v1 i v2 endpointy) | `passctl rc`, `pass-api` |
| `RC_PROJECT_ID` | ID projektu `Ol1n` v RevenueCat (v URL dashboardu) | `passctl rc` |
| `RC_WEBHOOK_SECRET` | náhodný řetězec, `openssl rand -hex 24` | RevenueCat webhook ↔ `pass-api` |
| `DISCORD_WEBHOOK_URL`, `TELEGRAM_BOT_TOKEN`, `TELEGRAM_CHAT_ID` | kam hlásit nákupy | `pass-api` |

---

## 1. RevenueCat

### 1.1 Projekt a appky
1. <https://app.revenuecat.com> → **Create project** → název `Ol1n`. Jeden projekt pro všechny appky;
   cross-app entitlement funguje jen uvnitř jednoho projektu.
2. Pro každou appku z `catalog/apps.json` přidej **dvě** app položky:
   - **App Store** → Bundle ID (`ios_bundle_id`), **In-app purchase key** (ASC → Users and Access →
     Integrations → In-App Purchase → vygeneruj klíč „RevenueCat“ a nahraj `.p8`) a **App Store Connect
     API key** pro server-to-server (RC si z něj sám stáhne produkty a hlídá Family Sharing).
   - **Play Store** → Package (`android_package`), **Service account credentials JSON**: v Google Cloud
     projektu, který je propojený s Play Console, založ service account, v Play Console → Users and
     permissions mu dej *View financial data* + *Manage orders and subscriptions*; JSON nahraj do RC.
     Pub/Sub pro RTDN je volitelné (RC si vystačí s pollingem), ale zrychlí refundy.
3. Z každé app položky si zkopíruj **public SDK key** (`appl_…` / `goog_…`). Ty jdou do appky přes
   `--dart-define` (viz [INTEGRATION §3](INTEGRATION.md)), nikdy do repa.

### 1.2 Entitlement, produkty, offering
1. **Entitlements** → `+ New` → identifier `all_apps`. Jediný entitlement v projektu.
2. **Products**: pro každou appku přidej produkty podle CONTRACTS §2 (`com.ol1n.pass.lifetime.<slug>`,
   `com.ol1n.pass.yearly.<slug>` pro App Store; `pass_lifetime`, `pass_yearly:yearly` pro Play).
   Rychleji: nejdřív je založ ve storech (§2, §3) a pak `passctl rc products apply` — založí chybějící
   produkty v RC a přiřadí je k `all_apps`. `passctl rc check` ukáže, co nesedí.
3. **Offerings** → `+ New` → identifier `pass`, nastav jako **current**. Packages: `$rc_lifetime` →
   všechny lifetime produkty (jeden per app položku), `$rc_annual` → všechny yearly produkty.
   RC si při `getOfferings()` vybere produkt pro tu appku, která se ptá.
4. **Project settings → General → Restore behavior**: **Transfer to new App User ID**. Bez toho restore
   po reinstalu (nové UUID v klíčence smazané appky) nárok nepřenese.

### 1.3 Webhook a klíče
1. **Integrations → Webhooks → + New**: URL `https://pass.ol1n.com/webhooks/revenuecat`,
   **Authorization header** = hodnota `RC_WEBHOOK_SECRET` (RC ji posílá doslova v hlavičce
   `Authorization`; `pass-api` očekává `Bearer <secret>` i holou hodnotu — ověř v `.env.example`),
   Environment: Production i Sandbox (sandbox je pro pilot), events: všechny.
2. **API keys**: vytvoř **Secret API key v2** s právy *read* na projects/products/entitlements/offerings
   a *write* na products/entitlements (pro `passctl rc products apply`) a *read* na customers
   (pro `pass-api` `/v1/entitlements` a `/v1/link-code`). Ulož do Bitwardenu `Pass / RC_SECRET_API_KEY`.
3. Ověření: Webhooks → Send test event → v `docker logs pass-api` řádek s `event=TEST`.

---

## 2. App Store Connect

### 2.1 Jednou za účet
- **Agreements, Tax, and Banking → Paid Apps**: aktivní smlouva a vyplněné bankovní/daňové údaje.
  Bez ní se IAP nedají odeslat do review (stav „Missing Metadata“ navždy).
- **Keychain Sharing**: na developer portálu se **nic nezapíná**. `keychain-access-groups` je
  entitlement, který Xcode zapíše do `Runner.entitlements` a do profilu sám při zapnutí
  Signing & Capabilities → Keychain Sharing; App ID capability pro to neexistuje. Nutné je jen, aby
  všechny appky měly stejný Team ID (mají) — `$(AppIdentifierPrefix)` je pak stejný prefix pro
  všechny. Postup per appka je v [INTEGRATION §2](INTEGRATION.md).

### 2.2 Per appka (`passctl`)
```bash
export ASC_KEY_ID=… ASC_ISSUER_ID=… ASC_PRIVATE_KEY_PATH=…
passctl asc products plan  --app kirian     # vypíše, co by vzniklo; bez sítě
passctl asc products apply --app kirian     # založí chybějící
```
Co `apply` udělá: non-consumable `com.ol1n.pass.lifetime.<slug>` (reference name „Ol1n Pass“,
Family Sharing **zapnuté**, review note z CONTRACTS §2), lokalizaci `en-US` (display name
„Ol1n Pass“, description), price schedule s base territory USA a price pointem nejblíž 49 USD;
subscription group `ol1n_pass` a v ní `com.ol1n.pass.yearly.<slug>` (P1Y, lokalizace, cena nejblíž
14.99 USD). Druhé spuštění nic nemění.

### 2.3 Co zůstává ruční
1. **Review screenshot** u každého IAP: screenshot paywallu se seznamem appek (plán §2.1). Nahraj
   v ASC → appka → In-App Purchases → produkt → Review Information.
2. **Submit IAP s první verzí appky**, která `ol1n_pass` má: první IAP appky musí jít do review
   spolu s binárkou (In-App Purchases and Subscriptions sekce ve verzi). Další už jdou samostatně.
3. **App Review notes** verze: text z CONTRACTS §2 + sandbox test account (Users and Access →
   Sandbox → Testers; pro Pass jeden sdílený tester, aby reviewer viděl cross-app efekt).
4. Ověř **Family Sharing** toggle u lifetime (apply ho nastaví; v UI zkontroluj, po zapnutí se nedá
   vypnout).
5. Pokud se roční varianta zatím nespouští (plán §6), nech subscription ve stavu *Missing Metadata*
   a do verze ji nezařazuj — RC offering `pass` pak `$rc_annual` pro tu appku prostě nemá a paywall
   kartu skryje.

---

## 3. Google Play

### 3.1 Jednou za účet
- Play Console → **Monetization setup**: Play Billing aktivní, obchodní profil a platební údaje
  hotové (jinak `inappproducts.insert` vrací 403 „not eligible“).
- Service account pro `passctl` (může být stejný jako pro RevenueCat): Play Console → Users and
  permissions → Invite → service account e-mail → *Manage store presence → View app information,
  Manage in-app products*; pro subscriptions navíc *Manage orders and subscriptions*.
- **License testing**: Setup → License testing → tvoje Google účty, response „RESPOND_NORMALLY“.
  Nákupy těchto účtů jsou zdarma a RC je vidí jako sandbox.

### 3.2 Per appka
```bash
export GOOGLE_PLAY_SERVICE_ACCOUNT_JSON=…
passctl play products plan  --app kirian
passctl play products apply --app kirian
```
Co `apply` udělá: one-time product `pass_lifetime` (managed, `en-US` title „Ol1n Pass“, default
price 49 USD, status active) a subscription `pass_yearly` s base plan `yearly` (P1Y, auto-renewing,
cena USD + EUR pro ostatní regiony). Produkt musí mít appka aspoň v **internal testing** nahranou
APK/AAB s Billing permission, jinak Play Console produkt odmítne aktivovat — pořadí: první build
s `ol1n_pass` → internal track → `passctl play products apply`.

### 3.3 Ruční
- **Family Library** u `pass_lifetime`: Play Console → produkt → Family Library (rozhodnutí z plánu
  §6; jednou zapnuté se nedá vypnout).
- Subscriptions: zkontroluj v Console, že base plan je **active** (API ho vytvoří jako draft).

---

## 4. JODA: nasazení `pass-api`

1. **DNS a tunel**: Cloudflare Zero Trust → Tunnels → tunel JODA → public hostname `pass.ol1n.com` →
   `http://caddy:80` (stejně jako `promo.ol1n.com` a `hooks.ol1n.com`). Bez Cloudflare Access —
   endpointy volají appky anonymně.
2. **Caddy**: vlož `deploy/Caddyfile.snippet` do Caddyfile na JODĚ a
   `docker compose exec caddy caddy reload --config /etc/caddy/Caddyfile`. Ven jdou `/healthz`,
   `/pass/apps.json`, `/v1/*`, `/webhooks/*`.
3. **Compose**: `mkdir -p ~/deploy/pass-api`, zkopíruj `deploy/docker-compose.yml`, vytvoř `.env`
   podle `.env.example` (`chmod 600`), DB je SQLite v `~/deploy/pass-api/data/pass.db` (bind mount —
   zálohuj spolu s ostatními `~/deploy/*`). Image staví a pushuje
   `make deploy-pass-api` z Macu (`ghcr.io/lioilsources/pass-api`, linux/amd64).
4. **Discord**: na serveru Ol1n (kategorie, které spravuje Ol1nDevBot) založ kanál `#pass` →
   Integrations → Webhooks → New → URL do `DISCORD_WEBHOOK_URL`. Telegram volitelně (bot token +
   chat ID tvého privátního chatu; musí to být **jiný bot** než PromoClown nebo PromoHub — jeden
   token má jednoho pollera, `pass-api` jen posílá, ale ať se to neplete).
5. **Ověření**: `curl https://pass.ol1n.com/healthz` → `ok`; `curl https://pass.ol1n.com/pass/apps.json
   | jq .version` → `1`; RevenueCat → Webhooks → Send test event → zpráva v `#pass`
   (TEST eventy jdou jen do logu — ověř v `docker logs pass-api`).
6. Port 8096 zapiš do `DevConventions/shared/ports.md` (privátní repo), tady ne.

---

## 5. Pilot (plán §5, krok 4): jedna dětská a jedna nedětská appka

Doporučení: **Kirian** (nedětská, iOS i Play, skiny) + **Swype Kids** (dětská, TestFlight).
Checklist, všechno na sandbox/license účtech:

| # | Scénář | Jak | Očekávání |
|---|---|---|---|
| 1 | Koupě v A → odemčeno v B, iOS | sandbox tester, kup Pass v Kirianu, otevři Swype Kids | Swype Kids bez jakéhokoli dialogu ukazuje `PassBadge`; v RC Customer má obě appky stejné `app_user_id` |
| 2 | Koupě v A → odemčeno v B, Android | license tester, obě appky nainstalované | B najde UUID přes ContentProvider (log `identity source=provider`) |
| 3 | Android, jen jedna appka nainstalovaná | odinstaluj A, nainstaluj B, „Propojit s Google účtem“ | `Purchases.logIn(google:…)` → RC sloučí; B odemčená |
| 4 | Link code iOS → Android | v A (iOS) „Zobrazit kód“, v B (Android) „Zadat kód“ | do 15 min odemčeno; druhé použití kódu → `invalid_code` |
| 5 | Restore po reinstalu | smaž všechny appky, nainstaluj jednu, „Obnovit nákupy“ | nové UUID, RC transfer → `all_apps` aktivní |
| 6 | Refund | RC Customer → Transactions → **Refund** (sandbox) | do minuty webhook `CANCELLATION`, Discord zpráva, obě appky zamčené po dalším `getCustomerInfo()` |
| 7 | Kids: žádný login bez brány | Swype Kids, Android, paywall | tlačítko Google účtu je až za rodičovskou bránou appky; iOS nic nenabízí |
| 8 | Offline | zapni letový režim, spusť B | `hasPass` z RC cache stále `true` |
| 9 | Review notes | ASC verze Kiranu | text z CONTRACTS §2 + sandbox účet |

Pilot je hotový, když projde 1–8 na obou OS a Kirian je v review s poznámkou z bodu 9.

---

## 6. Pravidlo pro novou appku (plán §5, krok 7)

Před **prvním** submitem každé nové appky:

- [ ] `ol1n_pass` v `pubspec.yaml` od první verze; init v `main()` ([INTEGRATION](INTEGRATION.md))
- [ ] iOS: Keychain Sharing `com.ol1n.pass` v Signing & Capabilities
- [ ] záznam v `catalog/apps.json`: `slug`, bundle/package, `unlock_value_usd`, `unlocks`, `kids`,
      `pass_since` = verze, kterou submituješ; `make sync-catalog`; `make check-catalog`
- [ ] RevenueCat: App Store + Play app položky, produkty (`passctl rc products apply`), `passctl rc check` zelený
- [ ] ASC: `passctl asc products apply --app <slug>`, review screenshot, IAP zařazené do verze
- [ ] Play: build v internal track, `passctl play products apply --app <slug>`
- [ ] review notes s textem z CONTRACTS §2
- [ ] `make sync-site` → v `ol1n.now` commit + push + `make deploy` (katalog i landing se aktualizují)
- [ ] PromoClown `projects.yaml`: appka má `hooks` zmínku o Passu, pokud ho má ([PROMO_WIRING](PROMO_WIRING.md))
