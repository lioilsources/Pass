# PASS_PLAN.md — „Ol1n Pass“: jeden nákup odemkne všechny appky

Handoff pro Claude Code Director. Cíl: jednorázový (lifetime) nákup, který odemkne všechny in‑app balíčky ve všech appkách Ol1n na App Store i Google Play, včetně appek, které teprve vyjdou. Doplňkově roční předplatné se stejným nárokem.

## 0. Shrnutí rozhodnutí

| Téma | Rozhodnutí |
|---|---|
| Produkt | `Ol1n Pass` — non‑consumable (lifetime) + volitelně `Ol1n Pass Yearly` (auto‑renewable) |
| Nárok (entitlement) | jediný: `all_apps` |
| Zdroj pravdy | RevenueCat (fáze 1), vlastní Go služba možná později (fáze 3) |
| Identita uživatele | iOS: sdílený keychain v App Group (bez loginu). Android: Google Sign‑In (RevenueCat `logIn`) + záložní „link code“ |
| Cena | $49 lifetime napevno (součet unlocků v aktuálních appkách už dnes přesahuje $49, např. Kiran ~$10, Lexify ~$20); roční $14.99–19.99 |
| Nativní store bundly | nepoužít (Apple App Bundle vyžaduje placené/subscription appky; Google nic nemá). Apple Suite zvážit jen pro roční variantu na iOS 27+ |

Důvod: ani App Store ani Google Play nemají produkt „vše od vývojáře navždy“. Cross‑app odemykání musí dělat vlastní entitlement server; nákup samotný musí vždy projít StoreKit / Play Billing.

## 1. Produktový design

- **Co Pass odemyká:** všechny non‑consumable unlocky a „premium“ režimy v každé appce. Consumables (herní měna apod.) Pass neodemyká — vyhne se to sporům s review i s ekonomikou her.
- **Formulace v storech:** „Odemkne všechny prémiové funkce ve všech appkách od Ol1n, včetně těch, které vyjdou později. Aktuální seznam: olin.now/pass“. Žádné konkrétní sliby budoucích appek v IAP metadata (Apple 2.3 metadata accuracy).
- **Cena:** $49 napevno. Paywall i olin.now/pass ukotví hodnotu: „Hodnota všech balíčků dnes: $XX · Ol1n Pass $49“ — číslo `$XX` se počítá z `apps.json` (součet non‑consumable unlocků) a roste s každou novou appkou, cena Passu zůstává.
- **Consumables v Kiranu apod.:** rozhodnout explicitně (viz §6). Výchozí doporučení: Pass odemyká non‑consumable obsah; u her dát „Pass bonus“ (např. jednorázový startovní balíček), ne nekonečnou měnu.
- **Roční předplatné:** stejný entitlement `all_apps`, cena ~ $14.99/rok. Na iOS 27+ ho lze později nabídnout jako Apple **Suite** (jedno předplatné přes až 15 appek) — to řeší nativní discovery, lifetime to neřeší.
- **Zobrazení v appce:** paywall ukáže tři možnosti: unlock této appky / Pass lifetime / Pass yearly. Po koupi Passu zobrazit „Máš Ol1n Pass — odemčeno i v: <seznam appek s odkazy do storu>“ → cross‑promo zdarma.

## 2. Architektura

```
┌──────────────┐  purchase (StoreKit2 / Play Billing)   ┌──────────────┐
│  Appka A     │ ─────────────────────────────────────▶ │  RevenueCat  │
│  (Flutter)   │ ◀───────── entitlements[all_apps] ──── │  1 projekt   │
└──────────────┘                                        │  N apps      │
       │ app_user_id                                     └──────┬───────┘
       ▼                                                        │ webhook
 iOS: shared keychain (App Group)                        ┌──────▼───────┐
 Android: Google Sign‑In uid / link code                 │ Go: pass‑api │ (fáze 3, volitelné)
                                                         │ + Discord bot│
┌──────────────┐                                         └──────────────┘
│  Appka B     │  stejné app_user_id → stejné entitlements
└──────────────┘
```

Princip: každá appka používá **stejné `app_user_id`** v jednom RevenueCat projektu. Entitlement `all_apps` je pak vidět všude, kde je ten uživatel přihlášený / identifikovaný.

