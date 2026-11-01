# PROMO_ACCOUNTS — guideline pro promo účty portfolia Ol1n

Jak založit a pojmenovat účty na sítích a v obchodech tak, aby je šlo z jednoho místa řídit
(PromoClown, PromoHub, Postiz, IFTTT), aby se neztratily a aby jim nic nebránilo v automatizaci.
Platí pro **celé portfolio** (olin.now), ne jen pro Pass. Konkrétní stav účtů se neeviduje tady, ale
v Bitwardenu a v `PromoHub/accounts.yaml`.

## 1. Společná identita

| Co | Pravidlo |
|---|---|
| Handle | `ol1n` všude, kde je volný. Fallback v tomto pořadí: `ol1n_apps`, `olin_now`, `ol1napps`. Jednou zvolený fallback zapiš do tabulky v §4 — ať ho další platformy opakují, ne vymýšlejí nový. |
| Zobrazované jméno | „Ol1n“. Na B2B platformách (LinkedIn) „Olin Vořechovský“ s Company Page „Ol1n“. |
| Bio (EN, ≤ 160 znaků) | `Indie dev from Czechia. Games and apps in Flutter, backends in Go, AI on my own GPU. Everything: olin.now` — platformy s delším limitem přidají jednu větu o Passu: `One purchase unlocks all my apps: olin.now/pass`. |
| Bio (CZ, kde je publikum české) | `Nezávislý vývojář z Česka. Hry a appky ve Flutteru, servery v Go, AI na vlastním GPU. Všechno: olin.now` |
| Avatar | Logo z `ol1n.now/assets/img/ananas-bananas-logo.png` oříznuté na čtverec, exportovat 1024×1024 PNG jednou a používat **ten samý soubor** všude. Ulož ho do `PromoClown/assets/brand/avatar.png` (repo PromoClown má `assets/<slug>/`). |
| Banner | Jeden 1500×500 (X/Bluesky/LinkedIn) a jeden 2560×1440 (YouTube) s wordmarkem + „olin.now“. Stejná sada barev jako web (`--accent #2f7d32`, krém `#f8f6e9`). |
| Link v bio | **Vždy `https://olin.now`**, nikdy konkrétní appka. Důvod: X účtuje odkaz v postu (0,20 USD vs 0,015 USD bez odkazu — viz komentář v `PromoClown/projects.yaml`), takže posty odkazují „link in bio“; bio musí vést na místo, které ukáže všechno. Pass má na webu vlastní `/pass`. |
| E-mail | Per platforma alias přes Cloudflare Email Routing na doméně **olin.now**: `yt@`, `tiktok@`, `reddit@`, `bsky@`, `x@`, `ig@`, `mastodon@`, `linkedin@`, `ph@`, `itch@`, `dev@`. Catch-all vypnutý (stejně jako `business@`). Když alias začne chodit spam nebo přijde phishing „reset hesla“, vidíš hned, odkud únik je, a alias zrotuješ bez změny ostatních účtů. |
| Hesla a 2FA | Bitwarden položka `Promo / <Platform>` s heslem, TOTP (Bitwarden autentikátor, ne SMS — SIM swap), recovery kódy v poznámce a datem založení. SMS 2FA jen tam, kde platforma TOTP nemá (X vyžaduje telefon při registraci). |
| Telefon | Jedno číslo pro všechny registrace (hodně platforem ho chce). Napiš do Bitwarden poznámky `Promo / _common`, ať je jasné, které to je. |
| Jazyk | EN default, CZ jen pro české komunity — stejně jako `PromoClown/workspace/PROMO_RULES.md`. |

## 2. Platformy

Legenda stavu: ✔ založit teď · ⏳ později (až bude důvod) · ✖ ne. „Kam zapsat“ říká, kde si
automatizace účet najde.

