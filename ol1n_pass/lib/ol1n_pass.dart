/// Ol1n Pass — one purchase that unlocks every Ol1n app.
///
/// Entitlement lives in RevenueCat; identity is shared between the apps on a
/// device without a login (iOS keychain access group, Android identity
/// provider); link codes and Google Sign-In cover the rest.
/// See `docs/CONTRACTS.md` in the repository.
library;

export 'src/api/pass_api_client.dart' show PassApiClient;
export 'src/catalog/catalog_loader.dart' show CatalogLoader, kBundledCatalogAsset;
export 'src/catalog/pass_catalog.dart';
export 'src/identity/google_signin_link.dart'
    show GoogleSignInLink, GoogleLinkResult, GoogleLinkException;
export 'src/identity/identity_store.dart';
export 'src/identity/pass_identity.dart';
export 'src/pass_config.dart';
export 'src/pass_models.dart';
export 'src/pass_service.dart';
export 'src/store/pass_store.dart';
export 'src/ui/pass_apps_list.dart';
export 'src/ui/pass_badge.dart';
export 'src/ui/pass_paywall.dart';
export 'src/ui/pass_strings.dart';
