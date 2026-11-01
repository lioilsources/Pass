/// Where the `app_user_id` used for RevenueCat came from.
enum PassIdentitySource {
  /// Read from this app's own storage (a previous launch).
  local,

  /// Read from the keychain access group shared by all Ol1n apps (iOS).
  sharedKeychain,

  /// Read from another installed Ol1n app's identity provider (Android).
  sharedProvider,

  /// `google:<uid>` after Google Sign-In (Android) or a redeemed link code.
  google,

  /// Freshly generated UUID — first Ol1n app on this device.
  generated,

  /// Not resolved (desktop / web, or `init` not called yet).
  none,
}

/// Outcome of a purchase attempt. `cancelled` is not an error: the user
/// closed the store sheet.
enum PassPurchaseStatus { success, cancelled, error }

class PassPurchaseResult {
  const PassPurchaseResult._(this.status, this.message);

  const PassPurchaseResult.success() : this._(PassPurchaseStatus.success, null);
  const PassPurchaseResult.cancelled()
      : this._(PassPurchaseStatus.cancelled, null);
  const PassPurchaseResult.error(String message)
      : this._(PassPurchaseStatus.error, message);

  final PassPurchaseStatus status;

  /// Human-readable reason when [status] is [PassPurchaseStatus.error].
  final String? message;

  bool get isSuccess => status == PassPurchaseStatus.success;
  bool get isCancelled => status == PassPurchaseStatus.cancelled;
  bool get isError => status == PassPurchaseStatus.error;

  @override
  String toString() => 'PassPurchaseResult($status${message == null ? '' : ', $message'})';
}

/// A short-lived code that links another device / app to this Pass holder.
class LinkCode {
  const LinkCode({
    required this.code,
    required this.display,
    required this.expiresAt,
  });

  /// Eight characters, no separators (what the server stores hashed).
  final String code;

  /// `ABCD-EFGH` — what the user reads out.
  final String display;

  final DateTime expiresAt;

  bool isExpired([DateTime? now]) =>
      (now ?? DateTime.now().toUtc()).isAfter(expiresAt);
}

/// Snapshot of the service for UI that wants more than `hasPass`.
class PassState {
  const PassState({
    this.ready = false,
    this.supported = false,
    this.hasPass = false,
    this.appUserId,
    this.identitySource = PassIdentitySource.none,
    this.lifetimePrice,
    this.yearlyPrice,
    this.offeringsLoaded = false,
    this.lastError,
  });

  /// `init` finished (successfully or not).
  final bool ready;

  /// iOS or Android with a store; false on desktop and web.
  final bool supported;

  final bool hasPass;
  final String? appUserId;
  final PassIdentitySource identitySource;

  /// Localized store price strings, null until offerings loaded.
  final String? lifetimePrice;
  final String? yearlyPrice;
  final bool offeringsLoaded;

  /// Last non-fatal problem (offerings fetch, catalog fetch, …).
  final String? lastError;

  PassState copyWith({
    bool? ready,
    bool? supported,
    bool? hasPass,
    String? appUserId,
    PassIdentitySource? identitySource,
    String? lifetimePrice,
    String? yearlyPrice,
    bool? offeringsLoaded,
    String? lastError,
    bool clearError = false,
  }) =>
      PassState(
        ready: ready ?? this.ready,
        supported: supported ?? this.supported,
        hasPass: hasPass ?? this.hasPass,
        appUserId: appUserId ?? this.appUserId,
        identitySource: identitySource ?? this.identitySource,
        lifetimePrice: lifetimePrice ?? this.lifetimePrice,
        yearlyPrice: yearlyPrice ?? this.yearlyPrice,
        offeringsLoaded: offeringsLoaded ?? this.offeringsLoaded,
        lastError: clearError ? null : (lastError ?? this.lastError),
      );
}

/// Thrown by `pass-api` calls; [code] mirrors the server's `error` field
/// (`no_pass`, `invalid_code`, `rate_limited`, `network`, …).
class PassApiException implements Exception {
  const PassApiException(this.code, this.message, {this.statusCode});

  final String code;
  final String message;
  final int? statusCode;

  @override
  String toString() => 'PassApiException($code${statusCode == null ? '' : ' $statusCode'}: $message)';
}
