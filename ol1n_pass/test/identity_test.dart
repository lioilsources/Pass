import 'dart:math';

import 'package:flutter_test/flutter_test.dart';
import 'package:ol1n_pass/ol1n_pass.dart';
import 'package:ol1n_pass/ol1n_pass_testing.dart';

void main() {
  group('PassIdentityResolver', () {
    test('prefers the local id and seeds an empty shared store', () async {
      final local = InMemoryIdentityStore('local-id');
      final shared = InMemoryIdentityStore();
      final r = await PassIdentityResolver(local: local, shared: shared).resolve();

      expect(r.appUserId, 'local-id');
      expect(r.source, PassIdentitySource.local);
      expect(shared.value, 'local-id');
    });

    test('adopts a sibling app id when nothing is local', () async {
      final local = InMemoryIdentityStore();
      final shared = InMemoryIdentityStore('from-app-a');
      final r = await PassIdentityResolver(
        local: local,
        shared: shared,
        sharedSource: PassIdentitySource.sharedProvider,
      ).resolve();

      expect(r.appUserId, 'from-app-a');
      expect(r.source, PassIdentitySource.sharedProvider);
      expect(local.value, 'from-app-a');
    });

    test('generates a UUID v4 and persists it everywhere', () async {
      final local = InMemoryIdentityStore();
      final shared = InMemoryIdentityStore();
      final r = await PassIdentityResolver(
        local: local,
        shared: shared,
        random: Random(42),
      ).resolve();

      expect(r.source, PassIdentitySource.generated);
      expect(PassIdentityResolver.uuidV4.hasMatch(r.appUserId), isTrue,
          reason: r.appUserId);
      expect(local.value, r.appUserId);
      expect(shared.value, r.appUserId);
    });

    test('works without a shared store (desktop, no access group)', () async {
      final local = InMemoryIdentityStore();
      final r = await PassIdentityResolver(local: local).resolve();
      expect(r.source, PassIdentitySource.generated);
      expect(local.value, r.appUserId);
    });

    test('adopt overwrites both stores', () async {
      final local = InMemoryIdentityStore('old');
      final shared = InMemoryIdentityStore('old');
      final resolver = PassIdentityResolver(local: local, shared: shared);
      await resolver.adopt('google:123');
      expect(local.value, 'google:123');
      expect(shared.value, 'google:123');
    });

    test('generateUuidV4 is unique and well-formed', () {
      final rnd = Random(1);
      final ids = {for (var i = 0; i < 200; i++) PassIdentityResolver.generateUuidV4(rnd)};
      expect(ids.length, 200);
      for (final id in ids) {
        expect(PassIdentityResolver.uuidV4.hasMatch(id), isTrue, reason: id);
      }
    });
  });
}
