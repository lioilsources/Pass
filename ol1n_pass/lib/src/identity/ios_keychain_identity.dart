import 'package:flutter/foundation.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

import 'identity_store.dart';

/// Keychain item `pass_user_id` in the access group shared by all Ol1n apps.
///
/// The app must declare the group in `Runner.entitlements`
/// (`keychain-access-groups` → `$(AppIdentifierPrefix)com.ol1n.pass`). With
/// [accessGroup] null the item lives in the app's own keychain — still
/// survives reinstalls, but other apps cannot see it.
class IosKeychainIdentityStore implements IdentityStore {
  IosKeychainIdentityStore({
    required this.accessGroup,
    this.accountName = kService,
    FlutterSecureStorage? storage,
  }) : _storage = storage ??
            FlutterSecureStorage(
              iOptions: IOSOptions(
                groupId: accessGroup,
                accountName: accountName,
                // The paywall is UI, so the item never has to be readable
                // before first unlock; `unlocked` keeps it off that path.
                accessibility: KeychainAccessibility.unlocked,
                synchronizable: false,
              ),
            );

  static const String kKey = 'pass_user_id';
  static const String kService = 'com.ol1n.pass';

  final String? accessGroup;

  /// `kSecAttrService`; the app-private copy uses `com.ol1n.pass.local`.
  final String accountName;
  final FlutterSecureStorage _storage;

  @override
  Future<String?> read() async {
    try {
      final v = await _storage.read(key: kKey);
      return (v == null || v.isEmpty) ? null : v;
    } catch (e) {
      debugPrint('ol1n_pass: keychain read failed: $e');
      return null;
    }
  }

  @override
  Future<void> write(String id) async {
    try {
      await _storage.write(key: kKey, value: id);
    } catch (e) {
      // Typical cause: access group not in the entitlements (-34018).
      debugPrint('ol1n_pass: keychain write failed: $e');
    }
  }
}
