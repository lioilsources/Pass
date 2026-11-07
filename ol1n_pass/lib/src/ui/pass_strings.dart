import 'package:flutter/widgets.dart';

/// UI strings, Czech and English. Picked from `Localizations.localeOf`;
/// anything that is not Czech gets English.
class PassStrings {
  const PassStrings._({
    required this.passName,
    required this.unlockThisApp,
    required this.lifetime,
    required this.yearly,
    required this.perYear,
    required this.oneTime,
    required this.buy,
    required this.restore,
    required this.restored,
    required this.nothingToRestore,
    required this.youHavePass,
    required this.unlockedAlsoIn,
    required this.comingSoon,
    required this.valueToday,
    required this.lifetimeBlurb,
    required this.yearlyBlurb,
    required this.haveCode,
    required this.showCode,
    required this.enterCode,
    required this.codeHint,
    required this.redeem,
    required this.codeInvalid,
    required this.codeExpiresIn,
    required this.linkGoogle,
    required this.cancel,
    required this.error,
    required this.openStore,
  });

  static const PassStrings cs = PassStrings._(
    passName: 'Ol1n Pass',
    unlockThisApp: 'Odemknout tuto appku',
    lifetime: 'Ol1n Pass navždy',
    yearly: 'Ol1n Pass na rok',
    perYear: '/ rok',
    oneTime: 'jednorázově',
    buy: 'Koupit',
    restore: 'Obnovit nákupy',
    restored: 'Nákupy obnoveny.',
    nothingToRestore: 'Žádný nákup k obnovení.',
    youHavePass: 'Máš Ol1n Pass',
    unlockedAlsoIn: 'Odemčeno i v:',
    comingSoon: 'brzy',
    valueToday: 'Hodnota všech balíčků dnes',
    lifetimeBlurb:
        'Všechny prémiové funkce ve všech appkách od Ol1n, včetně těch, které vyjdou později.',
    yearlyBlurb: 'To samé, na rok. Kdykoliv zrušíš.',
    haveCode: 'Mám kód',
    showCode: 'Zobrazit kód',
    enterCode: 'Zadej kód z appky, kde máš Pass',
    codeHint: 'ABCD-EFGH',
    redeem: 'Propojit',
    codeInvalid: 'Kód neplatí nebo vypršel.',
    codeExpiresIn: 'Platí 15 minut. Zadej ho v jiné appce v „Mám kód“.',
    linkGoogle: 'Propojit s účtem Google',
    cancel: 'Zrušit',
    error: 'Něco se nepovedlo',
    openStore: 'Otevřít ve storu',
  );

  static const PassStrings en = PassStrings._(
    passName: 'Ol1n Pass',
    unlockThisApp: 'Unlock this app',
    lifetime: 'Ol1n Pass forever',
    yearly: 'Ol1n Pass yearly',
    perYear: '/ year',
    oneTime: 'one-time',
    buy: 'Buy',
    restore: 'Restore purchases',
    restored: 'Purchases restored.',
    nothingToRestore: 'Nothing to restore.',
    youHavePass: 'You have Ol1n Pass',
    unlockedAlsoIn: 'Also unlocked in:',
    comingSoon: 'soon',
    valueToday: 'Value of all packs today',
    lifetimeBlurb:
        'Every premium feature in every Ol1n app, including the ones that come out later.',
    yearlyBlurb: 'The same, for a year. Cancel any time.',
    haveCode: 'I have a code',
    showCode: 'Show code',
    enterCode: 'Enter the code from the app where you have the Pass',
    codeHint: 'ABCD-EFGH',
    redeem: 'Link',
    codeInvalid: 'The code is invalid or expired.',
    codeExpiresIn: 'Valid for 15 minutes. Enter it in another app under “I have a code”.',
    linkGoogle: 'Link Google account',
    cancel: 'Cancel',
    error: 'Something went wrong',
    openStore: 'Open in store',
  );

  static PassStrings of(BuildContext context) {
    final locale = Localizations.maybeLocaleOf(context);
    return locale?.languageCode == 'cs' ? cs : en;
  }

  final String passName;
  final String unlockThisApp;
  final String lifetime;
  final String yearly;
  final String perYear;
  final String oneTime;
  final String buy;
  final String restore;
  final String restored;
  final String nothingToRestore;
  final String youHavePass;
  final String unlockedAlsoIn;
  final String comingSoon;
  final String valueToday;
  final String lifetimeBlurb;
  final String yearlyBlurb;
  final String haveCode;
  final String showCode;
  final String enterCode;
  final String codeHint;
  final String redeem;
  final String codeInvalid;
  final String codeExpiresIn;
  final String linkGoogle;
  final String cancel;
  final String error;
  final String openStore;

  /// `$49.99` / `$14.99` style formatting for catalog USD values.
  static String usd(double value) =>
      value == value.roundToDouble() ? '\$${value.round()}' : '\$${value.toStringAsFixed(2)}';
}
