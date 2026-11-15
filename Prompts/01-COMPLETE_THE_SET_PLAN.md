# 01 — Doplnění sady: cena „za vše“ klesá s tím, co už uživatel koupil

Plán pro implementaci. Dvě části, každý PR ve vlastním repu:

- **Část A** — uvnitř jedné appky: balíček „vše“ zlevňuje podle počtu už koupených položek.
  Čtyři appky, čtyři PR: Kirian (repo `Kiran`), Lexify (repo `DuolingoCards`), Mirror Booth
  (repo `MirrorBooth`) a Handwritten Stickers (repo `HandWritenStickers`).
- **Část B** — Ol1n Pass: cena lifetime Passu klesá podle toho, co uživatel nakoupil ve všech
  appkách Ol1n (repo `Pass`).

Než začneš, přečti `CLAUDE.md` každého repa, do kterého saháš, `docs/CONTRACTS.md`
a `docs/INTEGRATION.md` v `Pass` a `docs/PRESETS_MONETIZATION_PLAN.md` v `HandWritenStickers`.

## 1. Co chceme a proč

Dnes: balíček stojí 0,99 USD, „vše“ 4,99 USD. Kdo si koupil dva balíčky po jednom, zaplatí za
„vše“ pořád plnou cenu, takže se mu to nevyplatí a nabídku přestane dostávat. Chceme, aby se mu
dosavadní nákupy odečetly: po dvou balíčcích stojí „vše“ 2,99 USD.

Stejný princip pro Pass: kdo už v appkách nakoupil za 20 USD, nemá platit za Pass plných 49,99 USD.

## 2. Jak to jde udělat (ověřená fakta)

App Store ani Google Play neumí cenu produktu měnit podle uživatele. Cena patří k produktu
a k zemi, ne k zákazníkovi. Proto se to dělá **sadou produktů**: několik produktů odemyká totéž,
každý má jinou pevnou cenu a appka nabídne právě jeden podle toho, co uživatel vlastní.

Pojmy v tomto plánu:

- **stupeň** — jeden z produktů „vše“. Každý má **práh**: kolik položek (část A) nebo jaký
  kredit (část B) musí uživatel mít, aby se mu nabídl. Stupeň 0 je plná cena s prahem 0.
  Nabízí se vždy nejvyšší stupeň, jehož práh uživatel splňuje.
- **kredit** (jen část B) — součet katalogových cen v USD za nespotřební produkty, které uživatel
  už vlastní ve všech appkách Ol1n.

Zvážené a zamítnuté cesty, nevracej se k nim bez nového důvodu:

- **Apple offer codes pro non-consumable** (od WWDC25, iOS 16.3+): sleva přes kód a systémový
  dialog. Vlastní kódy jsou sdílitelné, jednorázové by musel rozdávat server, a na Androidu
  to nemá obdobu.
- **Google Play slevové nabídky u one-time produktů** (Play Billing Library 8): jeden produkt
  s více nabídkami. RevenueCat je nepodporuje a sám doporučuje druhý produkt se zlevněnou cenou;
  podpora v pluginu `in_app_purchase` není ověřená.

Co sada produktů stojí: každý produkt v App Store Connect potřebuje vlastní review screenshot,
projde review a jeho ID už nejde nikdy použít znovu. Proto málo stupňů.

Co nejde zabránit: appka rozhoduje, který stupeň nabídne, a obchod prodá kterýkoli produkt,
o který appka požádá. Upravený klient si tedy může koupit nejlevnější stupeň. U částek do 4,99 USD
to přijímáme. U Passu to aspoň zjistíme (část B, krok 7). Produkty vyšších stupňů se nesmí
zapnout jako propagované v App Storu, jinak by je šlo koupit ze stránky appky.

## 3. Rozhodnutí vlastníka

Rozhodnuto (Olin, 15. 11. 2026):

- Lexify a Kirian balíček „vše“ dostanou.
- Pořadí implementace: Kirian, Lexify, Mirror Booth, pak Handwritten Stickers.
- Balíček „vše“ stojí v každé appce 4,99 USD.
- Appek bude brzy víc než 10; cena Passu 49,99 USD zůstává.

Výchozí hodnoty, které platí, dokud Olin neřekne jinak:

