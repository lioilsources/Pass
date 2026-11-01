import 'dart:convert';

/// Schema major this package understands (`apps.json` → `version`).
const int kPassCatalogSchemaVersion = 1;

/// One Ol1n app as listed in `apps.json` (docs/CONTRACTS.md §3).
class PassApp {
  const PassApp({
    required this.slug,
    required this.name,
    this.iosBundleId,
    this.androidPackage,
    this.appStoreId,
    this.appStoreUrl,
    this.playStoreUrl,
    this.unlockValueUsd,
    this.unlocks = const [],
    this.kids = false,
    this.passSince,
    this.notes,
  });

  factory PassApp.fromJson(Map<String, dynamic> json) => PassApp(
        slug: json['slug'] as String,
        name: json['name'] as String,
        iosBundleId: json['ios_bundle_id'] as String?,
        androidPackage: json['android_package'] as String?,
        appStoreId: json['appstore_id'] as String?,
        appStoreUrl: _uri(json['appstore_url']),
        playStoreUrl: _uri(json['playstore_url']),
        unlockValueUsd: (json['unlock_value_usd'] as num?)?.toDouble(),
        unlocks: (json['unlocks'] as List?)?.cast<String>() ?? const [],
        kids: json['kids'] as bool? ?? false,
        passSince: json['pass_since'] as String?,
        notes: json['notes'] as String?,
      );

  final String slug;
  final String name;
  final String? iosBundleId;
  final String? androidPackage;
  final String? appStoreId;
  final Uri? appStoreUrl;
  final Uri? playStoreUrl;

  /// Sum of the app's non-consumable unlocks; null = not filled in yet.
  final double? unlockValueUsd;
  final List<String> unlocks;
  final bool kids;

  /// First app version shipping `ol1n_pass`; null = Pass not in the app yet.
  final String? passSince;
  final String? notes;

  /// The Pass already unlocks something in this app.
  bool get hasPass => passSince != null && passSince!.isNotEmpty;

  /// Listed, but the Pass is not wired into the app yet ("coming soon").
  bool get comingSoon => !hasPass;

  /// Store link for the current platform, falling back to the other store.
  Uri? storeUrl({required bool preferIos}) =>
      preferIos ? (appStoreUrl ?? playStoreUrl) : (playStoreUrl ?? appStoreUrl);

  static Uri? _uri(Object? v) =>
      v is String && v.isNotEmpty ? Uri.tryParse(v) : null;
}

/// Pass-level facts from `apps.json` → `pass`.
class PassInfo {
  const PassInfo({
    required this.name,
    required this.lifetimeUsd,
    required this.yearlyUsd,
    required this.landingUrl,
    required this.catalogUrl,
    required this.apiUrl,
  });

  factory PassInfo.fromJson(Map<String, dynamic> json) => PassInfo(
        name: json['name'] as String? ?? 'Ol1n Pass',
        lifetimeUsd: (json['lifetime_usd'] as num).toDouble(),
        yearlyUsd: (json['yearly_usd'] as num?)?.toDouble(),
        landingUrl: Uri.parse(json['landing_url'] as String),
        catalogUrl: Uri.parse(json['catalog_url'] as String),
        apiUrl: Uri.parse(json['api_url'] as String),
      );

  final String name;
  final double lifetimeUsd;
  final double? yearlyUsd;
  final Uri landingUrl;
  final Uri catalogUrl;
  final Uri apiUrl;
}

/// Thrown when `apps.json` has a schema major this package cannot read.
class PassCatalogVersionException implements Exception {
  const PassCatalogVersionException(this.found);
  final int found;

  @override
  String toString() =>
      'PassCatalogVersionException: apps.json version $found, '
      'this package reads $kPassCatalogSchemaVersion';
}

/// The parsed `apps.json`.
class PassCatalog {
  const PassCatalog({
    required this.version,
    required this.updated,
    required this.entitlement,
    required this.pass,
    required this.apps,
  });

  /// Parses the JSON document. Throws [PassCatalogVersionException] when the
  /// schema major is newer than [kPassCatalogSchemaVersion], and
  /// [FormatException] on malformed JSON or duplicate slugs.
  factory PassCatalog.parse(String source) {
    final decoded = jsonDecode(source);
    if (decoded is! Map<String, dynamic>) {
      throw const FormatException('apps.json: top level is not an object');
    }
    return PassCatalog.fromJson(decoded);
  }

  factory PassCatalog.fromJson(Map<String, dynamic> json) {
    final version = json['version'] as int? ?? 0;
    if (version > kPassCatalogSchemaVersion || version < 1) {
      throw PassCatalogVersionException(version);
    }
    final apps = ((json['apps'] as List?) ?? const [])
        .map((e) => PassApp.fromJson(Map<String, dynamic>.from(e as Map)))
        .toList(growable: false);
    final slugs = <String>{};
    for (final app in apps) {
      if (!slugs.add(app.slug)) {
        throw FormatException('apps.json: duplicate slug "${app.slug}"');
      }
    }
    return PassCatalog(
      version: version,
      updated: json['updated'] as String? ?? '',
      entitlement: json['entitlement'] as String? ?? 'all_apps',
      pass: PassInfo.fromJson(Map<String, dynamic>.from(json['pass'] as Map)),
      apps: apps,
    );
  }

  final int version;
  final String updated;
  final String entitlement;
  final PassInfo pass;
  final List<PassApp> apps;

  /// Sum of `unlock_value_usd` over the apps that have one. This is the
  /// "value of all packs today" number the paywall anchors the price with.
  double get totalUnlockValueUsd => apps.fold(
      0.0, (sum, a) => sum + (a.hasPass ? (a.unlockValueUsd ?? 0) : 0));

  /// Sum over every app with a value, wired in or not — what the landing
  /// page may show as "will be worth".
  double get totalListedValueUsd =>
      apps.fold(0.0, (sum, a) => sum + (a.unlockValueUsd ?? 0));

  /// Apps the Pass unlocks today.
  List<PassApp> get appsWithPass =>
      apps.where((a) => a.hasPass).toList(growable: false);

  /// Apps listed but not wired in yet.
  List<PassApp> get comingSoon =>
      apps.where((a) => a.comingSoon).toList(growable: false);

  PassApp? bySlug(String slug) {
    for (final a in apps) {
      if (a.slug == slug) return a;
    }
    return null;
  }
}