| Platforma | Stav | Proč | Typ účtu a co nastavit kvůli automatizaci | Kam zapsat |
|---|---|---|---|---|
| **YouTube** | ✔ | Shorts z gameplayů, trailery; jediná platforma, kde video žije dlouho. PromoClown už publikuje přes Postiz. | **Brand channel** pod Google účtem portfolia (ne osobní kanál — brand kanál jde sdílet a převést). Handle `@ol1n`. Google Cloud projekt `ol1n-promo`: zapnout **YouTube Data API v3** (+ Analytics API), OAuth consent screen v režimu *Testing* s tvým účtem jako testerem (stačí pro vlastní kanál, bez verifikace), OAuth client *Desktop* pro Postiz a API key pro `youtube-comments`. | `PromoHub/accounts.yaml` (`platform: youtube`), Postiz integrace YouTube, PromoClown `promo.env` `YOUTUBE_API_KEY`, IFTTT služba YouTube (trigger „New video by you“). |
| **TikTok** | ✔ | Shorts recyklované; publikum pro casual hry (Puff, Kirian). | **Business account** (má analytics a je podmínka Content Posting API). Hned po založení **TikTok for Developers** → app → žádost o **Content Posting API** (audit trvá týdny; plán PromoHub §2 ho dává do fáze 5 — ale žádost nic nestojí a čekání běží). Do schválení se postuje ručně, monitoring přes Apify task `tiktok-profile`. | `PromoHub/accounts.yaml` (`platform: tiktok` + `apify_tasks[].tiktok-profile`). |
| **Reddit** | ✔ | Jediné místo, kde se vývojářské appky opravdu diskutují (r/FlutterDev, r/golang, r/indiegames, r/SideProject, r/iosapps, r/androidapps, r/czech). | Osobní účet `ol1n`, **karma budovat ručně** 2–3 týdny před prvním promo (odpovídej na cizí otázky). Reddit **script app** (preferences → apps → script, redirect `http://localhost`) jen `read` + `privatemessages` pro `reddit-monitor`. Retence 48 h z Reddit Data API policy drží PromoClown i PromoHub. Zapiš si pravidla self-promo každého subredditu do `PromoClown/workspace/PROMO_RULES.md` (sekce Reddit). | PromoClown `promo.env` `REDDIT_CLIENT_ID/SECRET/USERNAME/PASSWORD`, PromoHub Apify `reddit-mentions`. |
| **Bluesky** | ✔ | Levné, API zdarma, tech publikum; PromoClown už postuje. | Účet `ol1n.bsky.social`, pak **vlastní handle `olin.now`**: Settings → Change handle → „I have my own domain“ → DNS TXT `_atproto.olin.now` = `did=did:plc:…` (Cloudflare, proxy off) → Verify. Handle = doména je nejlepší ověření identity, které Bluesky má. **App password** (Settings → App passwords) zvlášť pro Postiz a zvlášť pro `bluesky-mentions`, aby šly revokovat nezávisle. | Postiz integrace Bluesky, PromoClown `promo.env` `BLUESKY_HANDLE/APP_PASSWORD`, PromoHub `accounts.yaml` `platform: bluesky`, IFTTT Bluesky („New post by you“). |
| **X** | ✔ | Pořád největší dosah pro indie dev; Tsumiki tam už má účet. | Účet `ol1n` (telefon povinný). **Developer account Free tier** — jen write (1 500 postů/měsíc), čtení je placené → monitoring přes Apify `x-profile` / `x-mentions`. V appce zapnout OAuth 2.0 s *Read and write*, callback podle Postiz. Odkazy v postech zpoplatněné → **link in bio**. | Postiz integrace X, PromoHub `accounts.yaml` `platform: x` + Apify tasky, IFTTT X („Post a tweet“) pro PromoHub `publish_x`. |
| **Instagram** | ⏳ | Vizuální appky (Tsumiki, Webtoon, MirrorBooth). Bez Business účtu nejde ani Graph API, ani IFTTT trigger. | **Business/Creator** účet, propojený s **Facebook Page** „Ol1n“ (Page je nutná, i když se nepoužívá). Meta app `ol1n-promo` s `instagram_basic`, `instagram_content_publish`, `pages_read_engagement`; App Review pro publish až ve fázi 4 PromoHubu. Založ účet + Page teď (zabrat handle), API později. | PromoHub `accounts.yaml` `platform: instagram`, IFTTT Instagram („New photo/video by you“). |
| **Threads** | ⏳ | Přichází zdarma s IG účtem; Threads API publikuje přes stejnou Meta app. | Zapnout z IG účtu, handle se zdědí. Nic dalšího do fáze 4. | Postiz (má Threads provider). |
| **Mastodon** | ✔ | Open-source publikum (DoggioWars, Kirian desktop buildy, Go nástroje); API zdarma bez žádostí; Postiz ho umí. | `fosstodon.org` (FOSS komunita, odpovídá tomu, co se tam bude postovat) spíš než `mastodon.social`. Handle `@ol1n@fosstodon.org`. Profile → Development → New application (`write:statuses`, `read`) pro Postiz. Verified link: v profilu přidat `https://olin.now` a na web `<a rel="me" href="https://fosstodon.org/@ol1n">` (do `templates/foot.html` v ol1n.now). | Postiz integrace Mastodon, PromoHub `accounts.yaml` `platform: mastodon` (collector zatím není — jen handle). |
| **LinkedIn** | ⏳ | B2B část (business.olin.now): agentury, e-shopy, firmy s GPU. Ne pro hry. | Osobní profil (už existuje?) + **Company Page „Ol1n“**; posty jdou z osobního profilu, Page je pro „works at“. LinkedIn API pro Page posting vyžaduje Marketing Developer Platform → ručně, Postiz to umí přes vlastní app. Založ Page, postuj ručně 1×/měsíc. | — (ruční). |
| **Discord** | ✔ (existuje) | Komunita a support; server Ol1n spravuje Ol1nDevBot. | Public invite link bez expirace, vanity není (potřebuje boost). Přidat invite na `olin.now` do patičky a do store descriptions, kde to dává smysl (Kirian co-op). Kanál `#pass` pro `pass-api` ([RUNBOOK §4](RUNBOOK.md)). | PromoHub `rules.yaml` sink `discord:` (webhook), `pass-api` `DISCORD_WEBHOOK_URL`. |
| **Product Hunt** | ⏳ | Launch moment per appka (Pass sám je dobrý launch: „one purchase, all my apps“). | **Maker profil** pod jménem Olin Vořechovský (PH je osobní), založit teď a sledovat 2–3 týdny (PH váží aktivitu). Launch připravit s PromoClown draftem a vlastním vizuálem; jeden launch za měsíc max. | — |
| **itch.io** | ✔ | Desktop buildy her (Kirian macOS/Windows/Linux, Archion) zdarma hostované, vlastní store page, devlog; publikum, které platí za indie. | Účet `ol1n`, Creator dashboard, **Butler** CLI (`butler push`) do release workflow — jedna řádka vedle GitHub Release v `Distribution/workflows/release-*.yml`. API key do Bitwardenu `Promo / itch.io`. | `ol1n.now/apps/<slug>/meta.md` nový klíč `itch:` (analog `contentdb:`) — vyžaduje malou úpravu `build-site.sh`. |
| **Steam** | ⏳ | 100 USD per appka (vratné po 1 000 USD tržeb), review, Steamworks integrace. Smysl jen pro Kirian/Archion s desktop verzí a trailerem. | Partner účet až po itch.io, když tam buildy někdo stahuje. | — |
| **Luanti ContentDB** | ✔ (existuje) | DoggioWars; `lioilsources/doggiowars_game`. | Nastavit `CONTENTDB_TOKEN` secret v repu DoggioWars (ol1n.now CLAUDE.md to popisuje) — jediná chybějící věc. | — |
| **App Store — developer page** | ✔ (existuje) | `apps.apple.com/developer/…` je de facto portfolio stránka; Pass bude v popisech appek odkazovat „all apps by Ol1n“. | Seller name je dnes **„Olin Vorechovsky“**, na Play je developer **„A+B“** a staré Play package `com.ab.*` — nesoulad, který zákazník vidí přesně v momentě, kdy Pass říká „všechny appky od Ol1na“. Návrh: sjednotit na **„Ol1n“** jako developer name na obou storech. Apple: změna názvu dev účtu individuálního vývojáře jde jen přes podporu a ne vždy (legal name musí sedět); na Play je **Developer name** volně editovatelné v Play Console → Developer page. Udělej aspoň Play hned; u Apple otevři ticket a počítej s „ne“. | `catalog/apps.json` nic; `ol1n.now` meta.md store odkazy beze změny. |
| **Google Play — developer page** | ✔ (existuje) | Play Console → Developer page: ikona, header, website `olin.now`, e-mail `play@olin.now`. | Developer name „Ol1n“ (viz výše). Vyplnit „Featured app“ (Kirian). | — |
| **GitHub** | ✔ (existuje) | `lioilsources` je zdroj buildů pro olin.now. | Profile README `lioilsources/lioilsources` s odkazem na olin.now a seznamem veřejných appek (generovat z `ol1n.now/apps/*/meta.md` — jednořádkový skript, ne ručně). Sponsors až bude důvod. | — |
| **Pinterest** | ✖ (zatím) | Dává smysl jen pro vizuální výstupy (Tsumiki restyly, Webtoon panely), a i tam až s objemem. | — | — |
| **Twitch** | ⏳ | Plán PromoClown v2: oznámení streamů, VOD → Shorts. Bez streamování zbytečné. | Zabrat handle `ol1n` teď (zdarma), nic dalšího. | — |
| **Hacker News** | ✔ (osobní) | „Show HN“ pro Pass, pro Archion, pro Go nástroje. HN nemá značkové účty — účet je osobní a karma se buduje komentováním. | Účet `ol1n`, bio s odkazem. Nikdy neposílat z PromoClown — HN posty píšeš ručně, agent max. navrhne text. | — |
| **Indie Hackers** | ⏳ | Milestones a revenue posty (Pass je přesně IH obsah). Až budou čísla, která nejsou vymyšlená (`Dev/README.md` pravidlo). | Účet `ol1n`, produkt „Ol1n Pass“. | — |
| **Dev.to / Medium** | ⏳ | Behind-the-scenes (keychain sharing napříč appkami, RevenueCat multi-app, Flutter + Go) — EN články s odkazem na repo Pass. Dev.to má RSS import a API; Medium ne. | Dev.to `ol1n`, canonical URL na vlastní blog, pokud vznikne; jinak Dev.to jako primární. | — |
| **Newsletter** | ⏳ | „Nová appka vyšla, Pass ji odemkl“ je přirozený newsletter pro držitele Passu. Buttondown (zdarma do 100 odběratelů, API) nebo Listmonk na JODĚ (3,8 GB RAM — Listmonk je lehký, ale JODA je plná; Buttondown napřed). | Až bude Pass prodejný. Sign-up form na `/pass`. | — |

