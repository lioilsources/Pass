## 0.1.0

- První verze. `PassService` nad RevenueCat (`purchases_flutter` 11) s
  entitlementem `all_apps` a offeringem `pass`.
- Identita bez loginu: iOS keychain access group (`flutter_secure_storage`),
  Android `PassIdentityProvider` + objevování sourozenců přes intent
  `com.ol1n.pass.IDENTITY` (signature permission).
- Google Sign-In (7.x) jako záložní identita na Androidu; link code přes
  `pass-api` (`/v1/link-code`, `/v1/redeem`).
- Katalog `apps.json` (vzdálený s bundlovaným fallbackem), hodnota unlocků.
- Widgety `PassPaywall`, `PassBadge`, `PassAppsList`; řetězce cs/en.
- `ol1n_pass_testing.dart`: `FakePassStore`, `InMemoryIdentityStore`.
