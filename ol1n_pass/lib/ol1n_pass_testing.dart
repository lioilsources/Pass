/// Test doubles for apps that integrate `ol1n_pass`: a store that grants the
/// Pass on demand and an identity store in memory. Import this only from
/// tests and dev builds.
library;

export 'src/identity/in_memory_identity.dart';
export 'src/store/fake_pass_store.dart';
