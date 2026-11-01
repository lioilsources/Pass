import 'package:flutter/foundation.dart';
import 'package:google_sign_in/google_sign_in.dart';

/// Result of [GoogleSignInLink.signIn]: null means the user cancelled.
class GoogleLinkResult {
  const GoogleLinkResult(this.appUserId, this.email);

  /// `google:<uid>` — the `app_user_id` to log into RevenueCat with.
  final String appUserId;
  final String? email;
}

/// Thrown when Google Sign-In fails for a reason other than cancellation.
class GoogleLinkException implements Exception {
  const GoogleLinkException(this.message);
  final String message;
  @override
  String toString() => 'GoogleLinkException: $message';
}

/// Android fallback identity (plan §2.3 layer 1): a Google account id that
/// is the same in every Ol1n app. The app must call this from behind a
/// parental gate when it is in the Kids category.
///
/// Implemented against `google_sign_in` 7.x
/// (`GoogleSignIn.instance.initialize()` + `authenticate()`).
class GoogleSignInLink {
  GoogleSignInLink({GoogleSignIn? instance})
      : _signIn = instance ?? GoogleSignIn.instance;

  static const String prefix = 'google:';

  final GoogleSignIn _signIn;
  bool _initialized = false;

  Future<void> _ensureInitialized() async {
    if (_initialized) return;
    await _signIn.initialize();
    _initialized = true;
  }

  /// Interactive sign-in. Returns null when the user cancels.
  Future<GoogleLinkResult?> signIn() async {
    try {
      await _ensureInitialized();
      final account = await _signIn.authenticate();
      return GoogleLinkResult('$prefix${account.id}', account.email);
    } on GoogleSignInException catch (e) {
      if (e.code == GoogleSignInExceptionCode.canceled) return null;
      debugPrint('ol1n_pass: google sign-in failed: ${e.code} ${e.description}');
      throw GoogleLinkException(e.description ?? e.code.name);
    }
  }

  /// Non-interactive: a previously signed-in account, if the platform can
  /// return one without UI. Null when none.
  Future<GoogleLinkResult?> silent() async {
    try {
      await _ensureInitialized();
      final account = await _signIn.attemptLightweightAuthentication();
      if (account == null) return null;
      return GoogleLinkResult('$prefix${account.id}', account.email);
    } catch (e) {
      debugPrint('ol1n_pass: lightweight google auth failed: $e');
      return null;
    }
  }

  Future<void> signOut() => _signIn.signOut();
}
