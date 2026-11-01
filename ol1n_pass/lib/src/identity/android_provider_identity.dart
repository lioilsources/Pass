import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';

import 'identity_store.dart';

/// Android: the id is kept in this app's SharedPreferences (`ol1n_pass`) and
/// exposed to sibling Ol1n apps through `PassIdentityProvider`
/// (`content://<package>.ol1npass/identity`, signature permission).
///
/// `read()` asks the Kotlin side to look at the other installed Ol1n apps
/// (discovered by the `com.ol1n.pass.IDENTITY` intent action) and return the
/// first id it finds. `write()` stores locally — which is exactly what makes
/// it visible to the next app through *our* provider.
class AndroidProviderIdentityStore implements IdentityStore {
  AndroidProviderIdentityStore({MethodChannel? channel})
      : _channel = channel ?? const MethodChannel(channelName);

  static const String channelName = 'ol1n_pass/identity';

  final MethodChannel _channel;

  @override
  Future<String?> read() async {
    try {
      final v = await _channel.invokeMethod<String>('readShared');
      return (v == null || v.isEmpty) ? null : v;
    } on MissingPluginException {
      return null; // not Android, or plugin not registered (tests)
    } on PlatformException catch (e) {
      debugPrint('ol1n_pass: readShared failed: ${e.message}');
      return null;
    }
  }

  @override
  Future<void> write(String id) async {
    try {
      await _channel.invokeMethod<void>('writeLocal', {'id': id});
    } on MissingPluginException {
      // ignore
    } on PlatformException catch (e) {
      debugPrint('ol1n_pass: writeLocal failed: ${e.message}');
    }
  }

  /// This app's own stored id (no cross-app lookup).
  Future<String?> readLocal() async {
    try {
      final v = await _channel.invokeMethod<String>('readLocal');
      return (v == null || v.isEmpty) ? null : v;
    } on MissingPluginException {
      return null;
    } on PlatformException catch (e) {
      debugPrint('ol1n_pass: readLocal failed: ${e.message}');
      return null;
    }
  }
}

/// Local-only view over the same channel, so the resolver can treat
/// "my own prefs" and "other apps" as two stores with distinct sources.
class AndroidLocalIdentityStore implements IdentityStore {
  AndroidLocalIdentityStore(this._provider);
  final AndroidProviderIdentityStore _provider;

  @override
  Future<String?> read() => _provider.readLocal();

  @override
  Future<void> write(String id) => _provider.write(id);
}
