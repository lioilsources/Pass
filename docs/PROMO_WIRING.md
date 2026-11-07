# PROMO_WIRING — jak Pass zapadá do PromoClown, PromoHub, Ol1nDevBot a storů

Cíl: nákup Passu, nová appka v Passu i zmínka o Passu na síti končí na stejných místech jako
všechno ostatní (Discord server Ol1n, Telegram, promo fronta), a store produkty vznikají z jednoho
souboru. Nic se nedělá dvakrát ručně.

## 1. Tok událostí

```
App Store / Play ──▶ RevenueCat ──webhook──▶ pass-api (JODA) ──▶ Discord #pass
                                                   │                 Telegram (volitelně)
                                                   └─▶ SQLite purchase_events (dedup event.id)

catalog/apps.json ──make sync-site──▶ ol1n.now/static/pass.json ──▶ paywally všech appek
                 ──passctl──────────▶ ASC + Play IAP, RevenueCat produkty

PromoHub (Apify/IFTTT) ──mention "Ol1n Pass"──▶ Telegram přes IFTTT promo_mention
PromoClown (OpenClaw) ──weekly plan──▶ drafty o Passu ──ok──▶ Postiz ──▶ sítě
```

| Událost | Zdroj | Kam |
|---|---|---|
| `INITIAL_PURCHASE`, `NON_RENEWING_PURCHASE` (lifetime), `RENEWAL` | RC webhook → `pass-api` | Discord `#pass` embed: produkt, appka (ze store product ID → slug), země, cena v USD, `app_user_id` zkrácené; Telegram stejný text |
| `CANCELLATION`, `EXPIRATION`, `BILLING_ISSUE` | totéž | Discord `#pass` s ⚠️ |
| `TRANSFER` (restore po reinstalu) | totéž | jen log, do Discordu ne — není to prodej |
| první `INITIAL_PURCHASE` v produkci | `pass-api` (počet v DB == 1) | Discord `#pass` + `@everyone`-free zpráva „první Pass prodán“; stejný tvar jako milníky Ol1nDevBotu, ale posílá ho `pass-api` (viz §4) |
| nová appka s `pass_since` | commit do `catalog/apps.json` + `make sync-site` | landing a `pass.json`; **ručně** draft do PromoClown (`promo posts draft`) — agent to v weekly planu sám uvidí z `PROJECTS.md`, když je u appky hook „Part of Ol1n Pass“ |

## 2. PromoClown

Pass je **promo projekt** jako každá appka: záznam `pass` v `PromoClown/projects.yaml` (přidaný,
`status: paused`, dokud se nedá koupit). Po importu (`promo projects import projects.yaml`, admin
token) ho agent vidí v `PROJECTS.md` a weekly plan s ním počítá.

Hooks (co agent smí říkat):
- One price, 49.99 USD, once. It unlocks every one-time unlock in every one of my apps.
- Every new app I ship is in the Pass from its first version — the Pass grows, the price does not.
- Works across iPhone, iPad and Android; buy once in any app.
- Family Sharing on the App Store.
- No account on iPhone: the apps share it through the keychain.

Forbidden claims (promo-api je odmítne s 422):
- jméno jakékoli **budoucí** appky nebo datum jejího vydání (Apple 2.3 metadata accuracy platí i
  pro store texty, a sliby na sítích se vrací v recenzích),
- „free“, „trial“, „subscription“ u lifetime varianty (a „lifetime“ u roční),
- „unlimited“, „everything“ bez „in my apps“ (Pass neodemyká consumables ani cizí appky),
- srovnání s konkurencí a jakákoli čísla (prodeje, hodnota) kromě 49,99 USD a hodnoty, kterou ukazuje
  `pass.json` ten den,
- „on Steam“, „on Windows“ — Pass je store IAP, desktop buildy jsou mimo.

Cross-promo zdarma: paywall a `PassBadge` v každé appce ukazují seznam ostatních appek se store
odkazy (plán §1). To je nejlevnější promo kanál, který máme — každá appka s Passem je reklama na
ostatní. Důsledek pro PromoClown: posty o Passu nemusí vyjmenovávat appky, stačí odkaz na
`olin.now/pass`, kde je seznam živý.

Až Pass půjde koupit: `status: active`, `store_ios_url` nechat prázdné (Pass nemá vlastní store
stránku — odkaz je `website_url`), u každé appky s Passem přidat do `hooks` „Part of Ol1n Pass
(olin.now/pass)“. Po změně `promo projects import projects.yaml` a refresh `PROJECTS.md`.

## 3. PromoHub

- `PromoHub/apps.yaml`: položka `pass` s `keywords: [ol1n pass, olin pass]` (přidaná). Zmínka na
  síti, která obsahuje „Ol1n Pass“, se tak přiřadí k `app: pass` a projde pravidlem `mention`.