| # | Otázka | Výchozí |
|---|---|---|
| R1 | Stupně Passu. | krok 10 USD: 49,99 / 39,99 / 29,99 / 19,99 |
| R2 | Kde Pass se stupni spustit. | nejdřív jen pilotní appka (Kirian) |
| R3 | Zahrnuje „vše“ v Kirianu a Lexify i budoucí skiny a balíčky, jako Komplet v Handwritten Stickers? | ano |

K R1: při ceně 4,99 USD za „vše“ utratí uživatel v jedné appce nejvýš asi 5 USD. Dnešní appky
dají dohromady necelých 20 USD kreditu, deset appek s balíčkem „vše“ asi 50 USD. Nejnižší stupeň
19,99 USD je dno: níž Pass nejde, ani když má uživatel koupené všechno. Stupně jsou v katalogu,
další jde přidat později a znamená jen nové produkty. Každý stupeň jsou ale dva produkty na appku
(App Store a Play) a u toho v App Storu ruční screenshot a Family Sharing, takže při deseti
appkách je to 60 produktů; proto krok 10 USD a ne 5.

**Zastav a zeptej se** před založením produktů v obchodech (ručně i přes `passctl … apply`):
ID, ceny a počet stupňů jsou nevratné. Kód, testy a dokumentaci dělej bez ptaní.

## 4. Část A — balíček „vše“ se stupni v appkách

### Stupně

Ve všech appkách stejně: položka stojí 0,99 USD, „vše“ 4,99 USD a za každou už koupenou
položku je „vše“ o dolar levnější.

| Stupeň | Cena (USD) | Práh: vlastněných položek |
|---|---|---|
| 0 | 4,99 | 0 |
| 1 | 3,99 | 1 |
| 2 | 2,99 | 2 |
| 3 | 1,99 | 3 |
| 4 | 0,99 | 4 a víc |

Položky: Kirian 21 skinů (14 schválených), Lexify 11 balíčků, Mirror Booth 15 placených kolekcí
(art, fantasy a 13 balíčků), Handwritten Stickers 6 balíčků.
V každé appce je to pět produktů „vše“.

Product ID (jen malá písmena, číslice, `_` a `.`, aby stejný řetězec prošel i v Google Play):

| Appka | Stupeň 0 | Stupeň k |
|---|---|---|
| Handwritten Stickers | `com.ol1n.handwrittenStickers.pack.all` (už v kódu) | `…pack.all.s<k>` |
| Kirian | `com.ol1n.kiran.skins_all` | `com.ol1n.kiran.skins_all_s<k>` |
| Lexify | `com.ol1n.duolingocards.decks_all` | `com.ol1n.duolingocards.decks_all_s<k>` |
| Mirror Booth | `com.ol1n.mirrorbooth.collection.all_unlock` (už v kódu) | `…collection.all_unlock_s<k>` |

U Handwritten Stickers mají dnešní ID velké `S`, které Play nejspíš odmítne (katalog Lexify to
u sebe výslovně zmiňuje). Ověř to při zakládání produktů; jestli to platí, řeš mapování
App Store ↔ Play v jednom místě a nahlas to.

Společná pravidla:

- Kdo vlastní **kterýkoli** stupeň, má odemčeno vše, i po Restore a včetně budoucích položek (R3).
- Kdo vlastní všechny dnešní položky, dostává dál nabídku posledního stupně, protože „vše“
  zahrnuje i budoucí.
- Ceny jsou cenové body v USD, ostatní měny přepočítá obchod. Ruční českou cenu neladit.
- Appka žádnou částku nepočítá. Ukazuje cenu nabízeného stupně z obchodu, u stupně 1 a výš
  k ní přeškrtnutou cenu stupně 0 a řádek „Už máš N …, odečetli jsme je“ (ARB, cs + en).
  Když některá cena chybí, přeškrtnutí skryje.
- Po koupi jednotlivé položky se nabídka přepočítá bez restartu.
- Výběr stupně je čistá funkce bez závislostí (`set_ladder.dart`): vstup počet vlastněných
  položek a seřazený seznam dvojic práh + ID, výstup ID k nabídnutí. Do každé appky jde stejný
  soubor se stejným tabulkovým testem; až budou appky záviset na `ol1n_pass`, přesune se tam.
- Review note u všech stupňů: *„Complete-the-set pricing: these products unlock identical
  content. The app offers exactly one of them, chosen by how many items the customer already
  owns, so earlier purchases are credited.“* Stejný název a screenshot paywallu.

