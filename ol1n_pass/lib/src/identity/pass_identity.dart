import 'dart:math';

import '../pass_models.dart';
import 'identity_store.dart';

/// A resolved identity: the id plus where it came from.
class ResolvedIdentity {
  const ResolvedIdentity(this.appUserId, this.source);
  final String appUserId;
  final PassIdentitySource source;
}

/// Decides which `app_user_id` this app uses for RevenueCat.
///
/// Order (plan §2.2/2.3): the app's own stored id → an id shared by sibling
/// Ol1n apps on the device → a new UUID v4. Whatever wins is written to every
/// store that does not have it yet, so the next app finds it.
class PassIdentityResolver {
  PassIdentityResolver({
    required this.local,
    this.shared,
    this.sharedSource = PassIdentitySource.sharedKeychain,
    Random? random,
  }) : _random = random ?? Random.secure();

  final IdentityStore local;

  /// Cross-app store; null on platforms without one (desktop).
  final IdentityStore? shared;

  /// Reported when the id came from [shared].
  final PassIdentitySource sharedSource;

  final Random _random;

  Future<ResolvedIdentity> resolve() async {
    final mine = await local.read();
    if (mine != null) {
      // Make sure siblings can see it (first launch after adding the shared
      // store, or the shared store was wiped).
      if (shared != null && await shared!.read() == null) {
        await shared!.write(mine);
      }
      return ResolvedIdentity(mine, PassIdentitySource.local);
    }
    if (shared != null) {
      final theirs = await shared!.read();
      if (theirs != null) {
        await local.write(theirs);
        return ResolvedIdentity(theirs, sharedSource);
      }
    }
    final fresh = generateUuidV4(_random);
    await local.write(fresh);
    await shared?.write(fresh);
    return ResolvedIdentity(fresh, PassIdentitySource.generated);
  }

  /// Replaces the id everywhere (after Google Sign-In or a redeemed code), so
  /// sibling apps follow this app to the Pass holder's identity.
  Future<void> adopt(String appUserId) async {
    await local.write(appUserId);
    await shared?.write(appUserId);
  }

  /// RFC 4122 version-4 UUID, lower-case, from [random].
  static String generateUuidV4(Random random) {
    final b = List<int>.generate(16, (_) => random.nextInt(256));
    b[6] = (b[6] & 0x0f) | 0x40; // version 4
    b[8] = (b[8] & 0x3f) | 0x80; // variant 10xx
    String hex(int i) => b[i].toRadixString(16).padLeft(2, '0');
    final h = List<String>.generate(16, hex).join();
    return '${h.substring(0, 8)}-${h.substring(8, 12)}-${h.substring(12, 16)}-'
        '${h.substring(16, 20)}-${h.substring(20)}';
  }

  static final RegExp uuidV4 = RegExp(
      r'^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$');
}