### 2.1 Store produkty

Product ID musí být unikátní v rámci účtu (Apple i Google), takže v každé appce vlastní ID, všechna mapovaná na entitlement `all_apps`:

- iOS: `com.ol1n.pass.lifetime.<appslug>`, `com.ol1n.pass.yearly.<appslug>`
- Android: `pass_lifetime` a `pass_yearly` (ID je per package, může být stejné napříč appkami; RevenueCat je rozliší podle package name)

V App Store Connect každé IAP vyplnit stejný display name „Ol1n Pass“ a screenshot pro review ukazující paywall se seznamem appek.

### 2.2 Identita bez loginu — iOS

- Všechny appky mají Keychain Sharing s access group `$(AppIdentifierPrefix)com.ol1n.pass` (stejný Team ID).
- Při prvním spuštění: přečti `pass_user_id` ze sdíleného keychainu; pokud není, vygeneruj UUID a ulož. Tohle UUID je `app_user_id` pro RevenueCat (`Purchases.configure(appUserID:)`).
- Výsledek: nákup v appce A → appka B při startu najde stejné UUID → `getCustomerInfo()` vrátí `all_apps`. Žádný login, žádná rodičovská brána.
- Flutter: `flutter_secure_storage` s `IOSOptions(accessGroup:)` nebo vlastní malý MethodChannel přes `SecItem*` s `kSecAttrAccessGroup`.
- Edge: uživatel smaže všechny appky → keychain v App Group zůstane (dokud nesmaže poslední appku a iOS nevyčistí group; v praxi přetrvá). Záchrana: RevenueCat `restorePurchases()` propojí původní Apple transakci s novým UUID (transfer behavior v RC nastavit na „transfer to new app user id“).

### 2.3 Identita — Android

Žádný sdílený keychain, `queryPurchasesAsync` vidí jen nákupy vlastního package. Vrstvy:

1. **Google Sign‑In (primární):** po koupi Passu nabídnout „Propojit s Google účtem, aby Pass fungoval i v dalších appkách“. `Purchases.logIn(googleUid)`. V dalších appkách stačí stejné tlačítko. (Pro dětské appky schovat za parental gate — viz §4.)
2. **Sdílení mezi appkami stejného podpisu (vylepšení):** malý `ContentProvider` v každé appce s `android:permission` na `signature` level, který vydá `pass_user_id`. Appka B při startu obejde nainstalované Ol1n appky (seznam package names v `<queries>`), zkusí UUID přečíst. Funguje offline, bez loginu, pokud je nainstalovaná aspoň jedna appka s Passem.
3. **Link code (záchranná síť):** v appce s Passem „Zobrazit kód“ → 8 znaků, server (RC REST / fáze 3 Go API) ho spáruje s `app_user_id`; v jiné appce „Zadat kód“. Jednoduché, cross‑platform (funguje i iOS↔Android).

### 2.4 Shared Flutter package `ol1n_pass`

Stejný vzor jako `cute_kid_fonts` — jeden balíček pro všechny appky:

```
ol1n_pass/
  lib/
    ol1n_pass.dart        // PassService: init(), hasPass, buyLifetime(), buyYearly(), restore(), linkCode()
    identity/ios_keychain.dart
    identity/android_provider.dart
    identity/google_signin.dart
    ui/pass_paywall.dart  // widget: 3 karty (app unlock / Pass lifetime / Pass yearly) + seznam appek
    ui/pass_badge.dart    // „Máš Ol1n Pass“
  apps.json              // seznam appek: slug, name, storeUrls, obsahuje Pass od verze X
```

`PassService.hasPass` je `ValueListenable<bool>`; existující per‑app unlock logika se přepíše na `hasAppUnlock || hasPass`.

`apps.json` tahat vzdáleně z `https://olin.now/pass/apps.json` (fallback na bundlovaný), aby nové appky přibývaly bez updatu.

## 3. Backend

**Fáze 1 — RevenueCat only.** Žádný vlastní server. Projekt „Ol1n“, do něj přidat všech 5–6 appek (iOS + Android zvlášť), entitlement `all_apps`, offering `pass`. Webhook → Discord (Ol1nBot) na `INITIAL_PURCHASE` / `RENEWAL` / `CANCELLATION`.

