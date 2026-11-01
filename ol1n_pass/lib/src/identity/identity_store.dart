/// A place a `pass_user_id` can be read from and written to.
///
/// Two kinds exist: the app's **local** store (always writable) and a
/// **shared** store other Ol1n apps on the device can see (iOS keychain
/// access group, Android identity provider). Shared stores may be read-only
/// from the resolver's point of view (another app owns the value) — writes
/// are best-effort and never throw.
abstract class IdentityStore {
  /// Stored id, or null when none.
  Future<String?> read();

  /// Persists [id]. Best-effort: implementations swallow platform errors.
  Future<void> write(String id);
}
