# INTEGRATION — jak dostat Ol1n Pass do existující appky

Na příkladu **Kiranu** (per-skin non-consumable v `lib/services/skin_store_service.dart`) a
**Lexify** (balíčky kartiček: `lib/services/iap_service.dart` + `EntitlementService`). Obě appky
dnes používají `in_app_purchase` napřímo; to zůstává — Pass přidává druhý zdroj nároku, nenahrazuje
existující unlocky. API balíčku je v [`CONTRACTS.md §6`](CONTRACTS.md) a `ol1n_pass/README.md`.

## 0. Co se v appce změní

```
před:  unlocked = hasAppUnlock
po:    unlocked = hasAppUnlock || PassService.instance.hasPass.value
```

Plus paywall s třetí a čtvrtou kartou (Pass lifetime / Pass yearly), badge po koupi, a na
Androidu tlačítko „Propojit s Google účtem“ / „Zadat kód“. Nic víc.

## 1. Závislost

```yaml
dependencies:
  ol1n_pass:
    git:
      url: https://github.com/lioilsources/Pass.git
      ref: v0.1.0          # tag, ne větev — stejně jako cute_kid_fonts
      path: ol1n_pass
```

Při vývoji vedle sebe (repo `Pass` jako sourozenec appky):

```yaml
dependency_overrides:
  ol1n_pass:
    path: ../Pass/ol1n_pass        # Kiran: ../../Pass/ol1n_pass (kód je v tyrian_mobile/)
```

`ol1n_pass` si přitáhne `purchases_flutter`, `flutter_secure_storage`, `google_sign_in` a `http`.
`in_app_purchase` v appce zůstává — StoreKit i Play Billing zvládnou dva klienty vedle sebe, ale
**oba musí dokončovat jen svoje transakce**: Pass produkty (`com.ol1n.pass.*`, `pass_*`) dokončuje
RevenueCat, ostatní appka. Lexify má v `IAPService.initialize()` „Clear stuck iOS transactions“, které
volá `finishTransaction` na **všechny** čekající transakce — po integraci to omez na vlastní SKU,
jinak to při startu spolkne nedokončenou Pass transakci dřív, než ji RC zpracuje.

## 2. iOS: sdílená klíčenka

1. Xcode → Runner → Signing & Capabilities → **+ Capability → Keychain Sharing** → přidej skupinu
   `com.ol1n.pass`. Xcode zapíše do `ios/Runner/Runner.entitlements`:
   ```xml
   <key>keychain-access-groups</key>
   <array><string>$(AppIdentifierPrefix)com.ol1n.pass</string></array>
   ```
2. Totéž v každé další appce. Stejný Team ID → stejný `AppIdentifierPrefix` → stejná skupina.
   Na developer portálu se nic nezapíná (není to App ID capability).
3. Do `PassConfig.iosKeychainAccessGroup` dej celý řetězec včetně prefixu (`<TEAMID>.com.ol1n.pass`).
   Prefix není v repu — ber ho z `Distribution/SKILL.md` a předávej přes `--dart-define`. `null` nebo
   prázdný řetězec znamená **bez sdíleného keychainu**: identita zůstane jen v této appce a Pass se do
   ostatních appek dostane jen přes Restore / link code. Hodí se pro dev buildy, do release ne.
4. CI (`Distribution/workflows/release-ios.yml`) podepisuje přes `ExportOptions.plist`; provisioning
   profil si Keychain Sharing nevyžaduje přegenerovat, ale **archiv bez entitlementu klíčenku
   nesdílí** — ověř v `codesign -d --entitlements :- Runner.app`, že skupina tam je.

## 3. Android

Nic ručně. Plugin má v manifestu `ContentProvider` (authority `${applicationId}.ol1npass`),
`signature` permission `com.ol1n.pass.READ_IDENTITY`, discovery service s akcí
`com.ol1n.pass.IDENTITY` a `<queries>` pro ni — merge do appky udělá Gradle. Po buildu ověř:

```bash
# v build/app/intermediates/merged_manifests/release/AndroidManifest.xml
grep -c "com.ol1n.pass" AndroidManifest.xml     # provider, permission, service, queries → ≥ 4
```

Podmínka pro sdílení mezi appkami: **stejný podpisový klíč**. Appky z Play s Play App Signing mají
stejný upload klíč, ale Play ji přepodepisuje — `signature` permission porovnává **Play App Signing
key**, takže musí být stejný napříč appkami. U appek založených přes `Distribution/setup-gh-secrets.sh`
to tak je jen tehdy, když se při prvním uploadu zvolilo „use the same key as another app“. Pokud
ne, provider mezi těmi appkami nefunguje a zbývá Google účet / kód — balíček to zvládne sám (zaloguje
`identity source=…`). Napiš to do `notes` v katalogu u appky, které se to týká.

## 4. Inicializace

```dart
// main.dart — před runApp(), po WidgetsFlutterBinding.ensureInitialized()
await PassService.instance.init(PassConfig(
  appSlug: 'kirian',                                         // = slug v catalog/apps.json
  revenueCatApiKeyIos: const String.fromEnvironment('RC_IOS_KEY'),
  revenueCatApiKeyAndroid: const String.fromEnvironment('RC_ANDROID_KEY'),
  iosKeychainAccessGroup: const String.fromEnvironment('PASS_KEYCHAIN_GROUP'), // '' → bez sdílení
  kidsMode: false,
));
```

