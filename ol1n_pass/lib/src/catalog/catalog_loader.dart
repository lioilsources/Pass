import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart' show AssetBundle, rootBundle;
import 'package:http/http.dart' as http;

import 'pass_catalog.dart';

/// Bundled fallback copy of `catalog/apps.json`.
const String kBundledCatalogAsset = 'packages/ol1n_pass/assets/apps.json';

/// Loads `apps.json`: remote first (so new apps appear without an app
/// update), bundled copy when the network or the schema fails. The result is
/// cached in memory for the process lifetime; call [refresh] to re-fetch.
class CatalogLoader {
  CatalogLoader({
    required this.remoteUrl,
    this.timeout = const Duration(seconds: 5),
    http.Client? client,
    AssetBundle? bundle,
    this.bundledAsset = kBundledCatalogAsset,
  })  : _client = client,
        _bundle = bundle;

  final Uri remoteUrl;
  final Duration timeout;
  final String bundledAsset;
  final http.Client? _client;
  final AssetBundle? _bundle;

  PassCatalog? _cached;
  String? _lastError;

  PassCatalog? get cached => _cached;

  /// Why the remote fetch fell back, if it did.
  String? get lastError => _lastError;

  Future<PassCatalog> load() async => _cached ??= await refresh();

  /// Fetches again, ignoring the cache. Never throws when the bundled copy
  /// parses; the bundled copy is part of the package, so it does.
  Future<PassCatalog> refresh() async {
    _lastError = null;
    final remote = await _fetchRemote();
    if (remote != null) {
      _cached = remote;
      return remote;
    }
    final bundled = await _loadBundled();
    _cached = bundled;
    return bundled;
  }

  Future<PassCatalog?> _fetchRemote() async {
    final client = _client ?? http.Client();
    try {
      final response = await client.get(remoteUrl).timeout(timeout);
      if (response.statusCode != 200) {
        _lastError = 'catalog: HTTP ${response.statusCode}';
        return null;
      }
      return PassCatalog.parse(response.body);
    } catch (e) {
      _lastError = 'catalog: $e';
      return null;
    } finally {
      if (_client == null) client.close();
    }
  }

  Future<PassCatalog> _loadBundled() async {
    final bundle = _bundle ?? rootBundle;
    final source = await bundle.loadString(bundledAsset);
    try {
      return PassCatalog.parse(source);
    } on PassCatalogVersionException {
      // The bundled copy always matches this package's schema; if it does not,
      // the package was built wrong and loud is right.
      rethrow;
    } catch (e, st) {
      debugPrint('ol1n_pass: bundled apps.json unreadable: $e\n$st');
      rethrow;
    }
  }
}