Gate v každé appce: `flutter analyze` a `flutter test`. Žádný build pro iOS ani Android na M2;
sandbox nákup se zkouší na MacBooku. Worktree a větve `*-l10n` neupravuj.

### A1 — Handwritten Stickers

Stav: 6 balíčků `…pack.<id>` po 0,99 USD a `…pack.all`. K 7. 11. 2026 v App Store Connect žádný
z nich neexistoval a verze 1.0 čekala na odeslání; ověř přes `passctl asc iaps`, jestli to pořád
platí. Pokud ano, stupně můžou jít do review hned s první verzí.

1. `set_ladder.dart` a test.
2. `lib/services/entitlement_service.dart`: stupně odemykají vše, načítají se z obchodu,
   záložní ceny vedle řádků `'25 Kč'`.
3. Paywall podle společných pravidel.
4. Testy přes `FakePurchaseBackend`: nabídka podle počtu, odemčení přes `all.s3`, Restore,
   chybějící cena.
5. `docs/PRESETS_MONETIZATION_PLAN.md`: přepiš odstavec, podle kterého se Komplet nabízí jen do
   jednoho koupeného balíčku, a cenu Kompletu (dokument počítá s 99 Kč, platí 4,99 USD).
   Do release plánu dopiš ruční kroky v konzolích.

### A2 — Kirian

Stav: kód v `tyrian_mobile/`. `lib/services/skin_registry.dart` má 21 skinů s
`com.ol1n.kiran.skin_<id>`, obchod je `lib/services/skin_store_service.dart`, výběr v
`SkinSelector`. Balíček „vše“ zatím neexistuje. Appka je v obchodech, takže nové produkty
potřebují novou verzi.

1. `set_ladder.dart` a test.
2. `SkinStoreService`: pět nových produktů; `isUnlocked` vrací true, když uživatel vlastní
   kterýkoli stupeň; `restore()` je obnovuje. Počet vlastněných skinů = jen placené skiny.
3. `SkinSelector`: u zamčeného skinu vedle ceny skinu nabídka „Všechny skiny“ podle společných
   pravidel.
4. Testy: nabídka při 0, 1, 3, 4 a 9 skinech, odemčení přes stupeň, Restore.
5. Až bude Kirian pilotem Passu (část B), jde Pass i „Všechny skiny“ do stejného paywallu;
   pořadí karet: tento skin, všechny skiny, Ol1n Pass.

### A3 — Lexify

Stav: `assets/catalog.json` má 11 placených balíčků (`sku.apple` / `sku.google`, pole `unlocks`),
nákup `lib/services/iap_service.dart`, nárok `lib/services/entitlement_service.dart`. Balíček
„vše“ zatím neexistuje.

1. `set_ladder.dart` a test.
2. `catalog.json`: zápis balíčku „vše“ se stupni tak, aby odemykal i balíčky přidané později
   (ne výčtem v `unlocks`). `EntitlementService` a `IAPService` podle toho.
3. Obrazovka placeného balíčku: vedle jeho ceny nabídka „Všechny balíčky“ podle společných
   pravidel.
4. Testy: nabídka při 0, 2, 4 a 7 balíčcích, odemčení přes stupeň, Restore, nově přidaný balíček
   je u vlastníka „vše“ odemčený.

### A4 — Mirror Booth

Stav: `mirrorbooth/lib/core/mirror_filter.dart` má 15 placených kolekcí a volitelný balíček
„All filters“ (`bundleProductIds`: `…collection.all_unlock`, na Play i starší `…collection.all`);
paywall ho nabízí jen, když ho obchod vrátí. Nárok je v
`lib/features/paywall/entitlement_controller.dart`, obchod `lib/services/purchase_service.dart`,
testy v `test/entitlement_controller_test.dart`. Jestli `all_unlock` v App Store Connect existuje
a za kolik, není ověřené (ověř přes `passctl asc iaps --app mirrorbooth`); stupně počítají s tím,
že stupeň 0 je 4,99 USD.

1. `set_ladder.dart` a test.
2. `bundleProductIds` → stupně; `grantedBy` u kteréhokoli stupně vrací všechny placené kolekce.
   Starší `…collection.all` na Play dál odemyká vše.
