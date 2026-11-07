import 'dart:convert';

import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:ol1n_pass/ol1n_pass.dart';
import 'package:ol1n_pass/ol1n_pass_testing.dart';

const _config = PassConfig(
  appSlug: 'kirian',
  revenueCatApiKeyIos: 'appl_test',
  revenueCatApiKeyAndroid: 'goog_test',
  iosKeychainAccessGroup: 'TEAM.com.ol1n.pass',
);

/// Catalog loader that never touches the network or the asset bundle.
CatalogLoader _offlineCatalog({Map<String, Object?>? remote}) => CatalogLoader(
      remoteUrl: Uri.parse('https://olin.now/pass.json'),
      client: MockClient((_) async => remote == null
          ? http.Response('down', 503)
          : http.Response(jsonEncode(remote), 200)),
      bundle: _TestBundle(),
    );

class _TestBundle extends CachingAssetBundle {
  @override
  Future<ByteData> load(String key) async {
    expect(key, kBundledCatalogAsset);
    final bytes = utf8.encode(_bundledDoc);
    return ByteData.view(Uint8List.fromList(bytes).buffer);
  }
}

const _bundledDoc = '''
{"version":1,"updated":"2026-11-01","entitlement":"all_apps",
 "pass":{"name":"Ol1n Pass","lifetime_usd":49,"yearly_usd":14.99,
  "landing_url":"https://olin.now/pass","catalog_url":"https://olin.now/pass.json",
  "api_url":"https://pass.ol1n.com"},
 "apps":[{"slug":"kirian","name":"Kirian","unlock_value_usd":10,"pass_since":"2.6.0"},
         {"slug":"lexify","name":"Lexify","unlock_value_usd":20,"pass_since":null}]}
''';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  group('PassService', () {
    late FakePassStore store;
    late InMemoryIdentityStore local;
    late InMemoryIdentityStore shared;

    PassService make({
      PassApiClient? api,
      bool supported = true,
      CatalogLoader? catalog,
    }) =>
        PassService.withDependencies(
          store: store,
          localIdentity: local,
          sharedIdentity: shared,
          sharedSource: PassIdentitySource.sharedKeychain,
          api: api,
          catalogLoader: catalog ?? _offlineCatalog(),
          supported: supported,
        );

    setUp(() {
      store = FakePassStore();
      local = InMemoryIdentityStore();
      shared = InMemoryIdentityStore();
    });

    test('init resolves identity, configures the store, loads the catalog', () async {
      final s = make();
      await s.init(_config);

      expect(s.isReady, isTrue);
      expect(s.appUserId, isNotNull);
      expect(store.currentUser, s.appUserId);
      expect(store.calls.first, 'configure:${s.appUserId}');
      expect(s.state.value.identitySource, PassIdentitySource.generated);
      expect(s.hasPass.value, isFalse);
      expect(s.catalog, isNotNull);
      expect(s.catalog!.totalUnlockValueUsd, 10); // bundled fallback: kirian only
      expect(s.state.value.lifetimePrice, r'$49.99');
      await s.dispose();
    });

    test('init is idempotent', () async {
      final s = make();
      await Future.wait([s.init(_config), s.init(_config)]);
      await s.init(_config);
      expect(store.calls.where((c) => c.startsWith('configure:')).length, 1);
      await s.dispose();
    });

    test('a sibling app id is used when present', () async {
      shared.value = 'shared-uuid';
      final s = make();
      await s.init(_config);
      expect(s.appUserId, 'shared-uuid');
      expect(s.state.value.identitySource, PassIdentitySource.sharedKeychain);
      expect(local.value, 'shared-uuid');
      await s.dispose();
    });

    test('hasPass is true at start when the shared user already owns it', () async {
      shared.value = 'holder';
      store.entitledUsers.add('holder');
      final s = make();
      await s.init(_config);
      expect(s.hasPass.value, isTrue);
      await s.dispose();
    });

    test('buyLifetime flips hasPass and notifies listeners', () async {
      final s = make();
      await s.init(_config);
      var notified = 0;
      s.hasPass.addListener(() => notified++);

      final r = await s.buyLifetime();
      expect(r.isSuccess, isTrue);
      expect(s.hasPass.value, isTrue);
      expect(notified, 1);
      expect(store.calls, contains('purchase:lifetime'));
      await s.dispose();
    });

    test('cancelled purchase leaves hasPass false', () async {
      store.nextPurchase = const PassPurchaseResult.cancelled();
      final s = make();
      await s.init(_config);
      final r = await s.buyYearly();
      expect(r.isCancelled, isTrue);
      expect(s.hasPass.value, isFalse);
      await s.dispose();
    });

    test('restore re-delivers a store transaction', () async {
      final s = make();
      await s.init(_config);
      store.restorableUsers.add(s.appUserId!);
      expect(await s.restore(), isTrue);
      expect(s.hasPass.value, isTrue);
      await s.dispose();
    });

    test('backend update (refund) drops hasPass', () async {
      final s = make();
      await s.init(_config);
      await s.buyLifetime();
      expect(s.hasPass.value, isTrue);

      store.entitledUsers.clear();
      store.emit();
      await Future<void>.delayed(Duration.zero);
      expect(s.hasPass.value, isFalse);
      await s.dispose();
    });

    test('redeemLinkCode logs in as the holder and adopts the identity', () async {
      store.entitledUsers.add('holder-uuid');
      final api = PassApiClient(
        baseUrl: 'https://pass.example',
        client: MockClient((req) async {
          expect(req.url.path, '/v1/redeem');
          expect(jsonDecode(req.body), {'code': 'ABCDEFGH'});
          return http.Response(jsonEncode({'app_user_id': 'holder-uuid'}), 200);
        }),
      );
      final s = make(api: api);
      await s.init(_config);
      expect(s.hasPass.value, isFalse);

      expect(await s.redeemLinkCode('abcd-efgh'), isTrue);
      expect(s.hasPass.value, isTrue);
      expect(s.appUserId, 'holder-uuid');
      expect(local.value, 'holder-uuid');
      expect(shared.value, 'holder-uuid');
      expect(store.calls, contains('logIn:holder-uuid'));
      await s.dispose();
    });

    test('createLinkCode sends the current app_user_id', () async {
      String? sent;
      final api = PassApiClient(
        baseUrl: 'https://pass.example',
        client: MockClient((req) async {
          sent = (jsonDecode(req.body) as Map)['app_user_id'] as String;
          return http.Response(
            jsonEncode({'code': 'ABCDEFGH', 'expires_at': '2026-11-01T10:15:00Z'}),
            200,
          );
        }),
      );
      final s = make(api: api);
      await s.init(_config);
      final code = await s.createLinkCode();
      expect(sent, s.appUserId);
      expect(code.display, 'ABCD-EFGH');
      await s.dispose();
    });

    test('remote catalog wins over the bundled copy', () async {
      final s = make(
        catalog: _offlineCatalog(remote: {
          'version': 1,
          'updated': '2026-12-01',
          'entitlement': 'all_apps',
          'pass': {
            'lifetime_usd': 49,
            'yearly_usd': 14.99,
            'landing_url': 'https://olin.now/pass',
            'catalog_url': 'https://olin.now/pass.json',
            'api_url': 'https://pass.ol1n.com',
          },
          'apps': [
            {'slug': 'kirian', 'name': 'Kirian', 'unlock_value_usd': 10, 'pass_since': '2.6.0'},
            {'slug': 'lexify', 'name': 'Lexify', 'unlock_value_usd': 20, 'pass_since': '1.4.0'},
            {'slug': 'newapp', 'name': 'New', 'unlock_value_usd': 5, 'pass_since': '1.0.0'},
          ],
        }),
      );
      await s.init(_config);
      expect(s.catalog!.updated, '2026-12-01');
      expect(s.catalog!.totalUnlockValueUsd, 35);
      await s.dispose();
    });

    test('unsupported platform: ready, no identity, no pass, purchases are errors', () async {
      final s = make(supported: false);
      await s.init(_config);
      expect(s.isReady, isTrue);
      expect(s.state.value.supported, isFalse);
      expect(s.appUserId, isNull);
      expect(s.hasPass.value, isFalse);
      expect(store.calls, isEmpty);
      expect((await s.buyLifetime()).isError, isTrue);
      expect(await s.restore(), isFalse);
      expect(await s.linkGoogleAccount(), isNull);
      expect(s.catalog, isNotNull);
      await s.dispose();
    });

    test('store failure during init is reported, not thrown', () async {
      final failing = _ThrowingStore();
      final s = PassService.withDependencies(
        store: failing,
        localIdentity: local,
        sharedIdentity: shared,
        catalogLoader: _offlineCatalog(),
      );
      await s.init(_config);
      expect(s.isReady, isTrue);
      expect(s.hasPass.value, isFalse);
      expect(s.state.value.lastError, contains('store:'));
    });
  });
}

class _ThrowingStore extends FakePassStore {
  @override
  Future<void> configure(String appUserId) async {
    throw StateError('no network');
  }
}
