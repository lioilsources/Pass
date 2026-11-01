import 'package:flutter/material.dart';

import '../pass_service.dart';
import 'pass_strings.dart';

/// Small "You have Ol1n Pass" chip. Renders nothing while the user has no
/// Pass, so it can sit permanently in a settings screen.
class PassBadge extends StatelessWidget {
  const PassBadge({super.key, this.service, this.onTap});

  final PassService? service;

  /// Typically opens [PassPaywall] / the apps list.
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    final s = service ?? PassService.instance;
    return ValueListenableBuilder<bool>(
      valueListenable: s.hasPass,
      builder: (context, has, _) {
        if (!has) return const SizedBox.shrink();
        final strings = PassStrings.of(context);
        final scheme = Theme.of(context).colorScheme;
        return Material(
          color: scheme.primaryContainer,
          borderRadius: BorderRadius.circular(999),
          child: InkWell(
            borderRadius: BorderRadius.circular(999),
            onTap: onTap,
            child: Padding(
              padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
              child: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Icon(Icons.verified, size: 18, color: scheme.onPrimaryContainer),
                  const SizedBox(width: 6),
                  Text(
                    strings.youHavePass,
                    style: Theme.of(context)
                        .textTheme
                        .labelLarge
                        ?.copyWith(color: scheme.onPrimaryContainer),
                  ),
                ],
              ),
            ),
          ),
        );
      },
    );
  }
}