## 3. Den 1 — pořadí (≈ 20 minut na platformu)

Pořadí podle toho, co odblokuje automatizaci nejdřív a co má nejdelší čekání:

1. **Cloudflare Email Routing** — všechny aliasy z §1 najednou (5 min), jinak každá další registrace čeká.
2. **Bitwarden** — složka `Promo`, položka `_common` (telefon, avatar cesta, pravidla handlu).
3. **Google účet portfolia** (`yt@olin.now`) → **YouTube brand channel** `@ol1n` → **Google Cloud
   projekt `ol1n-promo`**, YouTube Data API + OAuth consent (Testing). Tenhle Google účet je zároveň
   základ pro Play Console service account a pro Firebase, pokud se někdy sjednotí.
4. **TikTok Business** → **TikTok for Developers** žádost o Content Posting API (nejdelší čekání).
5. **X** + developer Free tier (telefon).
6. **Bluesky** → vlastní handle `olin.now` (DNS TXT) → dva app passwordy.
7. **Reddit** `ol1n` → script app → začni budovat karmu (žádné promo prvních 2–3 týdny).
8. **Mastodon** `fosstodon.org` → aplikace pro Postiz → `rel="me"` na web.
9. **itch.io** → Butler API key.
10. **Instagram Business + Facebook Page** (zabrat handle, API až fáze 4) → Threads.
11. **Product Hunt** maker profil, **Twitch** a **HN** handle (jen zabrat).
12. **Play Console Developer page** → developer name „Ol1n“, website, e-mail; **Apple** ticket na
    seller name.