3. `EntitlementState.bundle` drží nabízený stupeň podle počtu vlastněných placených kolekcí;
   promoted purchase z App Storu přijme kterýkoli stupeň.
4. Paywall podle společných pravidel.
5. Testy: nabídka při 0, 1, 4 a 15 kolekcích, odemčení přes stupeň, Restore, starší `…all`.

### Hotovo (část A)

V každé appce testy pokrývají tabulku stupňů a ve fake obchodu po koupi dvou položek paywall
nabízí stupeň 2 s jeho cenou z obchodu.

## 5. Část B — Ol1n Pass

Dnes: lifetime produkt na appku (`com.ol1n.pass.lifetime.<slug>`, na Play `pass_lifetime`),
cena 49,99 USD, nárok `all_apps` v RevenueCat. Appky prodávají své balíčky přes `in_app_purchase`
napřímo, Pass přes RevenueCat. Součet hodnot všech appek je dnes 35,64 USD.

### Návrh

- **Stupně** (R1): kredit ≥ 0 → 49,99; ≥ 10 → 39,99; ≥ 20 → 29,99; ≥ 30 → 19,99. Sleva je
  kredit zaokrouhlený dolů na krok, nikdy víc, než uživatel utratil.
- **ID stupňů** (rozšíření CONTRACTS §2): App Store `com.ol1n.pass.lifetime.<slug>.s<k>`,
  Play `pass_lifetime_s<k>`. Stupeň 0 zůstává na dnešních ID. Odvozují se ze `slug` v kódu,
  nikdy ručně.
- **Kdo počítá kredit: server.** Appka nevidí nákupy v jiných appkách ani na druhé platformě.
  `pass-api` dostane nový endpoint
  `GET /v1/offer?app_user_id=…&app=<slug>` →
  `{tier, credit_usd, package, products: {ios, android}}`, cache nejvýš 60 s jako u nároků.
- **Zdroj kreditu: zákaznický záznam v RevenueCat**, tedy nákupy, které RevenueCat pod sdíleným
  `app_user_id` zná ze všech appek. Započítají se jen produkty uvedené v katalogu; Pass,
  spotřební položky a předplatné ne. Balíček „vše“ z části A se počítá cenou stupně, který
  uživatel skutečně koupil.
- **Katalog** (`catalog/apps.json`, jediný zdroj pravdy): blok `pass` dostane seznam stupňů
  s cenou a minimálním kreditem; každá appka dostane seznam produktů s ID pro oba obchody, cenou
  v USD a druhem (jednotlivé odemknutí / balíček „vše“). `unlock_value_usd` se dál rovná součtu
  jednotlivých odemknutí bez balíčků „vše“ a `passctl catalog check` to hlídá. Když stupně
  v katalogu chybí, všechno se chová jako dnes.
- **Klient** (`ol1n_pass`): `PassService` si nabídku stáhne a paywall prodává balíček daného
  stupně. Bez sítě nebo při chybě platí stupeň 0; paywall na odpověď nikdy nečeká.
- **RevenueCat**: offering `pass` dostane vedle `$rc_lifetime` vlastní balíčky `lifetime_s1` až
  `lifetime_s3`, v každém produkty daného stupně za všechny appky. Všechny dávají `all_apps`.

### Kroky

0. **Ověření předpokladu, dřív než cokoli postavíš na RevenueCat.** Není ověřené, že RevenueCat
   zaznamená nákup, který appka provedla přes `in_app_purchase`, ani jak se oba klienti snesou
   při dokončování transakcí (viz INTEGRATION §1). Zjisti to z dokumentace RevenueCat (režim
   `purchasesAreCompletedBy`, `syncPurchases`) a napiš závěr do CONTRACTS. Sandbox test na
   zařízení potřebuje MacBook a RevenueCat účet, který zatím neexistuje; označ ho jako otevřený
   bod pilotu. Když z dokumentace vyjde, že to nejde, zastav a předlož Olinovi dvě náhrady:
   appka pošle serveru podepsaný doklad o nákupu (JWS ze StoreKit 2, purchase token z Play)
   a server ho ověří, nebo appka jen ohlásí, co vlastní, bez ověření.
