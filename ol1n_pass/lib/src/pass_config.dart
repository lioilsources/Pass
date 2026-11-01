/// Everything an app has to tell `ol1n_pass` before `PassService.init`.
///
/// Values that identify the developer account (keychain access group) are
/// parameters on purpose: the package is public, the account is not.
class PassConfig {
  const PassConfig({
    required this.appSlug,
    required this.revenueCatApiKeyIos,
    required this.revenueCatApiKeyAndroid,
    this.iosKeychainAccessGroup,
    this.kidsMode = false,
    this.apiBase = defaultApiBase,
    this.catalogUrl = defaultCatalogUrl,
    this.offeringId = defaultOfferingId,
    this.entitlementId = defaultEntitlementId,
    this.catalogTimeout = const Duration(seconds: 5),
    this.debugLogs = false,
  });

  /// Default `pass-api` base URL (see docs/CONTRACTS.md §4).
  static const String defaultApiBase = 'https://pass.ol1n.com';

  /// Default remote catalog location (see docs/CONTRACTS.md §3).
  static const String defaultCatalogUrl = 'https://olin.now/pass.json';

  /// RevenueCat offering holding the `$rc_lifetime` and `$rc_annual` packages.
  static const String defaultOfferingId = 'pass';

  /// The single entitlement every Pass product grants.
  static const String defaultEntitlementId = 'all_apps';

  /// Slug of the host app as listed in `apps.json` (e.g. `kirian`). Used to
  /// mark the host app in the paywall app list and to derive store product
  /// ids for diagnostics.
  final String appSlug;

  /// RevenueCat public SDK keys. One project, two platforms.
  final String revenueCatApiKeyIos;
  final String revenueCatApiKeyAndroid;

  /// Full keychain access group, `<AppIdentifierPrefix>com.ol1n.pass`
  /// (prefix = Team ID + dot). `null` disables the shared keychain and the
  /// identity stays local to this app — the Pass then only works through
  /// restore / link codes.
  final String? iosKeychainAccessGroup;

  /// Kids Category / Designed for Families. When true the service never
  /// starts Google Sign-In on its own; the app must call
  /// `linkGoogleAccount()` from behind its parental gate.
  final bool kidsMode;

  /// `pass-api` base URL, no trailing slash.
  final String apiBase;

  /// Remote `apps.json`; the bundled copy is the fallback.
  final String catalogUrl;

  final String offeringId;
  final String entitlementId;

  /// Remote catalog fetch timeout; the bundled copy is used when exceeded.
  final Duration catalogTimeout;

  /// Verbose RevenueCat SDK logging (debug builds only).
  final bool debugLogs;

  /// `com.ol1n.pass.lifetime.<slug>` (App Store) — see CONTRACTS §2.
  String get iosLifetimeProductId => 'com.ol1n.pass.lifetime.$appSlug';

  /// `com.ol1n.pass.yearly.<slug>` (App Store) — see CONTRACTS §2.
  String get iosYearlyProductId => 'com.ol1n.pass.yearly.$appSlug';

  /// Google Play one-time product id, identical in every app.
  static const String androidLifetimeProductId = 'pass_lifetime';

  /// Google Play subscription id, identical in every app.
  static const String androidYearlyProductId = 'pass_yearly';
}