13. **Discord** invite link na web; kanál `#pass`.
14. Zapiš všechno do §4 a do `PromoHub/accounts.yaml`; Postiz integrace propoj (YouTube, X, Bluesky,
    Mastodon) — OAuth callbacky potřebují veřejný hostname Postizu (PromoClown RUNBOOK).

## 4. Co zapsat kam

### `PromoHub/accounts.yaml`
```yaml
accounts:
  - platform: x
    handle: ol1n            # nebo zvolený fallback — stejný řetězec, jaký ukazuje URL profilu
  - platform: tiktok
    handle: ol1n
  - platform: youtube
    handle: ol1n            # handle bez @; channel ID si collector dohledá
  - platform: bluesky
    handle: olin.now        # po přepnutí na vlastní doménu; do té doby ol1n.bsky.social
  - platform: instagram
    handle: ol1n
  - platform: mastodon
    handle: ol1n@fosstodon.org
```
`apify_tasks[].account` musí odkazovat stejným řetězcem (`x:ol1n`, `tiktok:ol1n`).

### PromoClown `deploy/spark/promo.env` (jen názvy, hodnoty do `.env` na Sparku)
`YOUTUBE_API_KEY` (read-only), `BLUESKY_HANDLE` + `BLUESKY_APP_PASSWORD` (app password pro
*monitoring*, jiný než pro Postiz), `REDDIT_CLIENT_ID/SECRET/USERNAME/PASSWORD` (script app, read),
`ASC_*` a `GOOGLE_PLAY_*` (už jsou). Zapisovací klíče (Postiz API key, X write) **nikdy na Spark**
— PromoClown invariant.

