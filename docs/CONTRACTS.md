# CONTRACTS — co musí sedět mezi `ol1n_pass`, `pass-api`, `passctl`, webem a story

Zdroj pravdy pro identifikátory a rozhraní. Když se tu něco změní, mění se ve všech
čtyřech částech repa najednou. Implementuje `Prompts/00-PASS_PLAN.md`.

## 1. Entitlement a identita

| Co | Hodnota |
|---|---|
| RevenueCat projekt | `Ol1n` (jeden projekt, všechny appky iOS + Android) |
| Entitlement | `all_apps` |
| Offering | `pass` — packages `$rc_lifetime` (lifetime) a `$rc_annual` (yearly) |
| `app_user_id` | UUID v4 (iOS: sdílený keychain; Android: sdílený ContentProvider), nebo `google:<uid>` po Google Sign-In |
| iOS keychain access group | `<AppIdentifierPrefix>com.ol1n.pass`, klíč `pass_user_id` (service `com.ol1n.pass`) |
| Android identity provider | authority `<applicationId>.ol1npass`, path `identity`, sloupec `pass_user_id`; permission `com.ol1n.pass.READ_IDENTITY` (`signature`); objevování přes intent action `com.ol1n.pass.IDENTITY` |
| RC transfer behavior | „Transfer to new App User ID“ (restore po reinstalu přenese nákup na nové UUID) |

## 2. Store produkty (odvozené ze `slug`, nikdy ručně)

| Store | Lifetime (non-consumable) | Yearly (auto-renewable) |
|---|---|---|
| App Store | `com.ol1n.pass.lifetime.<slug>` | `com.ol1n.pass.yearly.<slug>` v subscription group `ol1n_pass` |
| Google Play | `pass_lifetime` (one-time, per package) | `pass_yearly`, base plan `yearly` (P1Y) |

Display name všude „Ol1n Pass“ / „Ol1n Pass Yearly“. Ceny: lifetime **49 USD**, yearly **14.99 USD**
(Apple price point tier nejblíž, Play `otherRegionsConfig` USD + EUR). Family Sharing u lifetime zapnuté.

Review note (stejná u všech IAP): *„Ol1n Pass is a non-consumable purchased via In-App Purchase;
the entitlement is shared across our own apps via our server (RevenueCat). Test account: …“*

## 3. `apps.json` (katalog)

Zdroj: `catalog/apps.json`. Publikuje se na `https://olin.now/pass.json` (statický soubor na
GitHub Pages — plán říká `/pass/apps.json`, ale adresář `pass/` by se tloukl se stránkou `/pass`)
a zrcadlí ho `pass-api` na `GET /pass/apps.json`. Balíček `ol1n_pass` má kopii v `assets/apps.json`
jako fallback.

```jsonc
{
  "version": 1,                 // schéma; klient odmítne vyšší major
  "updated": "YYYY-MM-DD",
  "entitlement": "all_apps",
  "pass": {
    "name": "Ol1n Pass",
    "lifetime_usd": 49.0, "yearly_usd": 14.99,
    "landing_url": "https://olin.now/pass",
    "catalog_url": "https://olin.now/pass.json",
    "api_url": "https://pass.ol1n.com",
    "ios_subscription_group": "ol1n_pass",
    "android_yearly_base_plan": "yearly"
  },
  "apps": [{
    "slug": "kirian",                 // = slug na olin.now
    "name": "Kirian",
    "ios_bundle_id": "com.ol1n.kiran",
    "android_package": "com.ab.kiran",
    "appstore_id": "6774868017" | null,
    "appstore_url": "https://apps.apple.com/app/id…" | null,
    "playstore_url": "https://play.google.com/store/apps/details?id=…" | null,
    "unlock_value_usd": 10.0 | null,  // součet non-consumable unlocků; null = ještě nevyplněno
    "unlocks": ["14 ship skins"],
    "kids": false,                    // Kids Category / Designed for Families → bez loginu bez brány
    "pass_since": "2.6.0" | null,     // verze appky s ol1n_pass; null = Pass tam ještě není
    "notes": "…"                      // jen pro lidi
  }]
}
```

Pravidla: `slug` unikátní; appka s `pass_since == null` se v paywallu i na webu ukazuje jako „brzy“,
ne jako odemčená. Dvě součtová čísla, všude stejně (Dart, Go, web JS):
- `total_unlock_value_usd` = součet `unlock_value_usd` jen přes appky s `pass_since != null` — „Hodnota
  všech balíčků dnes“ v paywallu i na landingu (nikdy nepočítat appku, kde Pass ještě nefunguje — Apple 2.3);
- `total_listed_value_usd` = součet přes všechny appky s hodnotou — kolik to bude po rolloutu (`passctl catalog value`).

## 4. `pass-api` (Go, JODA, `https://pass.ol1n.com`)

Všechny odpovědi JSON, chyby `{"error":"<code>","message":"…"}`.

