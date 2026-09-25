# Schriften

Instrument Sans und Instrument Serif entsprechen der Gestaltung aus `ui-preview/Fitty Tagesansicht.dc.html`. Die unveränderten TTF-Dateien werden lokal gebündelt; der Client lädt keine Schriften von Google Fonts nach.

- [Instrument Sans](https://github.com/google/fonts/tree/main/ofl/instrumentsans): variable Schrift, Lizenz in `InstrumentSans-OFL.txt`.
- [Instrument Serif](https://github.com/google/fonts/tree/main/ofl/instrumentserif): Regular, Lizenz in `InstrumentSerif-OFL.txt`.

Beide verwenden die SIL Open Font License 1.1. Importiert am 17. September 2026. Plattformübergreifendes Laden erfolgt mit [`expo-font` / `useFonts`](https://docs.expo.dev/versions/latest/sdk/font/); einfache UI-Icons werden über [`react-native-svg`](https://docs.expo.dev/versions/latest/sdk/svg/) gezeichnet.
