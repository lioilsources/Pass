import '../pass_models.dart';

/// Localized prices of the two Pass packages, null when the store has no
/// such package in the offering (misconfiguration) or is unreachable.
class PassPrices {
  const PassPrices({this.lifetime, this.yearly});
  final String? lifetime;
  final String? yearly;

  static const PassPrices none = PassPrices();
}

/// Called with "has `entitlementId`" whenever the backend reports a change.
typedef PassEntitlementListener = void Function(bool hasEntitlement);

/// What `PassService` needs from the entitlement backend. The production
/// implementation is [RevenueCatStore]; tests use `FakePassStore`.
///
/// All methods may be called only after [configure]. Implementations must not
/// throw for "no store on this platform" — they report `false` / `none`.
///
/// Change notification is a plain listener list rather than a `Stream`: a
/// broadcast `StreamController` cannot be closed under `FakeAsync` (widget
/// tests) once a subscription was cancelled, and the service only ever needs
/// one synchronous callback anyway.
abstract class PassStore {
  /// Starts the SDK for [appUserId]. Idempotent.
  Future<void> configure(String appUserId);

  /// Fires on purchase, restore, login and background refresh.
  void addEntitlementListener(PassEntitlementListener listener);

  void removeEntitlementListener(PassEntitlementListener listener);

  /// Current truth from the backend (network or SDK cache).
  Future<bool> hasEntitlement();

  Future<PassPrices> fetchPrices();

  Future<PassPurchaseResult> purchaseLifetime();

  Future<PassPurchaseResult> purchaseYearly();

  /// Re-deliver past purchases; returns the entitlement state afterwards.
  Future<bool> restore();

  /// Switches to another `app_user_id` (Google uid, redeemed link code).
  /// Returns the entitlement state of the new user.
  Future<bool> logIn(String appUserId);

  Future<void> dispose();
}

/// Listener bookkeeping shared by the implementations.
mixin PassStoreListeners implements PassStore {
  final List<PassEntitlementListener> _listeners = [];

  @override
  void addEntitlementListener(PassEntitlementListener listener) =>
      _listeners.add(listener);

  @override
  void removeEntitlementListener(PassEntitlementListener listener) =>
      _listeners.remove(listener);

  /// Calls every listener with [hasEntitlement]. Safe against listeners that
  /// remove themselves while being notified.
  void notifyEntitlement(bool hasEntitlement) {
    for (final l in List<PassEntitlementListener>.of(_listeners)) {
      l(hasEntitlement);
    }
  }

  void clearEntitlementListeners() => _listeners.clear();
}