1. Katalog: schéma, načítání a validace v `internal/catalog`, `make sync-catalog`. Produkty appek
   doplň z `passctl asc iaps` (rozšiř ho o výstup ve tvaru katalogu); Play ID bez service
   accountu doplň ručně z kódu appek a označ jako neověřená. `Counts()` v `internal/asc/iaps.go`
   nesmí do hodnoty appky počítat balíčky „vše“ a jejich stupně; dnes by je po schválení
   započítal.
2. `internal/catalog`: funkce kredit → stupeň a odvození ID stupňů. Tabulkové testy včetně hranic
   (9,99 / 10,00 / přes poslední stupeň).
3. `internal/revenuecat` + `internal/httpapi`: čtení nákupů zákazníka, `GET /v1/offer`. Bez klíče
   RevenueCat vrací 503 `not_configured` jako ostatní endpointy. Testy s falešným RevenueCat,
   bez sítě a bez `time.Now`.
4. `passctl`: `asc products plan|apply`, `play products plan|apply` a `rc products apply|check`
   umí stupně. Platí dosavadní invarianty: idempotentní, nic nemaže, cenu existujícího produktu
   nemění. Spusť jen `plan` a zastav se (viz §3).
5. `ol1n_pass`: model nabídky, stažení v `PassService`, paywall. Karta lifetime ukáže cenu stupně
   z obchodu, přeškrtnutou cenu stupně 0 a řádek „Za to, co už máš, je Pass levnější“ (cs + en
   v `PassStrings`). Pravidlo pro srovnání hodnoty appek s cenou se nemění. Testy: stupeň 2,
   chyba sítě → stupeň 0, chybějící balíček stupně v offeringu → stupeň 0.
6. Web `site/pass*.html`: jedna věta a jedna otázka ve FAQ, že dřívější nákupy se odečtou.
   Konkrétní částku tam nepiš, liší se podle uživatele. `make sync-site`.
7. Webhook: když přijde nákup produktu vyššího stupně, než na jaký má uživatel kredit, pošli
   upozornění stávajícím kanálem (`internal/notify`). Nic automaticky neodebírej.
8. Dokumentace: CONTRACTS §2 a endpointy, RUNBOOK (u každého produktu stupně ručně Family
   Sharing a review note, screenshot paywallu), INTEGRATION (co musí appka udělat, aby její
   nákupy šly do kreditu), `CLAUDE.md` nejvýš 80 řádků.

Gate: `make test`, `make check-catalog`.

Hotovo, když `make test` projde, `passctl asc products plan --app kirian` vypíše tři nové
produkty stupňů vedle stávajícího a paywall v testu s kreditem 21 USD prodává balíček
`lifetime_s2`.

### Mimo rozsah

- Roční Pass: beze stupňů.
- Změna ceny existujících produktů a mazání čehokoli v konzolích.
- Nasazení `pass-api` na JODA a zakládání produktů Passu pro jiné appky než pilotní.
- Vedlejší účinek k nahlášení, ne k řešení: jakmile RevenueCat uvidí i běžné nákupy v appkách,
  počítají se do obratu, ze kterého si nad bezplatný limit bere poplatek.

## 6. Pořadí

A2 (Kirian) → A3 (Lexify) → A4 (Mirror Booth) → A1 (Handwritten Stickers) jsou na sobě nezávislé a nezávisí ani na části B. Část B kroky 1–8 jdou napsat
a otestovat bez účtů v obchodech; spuštění pilotu čeká na RevenueCat účet a produkty v obchodech.

## 7. Pravidla, která platí pořád

- Všechna dotčená repa jsou veřejná: žádné klíče, Team ID, domovské cesty ani jména položek
  v Bitwardenu.
- Soubory do gitu přidávej jmenovitě, žádný force push. Commit zprávy anglicky.
- Odemykání vychází jen ze store transakce. Sleva není nárok: server jen říká, který produkt
  nabídnout, nárok vzniká až nákupem v obchodě.

## 8. Zdroje

- Apple, offer codes pro non-consumable: <https://wwdcnotes.com/documentation/wwdc25-241-whats-new-in-storekit-and-inapp-purchase/>
- Google Play, více nabídek u one-time produktů: <https://developer.android.com/google/play/billing/one-time-product-multi-purchase-options-offers>
- RevenueCat a slevy u one-time produktů na Play: <https://community.revenuecat.com/sdks-51/re-progress-on-one-time-product-discounts-on-the-play-store-7811>
