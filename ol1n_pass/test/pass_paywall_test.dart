import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:ol1n_pass/ol1n_pass.dart';
import 'package:ol1n_pass/ol1n_pass_testing.dart';

const _config = PassConfig(
  appSlug: 'kirian',
  revenueCatApiKeyIos: 'appl_test',
  revenueCatApiKeyAndroid: 'goog_test',
);

String _doc = _docWorth;

const _docWorth = '''
{"version":1,"updated":"2026-11-01","entitlement":"all_apps",
 "pass":{"name":"Ol1n Pass","lifetime_usd":49.99,"yearly_usd":14.99,
  "landing_url":"https://olin.now/pass","catalog_url":"https://olin.now/pass.json",
  "api_url":"https://pass.ol1n.com"},
 "apps":[
  {"slug":"kirian","name":"Kirian","unlock_value_usd":10,"pass_since":"2.6.0",
   "appstore_url":"https://apps.apple.com/app/id1"},
  {"slug":"lexify","name":"Lexify","unlock_value_usd":45,"pass_since":"1.4.0",
   "appstore_url":"https://apps.apple.com/app/id2",
   "playstore_url":"https://play.google.com/store/apps/details?id=x"},
  {"slug":"puff","name":"Puff","unlock_value_usd":null,"pass_since":null}
 ]}
''';

const _docCheap = '''
{"version":1,"updated":"2026-11-07","entitlement":"all_apps",
 "pass":{"name":"Ol1n Pass","lifetime_usd":49.99,"yearly_usd":14.99,
  "landing_url":"https://olin.now/pass","catalog_url":"https://olin.now/pass.json",
  "api_url":"https://pass.ol1n.com"},
 "apps":[
  {"slug":"kirian","name":"Kirian","unlock_value_usd":13.86,"pass_since":"2.6.0"},
  {"slug":"lexify","name":"Lexify","unlock_value_usd":10.89,"pass_since":"1.4.0"}
 ]}
''';

class _Bundle extends CachingAssetBundle {
  @override
  Future<ByteData> load(String key) async =>
      ByteData.view(Uint8List.fromList(utf8.encode(_doc)).buffer);
}

Future<PassService> _service(FakePassStore store) async {
  final s = PassService.withDependencies(
    store: store,
    localIdentity: InMemoryIdentityStore(),
    sharedIdentity: InMemoryIdentityStore(),
    api: PassApiClient(
      baseUrl: 'https://pass.example',
      client: MockClient((_) async => http.Response('', 503)),
    ),
    catalogLoader: CatalogLoader(
      remoteUrl: Uri.parse('https://olin.now/pass.json'),
      client: MockClient((_) async => http.Response('', 503)),
      bundle: _Bundle(),
    ),
  );
  await s.init(_config);
  return s;
}

Widget _app(Widget child, {Locale locale = const Locale('cs')}) => MaterialApp(
      locale: locale,
      supportedLocales: const [Locale('cs'), Locale('en')],
      localizationsDelegates: GlobalMaterialLocalizations.delegates,
      home: Scaffold(body: child),
    );

