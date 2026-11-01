import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:ol1n_pass/ol1n_pass.dart';

void main() {
  group('PassApiClient.normalizeCode', () {
    test('accepts separators and lower case', () {
      expect(PassApiClient.normalizeCode('abcd-efgh'), 'ABCDEFGH');
      expect(PassApiClient.normalizeCode(' abcd efgh '), 'ABCDEFGH');
      expect(PassApiClient.normalizeCode('ABCD_EFGH'), 'ABCDEFGH');
    });

    test('rejects wrong length and ambiguous characters', () {
      expect(PassApiClient.normalizeCode('ABC'), isNull);
      expect(PassApiClient.normalizeCode('ABCD-EFG0'), isNull); // 0
      expect(PassApiClient.normalizeCode('ABCD-EFGO'), isNull); // O
      expect(PassApiClient.normalizeCode('ABCD-EFG1'), isNull); // 1
      expect(PassApiClient.normalizeCode('ABCD-EFGI'), isNull); // I
    });

    test('displayCode inserts the dash', () {
      expect(PassApiClient.displayCode('ABCDEFGH'), 'ABCD-EFGH');
    });
  });

  group('PassApiClient over MockClient', () {
    late List<http.Request> seen;

    PassApiClient client(Future<http.Response> Function(http.Request) handler) {
      seen = [];
      return PassApiClient(
        baseUrl: 'https://pass.example/',
        client: MockClient((req) {
          seen.add(req);
          return handler(req);
        }),
      );
    }

    test('createLinkCode posts app_user_id and parses the code', () async {
      final api = client((req) async {
        expect(req.method, 'POST');
        expect(req.url.toString(), 'https://pass.example/v1/link-code');
        expect(jsonDecode(req.body), {'app_user_id': 'u1'});
        return http.Response(
          jsonEncode({
            'code': 'ABCDEFGH',
            'display': 'ABCD-EFGH',
            'expires_at': '2026-11-01T10:15:00Z',
          }),
          200,
          headers: {'content-type': 'application/json'},
        );
      });
      final code = await api.createLinkCode('u1');
      expect(code.code, 'ABCDEFGH');
      expect(code.display, 'ABCD-EFGH');
      expect(code.expiresAt, DateTime.utc(2026, 11, 1, 10, 15));
      expect(code.isExpired(DateTime.utc(2026, 11, 1, 10, 0)), isFalse);
      expect(code.isExpired(DateTime.utc(2026, 11, 1, 10, 16)), isTrue);
    });

    test('createLinkCode maps 403 no_pass to PassApiException', () async {
      final api = client((_) async => http.Response(
            jsonEncode({'error': 'no_pass', 'message': 'no active entitlement'}),
            403,
          ));
      await expectLater(
        api.createLinkCode('u1'),
        throwsA(isA<PassApiException>()
            .having((e) => e.code, 'code', 'no_pass')
            .having((e) => e.statusCode, 'status', 403)),
      );
    });

    test('redeem normalizes the code before sending', () async {
      final api = client((req) async {
        expect(req.url.path, '/v1/redeem');
        expect(jsonDecode(req.body), {'code': 'ABCDEFGH'});
        return http.Response(jsonEncode({'app_user_id': 'holder'}), 200);
      });
      expect(await api.redeem('abcd-efgh'), 'holder');
      expect(seen, hasLength(1));
    });

    test('redeem rejects malformed input locally', () async {
      final api = client((_) async => http.Response('', 200));
      await expectLater(
        api.redeem('nope'),
        throwsA(isA<PassApiException>().having((e) => e.code, 'code', 'bad_code')),
      );
      expect(seen, isEmpty);
    });

    test('redeem maps 404 invalid_code', () async {
      final api = client((_) async => http.Response(
            jsonEncode({'error': 'invalid_code', 'message': 'unknown'}),
            404,
          ));
      await expectLater(
        api.redeem('ABCDEFGH'),
        throwsA(isA<PassApiException>().having((e) => e.code, 'code', 'invalid_code')),
      );
    });

    test('hasAllApps reads the entitlement flag', () async {
      final api = client((req) async {
        expect(req.url.queryParameters['user'], 'u1');
        return http.Response(jsonEncode({'app_user_id': 'u1', 'all_apps': true}), 200);
      });
      expect(await api.hasAllApps('u1'), isTrue);
    });

    test('non-JSON error bodies become http_<status>', () async {
      final api = client((_) async => http.Response('<html>bad gateway</html>', 502));
      await expectLater(
        api.hasAllApps('u1'),
        throwsA(isA<PassApiException>().having((e) => e.code, 'code', 'http_502')),
      );
    });
  });
}
