import 'dart:io' show Platform;

import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart' show PlatformException;
import 'package:purchases_flutter/purchases_flutter.dart';

import '../pass_config.dart';
import '../pass_models.dart';
import 'pass_store.dart';

/// [PassStore] over the RevenueCat SDK (`purchases_flutter`).
///
/// One RevenueCat project, one entitlement, one offering with the
/// `$rc_lifetime` and `$rc_annual` packages — see docs/CONTRACTS.md §1.
class RevenueCatStore with PassStoreListeners implements PassStore {
  RevenueCatStore(this.config);

  final PassConfig config;

  bool _configured = false;
  void Function(CustomerInfo)? _sdkListener;

  bool _has(CustomerInfo info) =>
      info.entitlements.active.containsKey(config.entitlementId);

  @override
  Future<void> configure(String appUserId) async {
    if (_configured) return;
    final apiKey =
        Platform.isIOS ? config.revenueCatApiKeyIos : config.revenueCatApiKeyAndroid;
    if (config.debugLogs) {
      await Purchases.setLogLevel(LogLevel.debug);
    }
    final configuration = PurchasesConfiguration(apiKey)..appUserID = appUserId;
    await Purchases.configure(configuration);
    _sdkListener = (info) => notifyEntitlement(_has(info));
    Purchases.addCustomerInfoUpdateListener(_sdkListener!);
    _configured = true;
  }

  @override
  Future<bool> hasEntitlement() async {
    try {
      return _has(await Purchases.getCustomerInfo());
    } on PlatformException catch (e) {
      debugPrint('ol1n_pass: getCustomerInfo failed: ${e.message}');
      return false;
    }
  }

  Future<Offering?> _offering() async {
    final offerings = await Purchases.getOfferings();
    return offerings.getOffering(config.offeringId) ?? offerings.current;
  }

  @override
  Future<PassPrices> fetchPrices() async {
    try {
      final offering = await _offering();
      if (offering == null) return PassPrices.none;
      return PassPrices(
        lifetime: offering.lifetime?.storeProduct.priceString,
        yearly: offering.annual?.storeProduct.priceString,
      );
    } on PlatformException catch (e) {
      debugPrint('ol1n_pass: getOfferings failed: ${e.message}');
      return PassPrices.none;
    }
  }

  Future<PassPurchaseResult> _purchase({required bool lifetime}) async {
    try {
      final offering = await _offering();
      final package = lifetime ? offering?.lifetime : offering?.annual;
      if (package == null) {
        return PassPurchaseResult.error(
            'offering "${config.offeringId}" has no ${lifetime ? 'lifetime' : 'annual'} package');
      }
      final result = await Purchases.purchase(PurchaseParams.package(package));
      final has = _has(result.customerInfo);
      notifyEntitlement(has);
      return has
          ? const PassPurchaseResult.success()
          : const PassPurchaseResult.error(
              'purchase completed but entitlement is not active');
    } on PlatformException catch (e) {
      final code = PurchasesErrorHelper.getErrorCode(e);
      if (code == PurchasesErrorCode.purchaseCancelledError) {
        return const PassPurchaseResult.cancelled();
      }
      return PassPurchaseResult.error(e.message ?? code.name);
    }
  }

  @override
  Future<PassPurchaseResult> purchaseLifetime() => _purchase(lifetime: true);

  @override
  Future<PassPurchaseResult> purchaseYearly() => _purchase(lifetime: false);

  @override
  Future<bool> restore() async {
    try {
      final has = _has(await Purchases.restorePurchases());
      notifyEntitlement(has);
      return has;
    } on PlatformException catch (e) {
      debugPrint('ol1n_pass: restorePurchases failed: ${e.message}');
      return false;
    }
  }

  @override
  Future<bool> logIn(String appUserId) async {
    final result = await Purchases.logIn(appUserId);
    final has = _has(result.customerInfo);
    notifyEntitlement(has);
    return has;
  }

  @override
  Future<void> dispose() async {
    if (_sdkListener != null) {
      Purchases.removeCustomerInfoUpdateListener(_sdkListener!);
      _sdkListener = null;
    }
    clearEntitlementListeners();
  }
}
