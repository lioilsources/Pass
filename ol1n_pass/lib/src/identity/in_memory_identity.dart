import 'identity_store.dart';

/// [IdentityStore] in a variable — tests, and desktop builds where nothing
/// needs to survive the process.
class InMemoryIdentityStore implements IdentityStore {
  InMemoryIdentityStore([this.value]);

  String? value;

  /// Every write, in order — lets a test assert that the resolver persisted.
  final List<String> writes = [];

  @override
  Future<String?> read() async => value;

  @override
  Future<void> write(String id) async {
    value = id;
    writes.add(id);
  }
}
