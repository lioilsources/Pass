import 'package:flutter/material.dart';

import '../api/pass_api_client.dart';
import '../pass_models.dart';
import '../pass_service.dart';
import 'pass_apps_list.dart';
import 'pass_strings.dart';

/// The host app's own unlock, shown as the first card. Price and action are
/// the app's existing IAP — the Pass never replaces it.
class PassAppUnlock {
  const PassAppUnlock({
    required this.title,
    required this.price,
    required this.onBuy,
    this.subtitle,
    this.owned = false,
  });

  final String title;
  final String? subtitle;

  /// Localized price string from the store, null while loading.
  final String? price;
  final Future<void> Function() onBuy;

  /// Already bought — the card shows a check instead of a button.
  final bool owned;
}

/// Three cards (app unlock / Pass lifetime / Pass yearly), the value anchor
/// from `apps.json`, the app list, Restore and link-code actions.
///
/// Drop it in a sheet or a page. After a successful Pass purchase it swaps to
/// the "you have Ol1n Pass — also unlocked in…" view, which is the free
/// cross-promo the plan asks for.
class PassPaywall extends StatefulWidget {
  const PassPaywall({
    super.key,
    this.service,
    this.appUnlock,
    this.onOpenStore,
    this.showYearly = true,
    this.showLinkCode = true,
    this.showGoogleLink = false,
    this.padding = const EdgeInsets.all(16),
    this.preferIosStore,
  });

  /// Passed to [PassAppsList]; defaults to the running platform.
  final bool? preferIosStore;

  final PassService? service;
  final PassAppUnlock? appUnlock;
  final void Function(Uri url)? onOpenStore;
  final bool showYearly;

  /// Show "Show code" (Pass holders) / "I have a code" (others).
  final bool showLinkCode;

  /// Android: show "Link Google account". The app decides — in kids mode
  /// it must be behind a parental gate, so default off.
  final bool showGoogleLink;

  final EdgeInsets padding;

  @override
  State<PassPaywall> createState() => _PassPaywallState();
}

class _PassPaywallState extends State<PassPaywall> {
  PassService get service => widget.service ?? PassService.instance;
  bool _busy = false;

  Future<void> _run(Future<void> Function() action) async {
    if (_busy) return;
    setState(() => _busy = true);
    try {
      await action();
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  void _snack(String text) {
    if (!mounted) return;
    ScaffoldMessenger.maybeOf(context)?.showSnackBar(SnackBar(content: Text(text)));
  }

  Future<void> _buy(Future<PassPurchaseResult> Function() buy, PassStrings s) =>
      _run(() async {
        final r = await buy();
        if (r.isError) _snack('${s.error}: ${r.message}');
      });

  Future<void> _restore(PassStrings s) => _run(() async {
        final has = await service.restore();
        _snack(has ? s.restored : s.nothingToRestore);
      });

  Future<void> _showCode(PassStrings s) => _run(() async {
        try {
          final code = await service.createLinkCode();
          if (!mounted) return;
          await showDialog<void>(
            context: context,
            builder: (ctx) => AlertDialog(
              title: Text(s.showCode),
              content: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  SelectableText(
                    code.display,
                    style: Theme.of(ctx).textTheme.headlineMedium?.copyWith(
                          fontFeatures: const [FontFeature.tabularFigures()],
                          letterSpacing: 2,
                        ),
                  ),
                  const SizedBox(height: 12),
                  Text(s.codeExpiresIn, textAlign: TextAlign.center),
                ],
              ),
              actions: [
                TextButton(onPressed: () => Navigator.of(ctx).pop(), child: const Text('OK')),
              ],
            ),
          );
        } on PassApiException catch (e) {
          _snack('${s.error}: ${e.message}');
        }
      });