Klíče přes `--dart-define` (`flutter run --dart-define=RC_IOS_KEY=appl_…`); v CI jako GitHub
secrets `RC_IOS_KEY`, `RC_ANDROID_KEY`, `PASS_KEYCHAIN_GROUP` a `--dart-define` v
`release-*.yml` (šablony v `Distribution/workflows/` — po úpravě re-sync golden podle jeho CLAUDE.md).
Public SDK klíče RC nejsou tajemství v pravém smyslu (jsou v binárce), ale do veřejného repa nepatří.

`init()` neblokuje start o víc než čtení klíčenky: RC se konfiguruje synchronně a `hasPass` se
doplní z cache a pak ze sítě; appka na nic nečeká.

## 5. Kirian: `SkinStoreService`

```dart
bool isUnlocked(String skinId) {
  if (!supported) return true;
  if (skinById(skinId)?.productId == null) return true;
  return _unlocked.contains(skinId) || PassService.instance.hasPass.value;   // ← Pass
}

Future<void> init() async {
  …
  PassService.instance.hasPass.addListener(notifyListeners);   // selector se překreslí, když Pass přijde/odejde
}
```

`priceFor` nech beze změny: u držitele Passu selector zámky nekreslí, protože `isUnlocked` je true.
`restore()` volej obojí: `InAppPurchase.instance.restorePurchases()` i `PassService.instance.restore()`
— Apple review chce jedno tlačítko „Restore“, které obnoví všechno.

## 6. Lexify: `EntitlementService`

Mapování product → deck zůstává. V místě, kde se rozhoduje, zda je deck odemčený:

```dart
bool isDeckUnlocked(String deckId) =>
    _ownedDecks.contains(deckId) || PassService.instance.hasPass.value;
```

A `catalog.json` free decky se nemění — Pass neovlivňuje, co je zdarma, jen co je placené.

## 7. Paywall a badge

```dart
PassPaywall(
  appUnlock: AppUnlockCard(                       // první karta: odemknutí jen této appky
    title: 'Všechny skiny',
    price: skinStore.priceFor(skinId),           // lokalizovaná cena ze storu, nebo null
    onBuy: () => skinStore.buy(skinId),
  ),
  onParentalGate: kidsMode ? () => showParentalGate(context) : null,   // Android, Google účet
)
```

Widget sám ukáže karty **Ol1n Pass** (lifetime) a **Ol1n Pass Yearly** (jen když RC offering
pro tuhle appku `$rc_annual` má), pod nimi seznam appek z katalogu s hodnotou unlocků a součtem
(„Hodnota všech balíčků dnes: $XX · Ol1n Pass $49.99“, jen když hodnota převyšuje cenu), a na Androidu „Propojit s Google účtem“ /
„Zadat kód“. Po koupi `PassBadge()` kdekoli v nastavení/obchodě: „Máš Ol1n Pass — odemčeno i v: …“
s odkazy do storů (cross-promo zdarma, plán §1). Kde paywall ukázat: tam, kde dnes appka ukazuje
cenu per-app unlocku (Kiran: `SkinSelector` při kliknutí na zamčený skin; Lexify: detail placeného
decku). Nepřidávej Pass do onboardingu — review to vidí jako nátlak u dětských appek.

Vlastní vzhled: `PassPaywall` bere `PassTheme` (barvy, zaoblení, font) — dětské appky mu dají
`cute_kid_fonts` styl, hry svůj. Texty jsou cs/en podle `Localizations.localeOf(context)`,
přepsat jde přes `PassStrings`.

## 8. Dětské appky (`kidsMode: true`)

- iOS: nic navíc, klíčenka bez loginu.
- Android: balíček **nezobrazí** Google Sign-In, dokud appka nezavolá `onParentalGate` a ten
  nevrátí `true`. Bránu appka dodá sama (SwypeKids má „Rodičovský koutek“ s početní otázkou — použij
  tu). Link code je bez brány: zadání osmi znaků není účet.
- Žádné odkazy ven mimo store (Kids Category 1.3): `PassBadge` v kids režimu ukazuje jen názvy
  appek a store ikonky, ne web.

## 9. Hry se spotřebními položkami (Kiran, další)

Plán §1: Pass neodemyká consumables. U hry dej **Pass bonus** — jednorázový balíček při prvním
startu s Passem (`PassService.instance.hasPass` přešlo na `true` a `prefs['pass_bonus_given']`
chybí). Zapiš ho do `unlocks` v katalogu jako „startovní balíček“ bez hodnoty (do součtu nepatří).

## 10. Před submitem

1. `catalog/apps.json`: u appky doplň `unlock_value_usd` (součet non-consumable unlocků v USD),
   `unlocks`, `pass_since` = verze, kterou submituješ. `make sync-catalog && make check-catalog`.
2. RevenueCat: app položky, produkty, `passctl rc check` ([RUNBOOK §1](RUNBOOK.md)).
3. Store produkty: `passctl asc products apply --app <slug>`, `passctl play products apply --app <slug>`
   ([RUNBOOK §2–3](RUNBOOK.md)); review screenshot paywallu.
4. Review notes z CONTRACTS §2.
5. `make sync-site` → commit + push + deploy v `ol1n.now`, aby `pass.json` ukazoval appku jako
   dostupnou **ve stejný den**, kdy verze vyjde — paywally v ostatních appkách si katalog stahují.
6. Verze appky s Passem je taky zpráva pro promo: PromoClown `projects.yaml` → `hooks` té appky
   doplň o „Part of Ol1n Pass“ ([PROMO_WIRING](PROMO_WIRING.md)).
