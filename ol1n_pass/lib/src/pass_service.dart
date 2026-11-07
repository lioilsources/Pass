import 'dart:async';
import 'dart:io' show Platform;

import 'package:flutter/foundation.dart';

import 'api/pass_api_client.dart';
import 'catalog/catalog_loader.dart';
import 'catalog/pass_catalog.dart';
import 'identity/android_provider_identity.dart';
import 'identity/google_signin_link.dart';
import 'identity/identity_store.dart';
import 'identity/ios_keychain_identity.dart';
import 'identity/pass_identity.dart';
import 'pass_config.dart';
import 'pass_models.dart';
import 'store/fake_pass_store.dart';
import 'store/pass_store.dart';
import 'store/revenuecat_store.dart';

/// The one object an app talks to. `PassService.instance` is wired for the
/// real platform; the named constructor takes fakes for tests and for
/// desktop dev builds.
///
/// ```dart
/// await PassService.instance.init(PassConfig(...));
/// final unlocked = hasAppUnlock || PassService.instance.hasPass.value;
/// ```
class PassService {
  PassService._platform();

  /// Fully injectable constructor. [store], [localIdentity] and
  /// [sharedIdentity] replace the platform pieces; [supported] overrides the
  /// iOS/Android check.
  @visibleForTesting
  PassService.withDependencies({
    required PassStore store,
    required IdentityStore localIdentity,
    IdentityStore? sharedIdentity,
    PassIdentitySource sharedSource = PassIdentitySource.sharedKeychain,
    PassApiClient? api,
    CatalogLoader? catalogLoader,
    GoogleSignInLink? google,
    bool supported = true,
  })  : _storeOverride = store,
        _localOverride = localIdentity,
        _sharedOverride = sharedIdentity,
        _sharedSourceOverride = sharedSource,
        _apiOverride = api,
        _catalogOverride = catalogLoader,
        _googleOverride = google,
        _supportedOverride = supported;

  static final PassService instance = PassService._platform();

  PassStore? _storeOverride;
  IdentityStore? _localOverride;
  IdentityStore? _sharedOverride;
  PassIdentitySource? _sharedSourceOverride;
  PassApiClient? _apiOverride;
  CatalogLoader? _catalogOverride;
  GoogleSignInLink? _googleOverride;
  bool? _supportedOverride;

  late PassConfig _config;
  PassStore? _store;
  PassIdentityResolver? _identity;
  PassApiClient? _api;
  CatalogLoader? _catalogLoader;
  GoogleSignInLink? _google;
  Future<void>? _initFuture;

  final ValueNotifier<bool> _hasPass = ValueNotifier<bool>(false);
  final ValueNotifier<PassState> _state = ValueNotifier<PassState>(const PassState());

  /// True while the user has the `all_apps` entitlement. Listen to it the
  /// same way as any other `ValueListenable`.
  ValueListenable<bool> get hasPass => _hasPass;

  /// Richer snapshot for UI (prices, identity, errors).
  ValueListenable<PassState> get state => _state;

  PassConfig get config => _config;

  /// The RevenueCat `app_user_id` in use, null before `init`.
  String? get appUserId => _state.value.appUserId;

  /// Parsed `apps.json`; the bundled copy until the remote one arrives.
  /// Null before `init` finished.
  PassCatalog? get catalog => _catalogLoader?.cached;

  /// iOS / Android with a store. Everything else is a no-op `false`.
  bool get supported => _supportedOverride ??
      (!kIsWeb && (Platform.isIOS || Platform.isAndroid));

  bool get isReady => _state.value.ready;

  /// Starts everything. Safe to call more than once; later calls return the
  /// first call's future.
  Future<void> init(PassConfig config) => _initFuture ??= _init(config);

  Future<void> _init(PassConfig config) async {
    _config = config;
    _catalogLoader = _catalogOverride ??
        CatalogLoader(
          remoteUrl: Uri.parse(config.catalogUrl),
          timeout: config.catalogTimeout,
        );
    _api = _apiOverride ?? PassApiClient(baseUrl: config.apiBase);

    // The catalog does not depend on the store; start it now.
    final catalogFuture = _catalogLoader!.load().catchError((Object e) {
      _setState((s) => s.copyWith(lastError: 'catalog: $e'));
      return PassCatalog(
        version: kPassCatalogSchemaVersion,
        updated: '',
        entitlement: PassConfig.defaultEntitlementId,
        pass: PassInfo(
          name: 'Ol1n Pass',
          lifetimeUsd: 49.99,
          yearlyUsd: 14.99,
          landingUrl: Uri(),
          catalogUrl: Uri.parse(config.catalogUrl),
          apiUrl: Uri.parse(config.apiBase),
        ),
        apps: const [],
      );
    });

    if (!supported) {
      // Desktop / web: no store, no identity. Keep a fake store so the UI
      // can be exercised without crashing; the Pass is never granted.
      _store = _storeOverride ?? FakePassStore();
      await catalogFuture;
      _setState((s) => s.copyWith(ready: true, supported: false, hasPass: false));
      return;
    }

    _identity = PassIdentityResolver(
      local: _localOverride ?? _platformLocalStore(config),
      shared: _sharedOverride ?? _platformSharedStore(config),
      sharedSource: _sharedSourceOverride ??
          (Platform.isIOS
              ? PassIdentitySource.sharedKeychain
              : PassIdentitySource.sharedProvider),
    );
    final resolved = await _identity!.resolve();
    _setState((s) => s.copyWith(
          supported: true,
          appUserId: resolved.appUserId,
          identitySource: resolved.appUserId.startsWith(GoogleSignInLink.prefix)
              ? PassIdentitySource.google
              : resolved.source,
        ));

    _store = _storeOverride ?? RevenueCatStore(config);
    try {
      await _store!.configure(resolved.appUserId);
      _store!.addEntitlementListener(_apply);
      _apply(await _store!.hasEntitlement());
    } catch (e) {
      _setState((s) => s.copyWith(lastError: 'store: $e'));
    }

    // Prices are cosmetic; do not hold `ready` for them.
    unawaited(_loadPrices());
    await catalogFuture;
    _setState((s) => s.copyWith(ready: true));
  }