### Postiz (JODA/Spark, `social.ol1n.com`)
Integrace: YouTube (OAuth brand channel), X (OAuth 2.0 app), Bluesky (app password #1),
Mastodon (app token). Per projekt v `PromoClown/projects.yaml` jde určit účet (`postiz_accounts:`,
jako má `tsumiki`); default jsou účty portfolia.

### IFTTT (PromoHub RUNBOOK §3)
Služby: YouTube, Bluesky, Instagram (business), X (akce „Post a tweet“), Telegram, Discord.
Přihlašuj přes účty portfolia, ne osobní.

### Tabulka skutečných handlů (vyplnit při zakládání)

| Platforma | Handle | URL | Alias | Bitwarden | API/dev účet | Založeno |
|---|---|---|---|---|---|---|
| YouTube | | | yt@ | Promo / YouTube | GCP `ol1n-promo` | |
| TikTok | | | tiktok@ | Promo / TikTok | TikTok for Developers, žádost CP API dne … | |
| Reddit | | | reddit@ | Promo / Reddit | script app | |
| Bluesky | | | bsky@ | Promo / Bluesky | app pw: postiz, monitor | |
| X | | | x@ | Promo / X | Free tier app | |
| Instagram + FB Page | | | ig@ | Promo / Instagram | Meta app | |
| Mastodon | | | mastodon@ | Promo / Mastodon | app token | |
| itch.io | | | itch@ | Promo / itch.io | Butler key | |
| Product Hunt | | | ph@ | Promo / Product Hunt | — | |
| Twitch / HN / IH / Dev.to | | | dev@ | Promo / <Platform> | — | |

## 5. Bezpečnost

- 2FA všude, TOTP v Bitwardenu, recovery kódy v poznámce položky. Žádné SMS, kde to jde.
- **Žádné zapisovací tokeny na Sparku.** Agent (PromoClown) má jen read-only klíče; publikace jde
  přes promo-api na JODĚ po schválení v Telegramu. Platí i pro nové platformy: Mastodon app token
  s `write:statuses` patří do Postizu, ne do `promo.env` na Sparku.
- Oddělené app passwordy / tokeny per spotřebitel (Postiz vs. monitor), aby se revokace jednoho
  nedotkla druhého.
- Rotace: tokeny 1× ročně nebo při odchodu služby; e-mail alias při prvním spamu. Datum založení
  a poslední rotace v Bitwarden poznámce.
- Phishing „reset hesla“ chodí na alias platformy — cokoli na jiný alias je podvod. To je hlavní
  důvod aliasů per platforma.
- Veřejná repa (Pass, PromoClown, PromoHub) neobsahují handly s tokeny ani webhook URL; handly samotné
  jsou veřejné a do `accounts.yaml` patří.
