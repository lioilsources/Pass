import 'dart:convert';

import 'package:http/http.dart' as http;

import '../pass_models.dart';

/// Thin client for `pass-api` (docs/CONTRACTS.md §4).
class PassApiClient {
  PassApiClient({
    required String baseUrl,
    http.Client? client,
    this.timeout = const Duration(seconds: 10),
  })  : baseUrl = baseUrl.endsWith('/')
            ? baseUrl.substring(0, baseUrl.length - 1)
            : baseUrl,
        _client = client ?? http.Client();

  final String baseUrl;
  final Duration timeout;
  final http.Client _client;

  /// Letters/digits the server uses: no 0/O/1/I.
  static const String codeAlphabet = 'ABCDEFGHJKLMNPQRSTUVWXYZ23456789';

  /// `abcd-efgh` / `ABCD EFGH` / `abcdefgh` → `ABCDEFGH`. Returns null when
  /// the result is not exactly eight characters from [codeAlphabet].
  static String? normalizeCode(String input) {
    final cleaned = input.toUpperCase().replaceAll(RegExp(r'[\s\-_]'), '');
    if (cleaned.length != 8) return null;
    for (final rune in cleaned.runes) {
      if (!codeAlphabet.contains(String.fromCharCode(rune))) return null;
    }
    return cleaned;
  }

  /// `ABCDEFGH` → `ABCD-EFGH`.
  static String displayCode(String code) =>
      code.length == 8 ? '${code.substring(0, 4)}-${code.substring(4)}' : code;

  /// `POST /v1/link-code`. Throws [PassApiException] with `no_pass` (403)
  /// when the user has no active entitlement.
  Future<LinkCode> createLinkCode(String appUserId) async {
    final json = await _post('/v1/link-code', {'app_user_id': appUserId});
    final code = json['code'] as String;
    return LinkCode(
      code: code,
      display: json['display'] as String? ?? displayCode(code),
      expiresAt: DateTime.parse(json['expires_at'] as String).toUtc(),
    );
  }

  /// `POST /v1/redeem`. Returns the Pass holder's `app_user_id`. Throws
  /// [PassApiException] with `invalid_code` (404) for unknown, used or
  /// expired codes, and `bad_code` locally for malformed input.
  Future<String> redeem(String code) async {
    final normalized = normalizeCode(code);
    if (normalized == null) {
      throw const PassApiException('bad_code', 'code must be 8 characters');
    }
    final json = await _post('/v1/redeem', {'code': normalized});
    return json['app_user_id'] as String;
  }

  /// `GET /v1/entitlements?user=…` — server-side view, for diagnostics.
  Future<bool> hasAllApps(String appUserId) async {
    final uri = Uri.parse('$baseUrl/v1/entitlements')
        .replace(queryParameters: {'user': appUserId});
    final json = await _send(() => _client.get(uri));
    return json['all_apps'] == true;
  }

  Future<Map<String, dynamic>> _post(String path, Map<String, Object?> body) =>
      _send(() => _client.post(
            Uri.parse('$baseUrl$path'),
            headers: const {'content-type': 'application/json'},
            body: jsonEncode(body),
          ));

  Future<Map<String, dynamic>> _send(
      Future<http.Response> Function() request) async {
    http.Response response;
    try {
      response = await request().timeout(timeout);
    } catch (e) {
      throw PassApiException('network', e.toString());
    }
    Map<String, dynamic>? json;
    if (response.body.isNotEmpty) {
      try {
        final decoded = jsonDecode(response.body);
        if (decoded is Map<String, dynamic>) json = decoded;
      } catch (_) {
        // Non-JSON body (proxy error page); handled below.
      }
    }
    if (response.statusCode >= 200 && response.statusCode < 300) {
      if (json == null) {
        throw PassApiException('bad_response', 'expected a JSON object',
            statusCode: response.statusCode);
      }
      return json;
    }
    throw PassApiException(
      json?['error'] as String? ?? 'http_${response.statusCode}',
      json?['message'] as String? ?? response.reasonPhrase ?? 'request failed',
      statusCode: response.statusCode,
    );
  }

  void close() => _client.close();
}