void main() {
  setUp(() => _doc = _docWorth);

  testWidgets('hides the value anchor while the apps are worth less than the Pass', (tester) async {
    _doc = _docCheap;
    final s = await _service(FakePassStore());
    await tester.pumpWidget(_app(PassPaywall(service: s)));
    await tester.pumpAndSettle();
    expect(find.byKey(const ValueKey('pass-card-lifetime')), findsOneWidget);
    expect(find.byKey(const ValueKey('pass-value-anchor')), findsNothing);
  });

  testWidgets('shows three cards, the value anchor and the app list', (tester) async {
    final store = FakePassStore();
    final s = await _service(store);
    var appBuys = 0;
    final opened = <Uri>[];

    await tester.pumpWidget(_app(PassPaywall(
      service: s,
      appUnlock: PassAppUnlock(
        title: 'Všechny skiny',
        price: r'$9.99',
        onBuy: () async => appBuys++,
      ),
      onOpenStore: opened.add,
      preferIosStore: true,
    )));
    await tester.pumpAndSettle();

    expect(find.byKey(const ValueKey('pass-card-app')), findsOneWidget);
    expect(find.byKey(const ValueKey('pass-card-lifetime')), findsOneWidget);
    expect(find.byKey(const ValueKey('pass-card-yearly')), findsOneWidget);
    expect(find.text(r'$49.99'), findsOneWidget); // store price from the fake
    expect(find.text(r'$14.99'), findsOneWidget);

    // Value anchor: kirian 10 + lexify 45 (puff has no pass yet) > 49.99.
    expect(find.byKey(const ValueKey('pass-value-anchor')), findsOneWidget);
    expect(find.textContaining(r'$55'), findsOneWidget);
    expect(find.textContaining(r'Ol1n Pass $49.99'), findsOneWidget);

    // App list: host app marked, coming-soon chip for puff, lexify tappable.
    expect(find.byKey(const ValueKey('pass-app-kirian')), findsOneWidget);
    expect(find.text('brzy'), findsOneWidget);
    await tester.tap(find.byKey(const ValueKey('pass-app-lexify')));
    expect(opened.single.toString(), 'https://apps.apple.com/app/id2');

    // The app's own unlock calls back into the app.
    await tester.tap(find.descendant(
      of: find.byKey(const ValueKey('pass-card-app')),
      matching: find.text('Koupit'),
    ));
    await tester.pumpAndSettle();
    expect(appBuys, 1);
    await s.dispose();
  });

  testWidgets('buying the Pass switches to the owned view', (tester) async {
    final store = FakePassStore();
    final s = await _service(store);

    await tester.pumpWidget(_app(PassPaywall(service: s)));
    await tester.pumpAndSettle();
    expect(find.byKey(const ValueKey('pass-card-app')), findsNothing);

    await tester.tap(find.descendant(
      of: find.byKey(const ValueKey('pass-card-lifetime')),
      matching: find.text('Koupit'),
    ));
    await tester.pumpAndSettle();

    expect(s.hasPass.value, isTrue);
    expect(find.text('Máš Ol1n Pass'), findsOneWidget);
    expect(find.byKey(const ValueKey('pass-card-lifetime')), findsNothing);
    expect(find.byKey(const ValueKey('pass-show-code')), findsOneWidget);
    expect(find.byKey(const ValueKey('pass-app-lexify')), findsOneWidget);

    await s.dispose();
  });

  testWidgets('English strings for non-Czech locales', (tester) async {
    final s = await _service(FakePassStore());
    await tester.pumpWidget(_app(PassPaywall(service: s), locale: const Locale('en')));
    await tester.pumpAndSettle();
    expect(find.text('Ol1n Pass forever'), findsOneWidget);
    expect(find.text('Restore purchases'), findsOneWidget);
    expect(find.text('soon'), findsOneWidget);
    await s.dispose();
  });

  testWidgets('restore shows a snackbar with the outcome', (tester) async {
    final store = FakePassStore();
    final s = await _service(store);
    await tester.pumpWidget(_app(PassPaywall(service: s)));
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const ValueKey('pass-restore')));
    await tester.pumpAndSettle();
    expect(find.text('Žádný nákup k obnovení.'), findsOneWidget);
    await s.dispose();
  });

  testWidgets('PassBadge is empty without a Pass and visible with one', (tester) async {
    final store = FakePassStore();
    final s = await _service(store);
    await tester.pumpWidget(_app(PassBadge(service: s)));
    await tester.pumpAndSettle();
    expect(find.text('Máš Ol1n Pass'), findsNothing);

    await s.buyLifetime();
    await tester.pumpAndSettle();
    expect(find.text('Máš Ol1n Pass'), findsOneWidget);
    await s.dispose();
  });
}