- `PromoHub/rules.yaml`: regex pravidla `mention` doplněn o `Ol1n Pass` (přidáno). Apify tasky
  `x-mentions` a `reddit-mentions` musí mít „Ol1n Pass“ i v **search terms** (RUNBOOK PromoHubu
  §2.2) — to je ruční krok v Apify konzoli, pravidlo samo zmínku nenajde.
- Budoucí collector (návrh, nepsat teď): `pass-api` by mohl vystavit `GET /v1/stats` (počty
  `purchase_events` po dnech a produktech, bez osobních dat) a PromoHub by ho četl jako
  `metric.snapshot` se `scope: pass` — pak by pravidlo `follower_milestone` mělo sourozence
  `pass_milestone` (`sales crosses [1, 10, 100]`) a hlásilo přes IFTTT jako ostatní metriky. Do té
  doby milníky hlásí `pass-api` sám (§1).

## 4. Ol1nDevBot

- Pass **není appka**: žádný záznam v `Ol1nDevBot/apps.yaml`. Bot sleduje verze a stažení appek;
  Pass se ho týká jen tím, že každá appka s Passem je appka jako každá jiná.
- Kanál `#pass`: založit ručně v kategorii INFRA (nebo vlastní „Pass“) a webhook dát `pass-api`.
  Reconciler bota kanály jen přidává/archivuje podle `apps.yaml`, ruční kanál mu nevadí. Poznámka
  pro fázi 4 bota: až bude mít generické `events` přes HTTP, může `pass-api` posílat tam místo
  přímo na Discord webhook; dnes je webhook jednodušší a drží invariant „bot externí API jen čte“.
- Milníky: Ol1nDevBot má `downloads_N`, `first_release`, `v1_0`. „Prvních N Passů“ je stejný tvar,
  ale zdroj dat je `pass-api`, ne ASC/Play → posílá ho `pass-api` (§1). Až bude `GET /v1/stats`
  (§3), může bot číst odtud a hlásit to sám.

## 5. Story (ASC, Play, RevenueCat)

Jediný ruční vstup je `catalog/apps.json`; zbytek dělá `passctl` idempotentně:

```
passctl catalog check                      # před každým apply
passctl asc  products plan|apply --app <slug>
passctl play products plan|apply --app <slug>
passctl rc   products apply                # RC produkty + přiřazení k all_apps
passctl rc   check                         # co kde chybí (tabulka store × RC × katalog)
```

Kdy: **před submitem verze appky, která `ol1n_pass` poprvé má** — IAP musí existovat, aby šel
zařadit do review spolu s binárkou (ASC), a Play produkt jde aktivovat až s buildem v internal
tracku. Pořadí je v [RUNBOOK §2–3](RUNBOOK.md). Klíče: `ASC_*` (stejný klíč jako CI nebo PromoClown
`store-reviews` — oba mají právo na IAP? CI klíč má App Manager, to stačí; `store-reviews` klíč je
Customer Support a **nestačí**), `GOOGLE_PLAY_SERVICE_ACCOUNT_JSON` (service account s *Manage
in-app products*), `RC_SECRET_API_KEY` + `RC_PROJECT_ID`. Zdroje v `Distribution/SKILL.md`.

Co `passctl` nedělá a nikdy nebude: mazat produkty, měnit ceny existujících, submitovat do review,
nahrávat review screenshoty.

## 6. olin.now

- `/pass` (cs) a `/pass.en` (en) jsou standalone stránky z `site/` v tomto repu; `/pass.json` je
  `catalog/apps.json` servírovaný ze `static/`. Všechny tři kopíruje `make sync-site`; v `ol1n.now`
  se **needitují** (jeho CLAUDE.md to říká).
- Publikace: v `ol1n.now` `git add pages/pass.html pages/pass.en.html assets/css/pass.css static/pass.json`
  → commit → push → `make deploy` (deploy staví z `origin/main`, lokální dist se nikam nedostane).
  Po deployi `curl -s https://olin.now/pass.json | jq .updated`.
- Paywally appek čtou `pass.json` z webu s fallbackem na bundlovanou kopii, takže deploy webu je
  součást releasu appky (INTEGRATION §10).
- Store description každé appky s Passem: jedna věta „Part of Ol1n Pass — one purchase unlocks all
  my apps: olin.now/pass“ (Apple povoluje odkaz na vlastní web; ne jmenovat budoucí appky).
- OG obrázek pro `/pass`: `make og` v ol1n.now až po první publikaci (potřebuje Chrome).

## 7. Checklist „Pass je živý“

- [ ] RC webhook → `#pass` ověřený test eventem (RUNBOOK §4)
- [ ] PromoClown `projects.yaml` `pass` → `status: active`, hooks appek doplněné, import
- [ ] PromoHub Apify tasky mají „Ol1n Pass“ v search terms
- [ ] olin.now `/pass` nasazené, `pass.json` ukazuje `pass_since` u pilotních appek
- [ ] Store descriptions pilotních appek mají větu o Passu
- [ ] Product Hunt / Show HN draft připravený ručně (PROMO_ACCOUNTS §2 — ne z agenta)
