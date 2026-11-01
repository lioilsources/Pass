import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:ol1n_pass/ol1n_pass.dart';

void main() {
  group('PassCatalog', () {
    test('parses the bundled apps.json and sums values', () {
      final source = File('assets/apps.json').readAsStringSync();
      final catalog = PassCatalog.parse(source);

      expect(catalog.version, kPassCatalogSchemaVersion);
      expect(catalog.entitlement, 'all_apps');
      expect(catalog.pass.lifetimeUsd, 49.0);
      expect(catalog.pass.catalogUrl.toString(), 'https://olin.now/pass.json');
      expect(catalog.apps.map((a) => a.slug), contains('kirian'));

      // Nothing is wired in yet, so "value today" is 0 and the listed value
      // is Kiran + Lexify (the two apps with a known unlock value).
      expect(catalog.appsWithPass, isEmpty);
      expect(catalog.totalUnlockValueUsd, 0);
      expect(catalog.totalListedValueUsd, 30.0);
      expect(catalog.comingSoon.length, catalog.apps.length);
    });

    test('value today counts only apps with pass_since', () {
      final catalog = PassCatalog.parse(_doc(apps: [
        _app('a', value: 10, since: '1.0.0'),
        _app('b', value: 20, since: null),
        _app('c', value: null, since: '2.0.0'),
      ]));
      expect(catalog.totalUnlockValueUsd, 10);
      expect(catalog.totalListedValueUsd, 30);
      expect(catalog.appsWithPass.map((a) => a.slug), ['a', 'c']);
      expect(catalog.comingSoon.map((a) => a.slug), ['b']);
      expect(catalog.bySlug('b')!.comingSoon, isTrue);
      expect(catalog.bySlug('zzz'), isNull);
    });

    test('rejects a newer schema major', () {
      expect(
        () => PassCatalog.parse(_doc(version: 2)),
        throwsA(isA<PassCatalogVersionException>()),
      );
    });

    test('rejects duplicate slugs', () {
      expect(
        () => PassCatalog.parse(_doc(apps: [_app('a'), _app('a')])),
        throwsA(isA<FormatException>()),
      );
    });

    test('store url prefers the current platform and falls back', () {
      final app = PassApp.fromJson({
        'slug': 'x',
        'name': 'X',
        'appstore_url': 'https://apps.apple.com/app/id1',
        'playstore_url': null,
      });
      expect(app.storeUrl(preferIos: true).toString(), 'https://apps.apple.com/app/id1');
      expect(app.storeUrl(preferIos: false).toString(), 'https://apps.apple.com/app/id1');
    });
  });
}

String _doc({int version = 1, List<Map<String, Object?>> apps = const []}) =>
    jsonEncode({
      'version': version,
      'updated': '2026-11-01',
      'entitlement': 'all_apps',
      'pass': {
        'name': 'Ol1n Pass',
        'lifetime_usd': 49,
        'yearly_usd': 14.99,
        'landing_url': 'https://olin.now/pass',
        'catalog_url': 'https://olin.now/pass.json',
        'api_url': 'https://pass.ol1n.com',
      },
      'apps': apps,
    });

Map<String, Object?> _app(String slug, {double? value, String? since}) => {
      'slug': slug,
      'name': slug.toUpperCase(),
      'unlock_value_usd': value,
      'pass_since': since,
    };