| Metoda a cesta | Auth | Tělo / odpověď |
|---|---|---|
| `GET /healthz` | — | `ok` |
| `GET /pass/apps.json` | — | katalog (soubor z `PASS_CATALOG_PATH`, default embedded) |
| `GET /v1/entitlements?user=<app_user_id>` | — | `{"app_user_id":"…","all_apps":true,"source":"revenuecat","checked_at":"…"}` — RC REST `GET /v1/subscribers/{id}` se secret key, cache 60 s |
| `POST /v1/link-code` | — | in `{"app_user_id":"…"}` → ověří aktivní `all_apps`, jinak `403 no_pass`; out `{"code":"ABCDEFGH","display":"ABCD-EFGH","expires_at":"…"}` |
| `POST /v1/redeem` | — | in `{"code":"ABCD-EFGH"}` (pomlčky/mezery/case ignoruj) → out `{"app_user_id":"…"}`; `404 invalid_code` i pro expirovaný/použitý; klient pak `Purchases.logIn(app_user_id)` |
| `POST /webhooks/revenuecat` | `Authorization: Bearer $RC_WEBHOOK_SECRET` | RC webhook v1 payload; dedup na `event.id`; → Discord embed (+ Telegram), uloží `purchase_events` |

Link code: 8 znaků z abecedy `ABCDEFGHJKLMNPQRSTUVWXYZ23456789` (bez 0/O/1/I), TTL 15 min,
single-use, v DB jen SHA-256. Rate limit: `/v1/redeem` 10/min/IP, `/v1/link-code` 5/min/user.

Env: `PASS_ADDR` (`:8096`), `PASS_DB_PATH`, `PASS_CATALOG_PATH`, `RC_SECRET_API_KEY`,
`RC_WEBHOOK_SECRET`, `DISCORD_WEBHOOK_URL`, `TELEGRAM_BOT_TOKEN`, `TELEGRAM_CHAT_ID`. Port 8096 je
další volný v registru JODA (`DevConventions/shared/ports.md`: 8094 promo-api, 8095 ugc-api).

RC webhook typy, které se hlásí: `INITIAL_PURCHASE`, `NON_RENEWING_PURCHASE`, `RENEWAL`,
`CANCELLATION`, `EXPIRATION`, `BILLING_ISSUE`, `TRANSFER`, `PRODUCT_CHANGE`. `TEST` jen do logu.

## 5. `passctl` (Go CLI, běží na Macu)

```
passctl catalog check                       # schéma, unikátní slugy, součet hodnoty
passctl catalog value                       # vytiskne total_unlock_value_usd
passctl asc products plan  --app <slug>     # co by se vytvořilo (dry-run), bez sítě
passctl asc products apply --app <slug>     # ASC API: IAP lifetime + yearly (group, localization, price), idempotentní
passctl play products plan|apply --app <slug>
passctl rc check                            # RC REST v2: produkty ↔ entitlement all_apps ↔ offering pass
passctl rc products apply                   # založí v RC chybějící produkty a přiřadí k all_apps
```

Env: `ASC_KEY_ID`, `ASC_ISSUER_ID`, `ASC_PRIVATE_KEY_PATH` (stejné názvy jako PromoClown
`store-reviews`), `GOOGLE_PLAY_SERVICE_ACCOUNT_JSON`, `RC_SECRET_API_KEY` (v2), `RC_PROJECT_ID`.
`apply` je idempotentní: nejdřív `GET`, vytváří jen chybějící, nikdy nemaže a nemění ceny
existujících produktů.

## 6. Balíček `ol1n_pass` (Flutter)

```dart
await PassService.instance.init(PassConfig(
  appSlug: 'kirian',
  revenueCatApiKeyIos: 'appl_…', revenueCatApiKeyAndroid: 'goog_…',
  iosKeychainAccessGroup: '<AppIdentifierPrefix>com.ol1n.pass', // z Distribution, ne v repu
  kidsMode: false,
));
PassService.instance.hasPass            // ValueListenable<bool>
PassService.instance.buyLifetime()      // Future<PassPurchaseResult>
PassService.instance.buyYearly()
PassService.instance.restore()
PassService.instance.createLinkCode()   // Future<LinkCode>
PassService.instance.redeemLinkCode('ABCD-EFGH')
PassService.instance.linkGoogleAccount() // Android, za parental gate když kidsMode
PassService.instance.catalog            // PassCatalog (apps, totalUnlockValueUsd)
```

Appka přepíše svůj unlock test na `hasAppUnlock || PassService.instance.hasPass.value`.
Závislosti: `purchases_flutter ^11`, `flutter_secure_storage ^11`, `google_sign_in ^7`, `http ^1.6`.
Testovatelnost: `PassStore` (RevenueCat / fake) a `IdentityStore` (keychain / provider / in-memory)
jsou rozhraní; testy nikdy nesahají na síť ani platformu.
