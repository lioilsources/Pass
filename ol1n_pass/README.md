# ol1n_pass

Jeden nákup, který odemkne všechny appky od Ol1n — i ty, které teprve vyjdou.
Balíček pro Flutter, který každá appka přidá stejně jako `cute_kid_fonts`:

- **Entitlement `all_apps`** v jednom RevenueCat projektu; nákup jde vždy přes
  StoreKit / Play Billing (`purchases_flutter`).
- **Identita bez loginu.** iOS: UUID ve sdíleném keychainu (access group
  `com.ol1n.pass`). Android: UUID, který si appky předávají přes
  `ContentProvider` chráněný `signature` oprávněním — stačí mít nainstalovanou
  jednu appku s Passem. Záchranná síť: Google Sign-In (Android) a **link code**
  (funguje i iOS ↔ Android).
- **Katalog `apps.json`** z `https://olin.now/pass.json` (fallback bundlovaný):
  seznam appek, odkazy do storů, hodnota všech unlocků → paywall ukazuje
  „Hodnota všech balíčků dnes: $XX · Ol1n Pass $49.99“, a to jen když hodnota převyšuje cenu Passu.
- **Widgety**: `PassPaywall` (3 karty + seznam appek + Restore + kód),
  `PassBadge`, `PassAppsList`. Česky a anglicky.
- **Testovatelné bez storu**: `FakePassStore` a `InMemoryIdentityStore`
  v `ol1n_pass_testing.dart`.

Kontrakty (product ID, API, schéma katalogu) jsou v
[`../docs/CONTRACTS.md`](../docs/CONTRACTS.md); postup integrace do konkrétní
appky krok za krokem v [`../docs/INTEGRATION.md`](../docs/INTEGRATION.md).

## Instalace

```yaml
dependencies:
  ol1n_pass:
    git:
      url: https://github.com/lioilsources/Pass.git
      ref: v0.1.0
      path: ol1n_pass
```

Při vývoji vedle repa:

```yaml
dependency_overrides:
  ol1n_pass:
    path: ../Pass/ol1n_pass
```

## Integrace do appky v pěti krocích

1. **pubspec** — závislost výše. Android nepotřebuje nic dalšího: manifest
   balíčku přidá oprávnění, `<queries>`, provider i marker service sám.
2. **iOS keychain sharing** — v Xcode Runner → Signing & Capabilities →
   *Keychain Sharing* → skupina `com.ol1n.pass`. Do `Runner.entitlements`
   tím přibude:

   ```xml
   <key>keychain-access-groups</key>
   <array>
     <string>$(AppIdentifierPrefix)com.ol1n.pass</string>
   </array>
   ```

   Bez toho Pass v appce funguje, ale nepřenese se do ostatních appek jinak
   než přes Restore / link code.
3. **init v `main()`** — před `runApp`, nebo hned po něm:

   ```dart
   await PassService.instance.init(PassConfig(
     appSlug: 'kirian',                       // slug z apps.json
     revenueCatApiKeyIos: 'appl_…',
     revenueCatApiKeyAndroid: 'goog_…',
     iosKeychainAccessGroup: '<AppIdentifierPrefix>com.ol1n.pass', // Team ID + tečka; drž mimo veřejné repo
     kidsMode: false,
   ));
   ```
4. **unlock logika** — kde appka dnes testuje vlastní nákup, přidej Pass:

   ```dart
   bool get unlocked => hasAppUnlock || PassService.instance.hasPass.value;
   // nebo reaktivně:
   ValueListenableBuilder<bool>(valueListenable: PassService.instance.hasPass, …)
   ```
5. **paywall** — místo (nebo vedle) vlastního:

   ```dart
   PassPaywall(
     appUnlock: PassAppUnlock(
       title: 'Všechny skiny', price: skinStore.bundlePrice, onBuy: skinStore.buyAll,
     ),
     onOpenStore: (url) => launchUrl(url),   // appka si zvolí, čím otevírá odkazy
   )
   ```

   Po koupi Passu widget sám přepne na „Máš Ol1n Pass — odemčeno i v: …“ se
   seznamem appek a odkazy do storu.

## Dětské appky (`kidsMode: true`)

Apple Kids Category i Google Designed for Families zakazují login bez
rodičovské brány. V `kidsMode` balíček **nikdy sám nespustí Google Sign-In**;
`PassPaywall` tlačítko *Propojit s účtem Google* ukáže jen s
`showGoogleLink: true`, a to má appka dát až za svou bránu. Keychain (iOS) a
provider (Android) brány nepotřebují — nejsou to účty.

## Link code

V appce s Passem *Zobrazit kód* → `ABCD-EFGH` (platí 15 minut, jednou).
V jiné appce / na jiném telefonu *Mám kód* → appka přejde na identitu držitele
(`Purchases.logIn`) a Pass je tam. Server `pass-api` kód vydá jen uživateli
s aktivním entitlementem, takže kód vždy vede k reálné store transakci
(Apple 3.1.1).

## Co balíček nedělá

- Neotevírá URL (`onOpenStore` callback).
- Neřeší consumables (herní měna) — Pass odemyká jen non-consumable obsah.
- Nenese RevenueCat klíče ani Team ID — ty má appka.

## Vývoj

```bash
flutter pub get
flutter analyze
flutter test
```

Po změně `../catalog/apps.json` zkopíruj novou verzi do `assets/apps.json`
(dělá to `make sync-catalog` v kořeni repa).