  IdentityStore _platformLocalStore(PassConfig config) {
    if (Platform.isAndroid) {
      return AndroidLocalIdentityStore(AndroidProviderIdentityStore());
    }
    // iOS: an app-private keychain item (default access group). It survives
    // reinstalls like the shared one, but only this app sees it.
    return IosKeychainIdentityStore(
      accessGroup: null,
      accountName: '${IosKeychainIdentityStore.kService}.local',
    );
  }

  IdentityStore? _platformSharedStore(PassConfig config) {
    if (Platform.isAndroid) return AndroidProviderIdentityStore();
    // An empty string (e.g. an unset `--dart-define`) means "no shared
    // keychain", same as null: the identity stays local to this app.
    final group = config.iosKeychainAccessGroup;
    if (group == null || group.isEmpty) return null;
    return IosKeychainIdentityStore(accessGroup: group);
  }

  Future<void> _loadPrices() async {
    final store = _store;
    if (store == null) return;
    try {
      final prices = await store.fetchPrices();
      _setState((s) => s.copyWith(
            lifetimePrice: prices.lifetime,
            yearlyPrice: prices.yearly,
            offeringsLoaded: true,
          ));
    } catch (e) {
      _setState((s) => s.copyWith(lastError: 'offerings: $e'));
    }
  }

  void _apply(bool has) {
    if (_hasPass.value != has) _hasPass.value = has;
    _setState((s) => s.copyWith(hasPass: has));
  }

  void _setState(PassState Function(PassState) f) => _state.value = f(_state.value);

  PassStore _requireStore() {
    final s = _store;
    if (s == null) {
      throw StateError('PassService.init() has not been called');
    }
    return s;
  }

  /// Opens the store sheet for the lifetime Pass.
  Future<PassPurchaseResult> buyLifetime() async {
    if (!supported) return const PassPurchaseResult.error('no store on this platform');
    final r = await _requireStore().purchaseLifetime();
    if (r.isSuccess) _apply(true);
    return r;
  }

  /// Opens the store sheet for the yearly Pass.
  Future<PassPurchaseResult> buyYearly() async {
    if (!supported) return const PassPurchaseResult.error('no store on this platform');
    final r = await _requireStore().purchaseYearly();
    if (r.isSuccess) _apply(true);
    return r;
  }

  /// Restore Purchases. Apple requires this to be reachable from the UI.
  Future<bool> restore() async {
    if (!supported) return false;
    final has = await _requireStore().restore();
    _apply(has);
    return has;
  }

  /// Asks `pass-api` for a code the user can type into another app / device.
  /// Throws [PassApiException] (`no_pass` when this user has no Pass).
  Future<LinkCode> createLinkCode() {
    final id = appUserId;
    if (id == null) throw StateError('PassService.init() has not been called');
    return _api!.createLinkCode(id);
  }

  /// Redeems a code from another app and switches this app to that identity.
  /// Returns the resulting entitlement state. Throws [PassApiException]
  /// (`invalid_code`, `bad_code`, `network`).
  Future<bool> redeemLinkCode(String code) async {
    final target = await _api!.redeem(code);
    return _switchIdentity(target, PassIdentitySource.google);
  }

  /// Android: Google Sign-In → `Purchases.logIn('google:<uid>')`. Returns
  /// null when the user cancelled, otherwise the entitlement state. In
  /// `kidsMode` the app must only call this from behind its parental gate;
  /// the service never triggers it by itself.
  Future<bool?> linkGoogleAccount() async {
    if (!supported) return null;
    _google ??= _googleOverride ?? GoogleSignInLink();
    final result = await _google!.signIn();
    if (result == null) return null;
    return _switchIdentity(result.appUserId, PassIdentitySource.google);
  }

  Future<bool> _switchIdentity(String appUserId, PassIdentitySource source) async {
    final has = await _requireStore().logIn(appUserId);
    await _identity?.adopt(appUserId);
    _setState((s) => s.copyWith(appUserId: appUserId, identitySource: source));
    _apply(has);
    return has;
  }

  /// Re-fetches `apps.json` and the store prices.
  Future<void> refresh() async {
    await _catalogLoader?.refresh();
    await _loadPrices();
    _setState((s) => s); // notify catalog listeners
  }

  /// Tear down (tests). The singleton is never disposed.
  @visibleForTesting
  Future<void> dispose() async {
    _store?.removeEntitlementListener(_apply);
    await _store?.dispose();
    _api?.close();
  }
}