  Future<void> _enterCode(PassStrings s) async {
    final controller = TextEditingController();
    final code = await showDialog<String>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(s.haveCode),
        content: TextField(
          controller: controller,
          autofocus: true,
          textCapitalization: TextCapitalization.characters,
          decoration: InputDecoration(hintText: s.codeHint, helperText: s.enterCode),
          onSubmitted: (v) => Navigator.of(ctx).pop(v),
        ),
        actions: [
          TextButton(onPressed: () => Navigator.of(ctx).pop(), child: Text(s.cancel)),
          FilledButton(
            onPressed: () => Navigator.of(ctx).pop(controller.text),
            child: Text(s.redeem),
          ),
        ],
      ),
    );
    if (code == null || code.trim().isEmpty) return;
    await _run(() async {
      if (PassApiClient.normalizeCode(code) == null) {
        _snack(s.codeInvalid);
        return;
      }
      try {
        final has = await service.redeemLinkCode(code);
        if (!has) _snack(s.codeInvalid);
      } on PassApiException catch (e) {
        _snack(e.code == 'invalid_code' || e.code == 'bad_code'
            ? s.codeInvalid
            : '${s.error}: ${e.message}');
      }
    });
  }

  Future<void> _google(PassStrings s) => _run(() async {
        try {
          await service.linkGoogleAccount();
        } catch (e) {
          _snack('${s.error}: $e');
        }
      });

  @override
  Widget build(BuildContext context) {
    final s = PassStrings.of(context);
    return ValueListenableBuilder<PassState>(
      valueListenable: service.state,
      builder: (context, state, _) {
        final catalog = service.catalog;
        return SingleChildScrollView(
          padding: widget.padding,
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: state.hasPass
                ? _owned(context, s, catalog)
                : _offer(context, s, state, catalog),
          ),
        );
      },
    );
  }

  List<Widget> _owned(BuildContext context, PassStrings s, catalog) {
    return [
      Row(
        children: [
          Icon(Icons.verified, color: Theme.of(context).colorScheme.primary),
          const SizedBox(width: 8),
          Text(s.youHavePass, style: Theme.of(context).textTheme.titleLarge),
        ],
      ),
      const SizedBox(height: 8),
      Text(s.unlockedAlsoIn, style: Theme.of(context).textTheme.labelLarge),
      if (catalog != null)
        PassAppsList(
          catalog: catalog,
          currentSlug: service.config.appSlug,
          onOpenStore: widget.onOpenStore,
          preferIos: widget.preferIosStore,
        ),
      const SizedBox(height: 8),
      if (widget.showLinkCode)
        OutlinedButton.icon(
          key: const ValueKey('pass-show-code'),
          onPressed: _busy ? null : () => _showCode(s),
          icon: const Icon(Icons.qr_code_2),
          label: Text(s.showCode),
        ),
    ];
  }

  List<Widget> _offer(BuildContext context, PassStrings s, PassState state, catalog) {
    final theme = Theme.of(context);
    final unlock = widget.appUnlock;
    final totalValue = catalog?.totalUnlockValueUsd ?? 0.0;
    final lifetimeUsd = catalog?.pass.lifetimeUsd ?? 49.0;
    final yearlyUsd = catalog?.pass.yearlyUsd;
    return [
      if (unlock != null)
        _Card(
          key: const ValueKey('pass-card-app'),
          title: unlock.title,
          subtitle: unlock.subtitle,
          price: unlock.price,
          priceNote: s.oneTime,
          buttonLabel: unlock.owned ? null : s.buy,
          owned: unlock.owned,
          onPressed: _busy ? null : () => _run(unlock.onBuy),
        ),
      _Card(
        key: const ValueKey('pass-card-lifetime'),
        title: s.lifetime,
        subtitle: s.lifetimeBlurb,
        price: state.lifetimePrice ?? PassStrings.usd(lifetimeUsd),
        priceNote: s.oneTime,
        highlighted: true,
        buttonLabel: s.buy,
        onPressed: _busy || !state.supported ? null : () => _buy(service.buyLifetime, s),
      ),
      if (widget.showYearly && (state.yearlyPrice != null || yearlyUsd != null))
        _Card(
          key: const ValueKey('pass-card-yearly'),
          title: s.yearly,
          subtitle: s.yearlyBlurb,
          price: state.yearlyPrice ?? PassStrings.usd(yearlyUsd!),
          priceNote: s.perYear,
          buttonLabel: s.buy,
          onPressed: _busy || !state.supported ? null : () => _buy(service.buyYearly, s),
        ),
      const SizedBox(height: 8),
      if (catalog != null && totalValue > 0)
        Text(
          '${s.valueToday}: ${PassStrings.usd(totalValue)} · ${s.passName} ${PassStrings.usd(lifetimeUsd)}',
          key: const ValueKey('pass-value-anchor'),
          textAlign: TextAlign.center,
          style: theme.textTheme.bodyMedium?.copyWith(color: theme.colorScheme.onSurfaceVariant),
        ),
      if (catalog != null) ...[
        const SizedBox(height: 8),
        PassAppsList(
          catalog: catalog,
          currentSlug: service.config.appSlug,
          onOpenStore: widget.onOpenStore,
          dense: true,
          preferIos: widget.preferIosStore,
        ),
      ],
      const SizedBox(height: 8),
      Wrap(
        alignment: WrapAlignment.center,
        spacing: 8,
        children: [
          TextButton(
            key: const ValueKey('pass-restore'),
            onPressed: _busy || !state.supported ? null : () => _restore(s),
            child: Text(s.restore),
          ),
          if (widget.showLinkCode)
            TextButton(
              key: const ValueKey('pass-have-code'),
              onPressed: _busy || !state.supported ? null : () => _enterCode(s),
              child: Text(s.haveCode),
            ),
          if (widget.showGoogleLink)
            TextButton(
              key: const ValueKey('pass-google'),
              onPressed: _busy || !state.supported ? null : () => _google(s),
              child: Text(s.linkGoogle),
            ),
        ],
      ),
      if (state.lastError != null)
        Padding(
          padding: const EdgeInsets.only(top: 8),
          child: Text(
            state.lastError!,
            style: theme.textTheme.bodySmall?.copyWith(color: theme.colorScheme.error),
            textAlign: TextAlign.center,
          ),
        ),
    ];
  }
}

class _Card extends StatelessWidget {
  const _Card({
    super.key,
    required this.title,
    this.subtitle,
    this.price,
    this.priceNote,
    this.buttonLabel,
    this.onPressed,
    this.highlighted = false,
    this.owned = false,
  });

  final String title;
  final String? subtitle;
  final String? price;
  final String? priceNote;
  final String? buttonLabel;
  final VoidCallback? onPressed;
  final bool highlighted;
  final bool owned;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final scheme = theme.colorScheme;
    return Card(
      margin: const EdgeInsets.only(bottom: 12),
      color: highlighted ? scheme.primaryContainer : null,
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Row(
          children: [
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(title, style: theme.textTheme.titleMedium),
                  if (subtitle != null) ...[
                    const SizedBox(height: 4),
                    Text(subtitle!, style: theme.textTheme.bodySmall),
                  ],
                ],
              ),
            ),
            const SizedBox(width: 12),
            Column(
              crossAxisAlignment: CrossAxisAlignment.end,
              children: [
                if (price != null)
                  Text(price!, style: theme.textTheme.titleLarge),
                if (priceNote != null)
                  Text(priceNote!, style: theme.textTheme.labelSmall),
                const SizedBox(height: 6),
                if (owned)
                  const Icon(Icons.check_circle)
                else if (buttonLabel != null)
                  FilledButton(onPressed: onPressed, child: Text(buttonLabel!)),
              ],
            ),
          ],
        ),
      ),
    );
  }
}