**Fáze 3 — vlastní Go `pass-api` (volitelné, až to bude nosné):**
- `GET /v1/entitlements?user=<id>` — čte z RC REST API nebo vlastní DB
- `POST /v1/link-code`, `POST /v1/redeem`
- `GET /pass/apps.json`
- App Store Server Notifications V2 + Play RTDN, pokud by se RC opouštěl
- deploy stejným způsobem jako ostatní Ol1n služby (Mac Mini / JODA)

## 4. Review a compliance

- **Apple 3.1.1:** odemykání je čistě z IAP transakcí (RC je jen eviduje) → OK. Nikdy neodemykat přes vlastní „license key“ bez vazby na store transakci; link code musí vždy vést k existující store transakci.
- **Apple 3.1.2(a) / multi‑app:** Apple sám dokumentuje sdílení předplatného napříč appkami přes vlastní server; pro non‑consumable stejný vzor. Do review notes napsat: „Ol1n Pass is a non‑consumable purchased via In‑App Purchase; entitlement is shared across our own apps via our server (RevenueCat). Test account: …“.
- **Kids Category (Apple) / Designed for Families (Google):** žádný login bez parental gate; proto na iOS keychain bez loginu, na Androidu Google Sign‑In jen za bránou a s vysvětlením. Cross‑promo seznam appek je povolený jen jako odkazy do storu, bez třetích stran.
- **Google Play:** vše přes Play Billing, Pass je standardní one‑time product. Cross‑app nárok z vlastního serveru Play Policy nezakazuje.
- **Refundy:** RC webhook `CANCELLATION` / `REFUND` → entitlement padá ve všech appkách automaticky.
- **Family Sharing:** u Apple non‑consumable zapnout Family Sharing (RC to podporuje) — pro dětské appky velký prodejní argument. Google Play Family Library obdobně zvážit.

## 5. Plán implementace

1. **RC projekt** — založit, přidat appky, entitlement `all_apps`, offering `pass`, webhook na Discord. (0,5 dne)
2. **Store produkty** — vytvořit lifetime + yearly v každé appce (ASC + Play Console), ceny $49 / $14.99–19.99. (0,5 dne)
3. **`ol1n_pass` package** — PassService, iOS keychain identity, Android signature ContentProvider, Google Sign‑In, paywall widget. (2 dny)
4. **Pilot ve 2 appkách** (jedna dětská, jedna ne) — integrace, TestFlight + internal testing, ověřit: koupě v A → odemčeno v B na obou OS; restore po reinstalu; refund shodí nárok. (1 den)
5. **olin.now/pass** — landing: co obsahuje, seznam appek, mechanika ceny, `apps.json`. (0,5 dne)
6. **Rollout do zbylých appek** — každá nová verze s `ol1n_pass`; submit s review notes. (dle počtu)
7. **Pravidlo pro nové appky:** každá nová appka má `ol1n_pass` od první verze a je přidaná do RC + `apps.json` (včetně hodnoty jejích unlocků) před submitem.

## 6. Otevřené otázky pro Ol1na

- Cena: $49 lifetime — rozhodnuto.
- Roční předplatné hned, nebo až po pilotu? Pokud hned, $14.99 nebo $19.99?
- Consumables (Kiran a další hry): Pass je neodemyká vůbec / dává jednorázový bonus / odemyká vše? Lexify ($20 non‑consumable) je bez debaty.
- Family Sharing zapnout od začátku?
- Zůstat na RC, nebo plánovat vlastní Go API od začátku kvůli Discord bot integraci a nezávislosti?

## 7. Zdroje

- Apple — Offering a Subscription Across Multiple Apps: https://developer.apple.com/documentation/storekit/offering-a-subscription-across-multiple-apps
- Apple — Bundles and Suites (iOS 27): https://developer.apple.com/app-store/subscriptions/bundles-and-suites/
- Apple — App bundles: https://developer.apple.com/app-store/app-bundles/
- RevenueCat — multi‑app entitlements: https://community.revenuecat.com/sdks-51/implement-subscritions-bundle-3165
