import '../pass_models.dart';
import 'pass_store.dart';

/// In-memory [PassStore] for tests and for running an app without store
/// credentials (e.g. a desktop dev build).
///
/// Entitlements are modelled per `app_user_id`, so a test can grant the Pass
/// to one id and check that `logIn` to it flips `hasPass`.
class FakePassStore with PassStoreListeners implements PassStore {
  FakePassStore({
    Set<String>? entitledUsers,
    this.prices = const PassPrices(lifetime: r'$49.00', yearly: r'$14.99'),
    this.nextPurchase = const PassPurchaseResult.success(),
  }) : entitledUsers = entitledUsers ?? <String>{};

  /// Users that own the Pass. Mutate freely in tests.
  final Set<String> entitledUsers;

  PassPrices prices;

  /// What the next `purchase*` call returns. On success the current user is
  /// added to [entitledUsers].
  PassPurchaseResult nextPurchase;

  /// Users whose Pass is restorable (store transaction exists on the device).
  final Set<String> restorableUsers = <String>{};

  String? currentUser;
  final List<String> calls = [];

  bool get _has => currentUser != null && entitledUsers.contains(currentUser);

  /// Simulates a background update from the backend (refund, renewal…).
  void emit() => notifyEntitlement(_has);

  @override
  Future<void> configure(String appUserId) async {
    calls.add('configure:$appUserId');
    currentUser = appUserId;
  }

  @override
  Future<bool> hasEntitlement() async => _has;

  @override
  Future<PassPrices> fetchPrices() async => prices;

  Future<PassPurchaseResult> _purchase(String what) async {
    calls.add('purchase:$what');
    final result = nextPurchase;
    if (result.isSuccess && currentUser != null) {
      entitledUsers.add(currentUser!);
      notifyEntitlement(true);
    }
    return result;
  }

  @override
  Future<PassPurchaseResult> purchaseLifetime() => _purchase('lifetime');

  @override
  Future<PassPurchaseResult> purchaseYearly() => _purchase('yearly');

  @override
  Future<bool> restore() async {
    calls.add('restore');
    if (currentUser != null && restorableUsers.contains(currentUser)) {
      entitledUsers.add(currentUser!);
    }
    notifyEntitlement(_has);
    return _has;
  }

  @override
  Future<bool> logIn(String appUserId) async {
    calls.add('logIn:$appUserId');
    currentUser = appUserId;
    notifyEntitlement(_has);
    return _has;
  }

  @override
  Future<void> dispose() async => clearEntitlementListeners();
}
