import 'dart:io' show Platform;

import 'package:flutter/foundation.dart' show kIsWeb;
import 'package:flutter/material.dart';

import '../catalog/pass_catalog.dart';
import 'pass_strings.dart';

/// The apps the Pass covers, with store links. The host app is marked and
/// not linked; apps without `pass_since` get a "soon" chip.
///
/// Opening a store page is the app's job (`url_launcher` or whatever it
/// already uses) — pass [onOpenStore].
class PassAppsList extends StatelessWidget {
  const PassAppsList({
    super.key,
    required this.catalog,
    required this.currentSlug,
    this.onOpenStore,
    this.showComingSoon = true,
    this.dense = false,
    this.preferIos,
  });

  final PassCatalog catalog;
  final String currentSlug;
  final void Function(Uri url)? onOpenStore;
  final bool showComingSoon;
  final bool dense;

  /// Which store link to prefer; defaults to the running platform.
  final bool? preferIos;

  @override
  Widget build(BuildContext context) {
    final strings = PassStrings.of(context);
    final preferIos = this.preferIos ?? (!kIsWeb && Platform.isIOS);
    final apps = showComingSoon ? catalog.apps : catalog.appsWithPass;
    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        for (final app in apps)
          ListTile(
            key: ValueKey('pass-app-${app.slug}'),
            dense: dense,
            leading: CircleAvatar(
              child: Text(app.name.isEmpty ? '?' : app.name[0].toUpperCase()),
            ),
            title: Text(app.name),
            subtitle: app.unlocks.isEmpty ? null : Text(app.unlocks.join(' · ')),
            trailing: _trailing(context, app, strings, preferIos),
            onTap: _tapFor(app, preferIos),
          ),
      ],
    );
  }

  Widget? _trailing(
      BuildContext context, PassApp app, PassStrings strings, bool preferIos) {
    if (app.slug == currentSlug) {
      return const Icon(Icons.check_circle_outline);
    }
    if (app.comingSoon) {
      final scheme = Theme.of(context).colorScheme;
      return Container(
        padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
        decoration: BoxDecoration(
          color: scheme.surfaceContainerHighest,
          borderRadius: BorderRadius.circular(999),
        ),
        child: Text(
          strings.comingSoon,
          style: Theme.of(context)
              .textTheme
              .labelSmall
              ?.copyWith(color: scheme.onSurfaceVariant),
        ),
      );
    }
    if (app.storeUrl(preferIos: preferIos) != null && onOpenStore != null) {
      return const Icon(Icons.open_in_new);
    }
    return null;
  }

  VoidCallback? _tapFor(PassApp app, bool preferIos) {
    if (app.slug == currentSlug || onOpenStore == null) return null;
    final url = app.storeUrl(preferIos: preferIos);
    if (url == null) return null;
    return () => onOpenStore!(url);
  }
}
