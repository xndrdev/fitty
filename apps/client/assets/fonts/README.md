# Fonts

Instrument Sans and Instrument Serif match the design in `ui-preview/Fitty Tagesansicht.dc.html`. The unmodified TTF files are bundled locally; the client does not fetch fonts from Google Fonts.

- [Instrument Sans](https://github.com/google/fonts/tree/main/ofl/instrumentsans): variable font, license in `InstrumentSans-OFL.txt`.
- [Instrument Serif](https://github.com/google/fonts/tree/main/ofl/instrumentserif): Regular, license in `InstrumentSerif-OFL.txt`.

Both use the SIL Open Font License 1.1. Imported on September 17, 2026. Fonts are loaded across platforms using [`expo-font` / `useFonts`](https://docs.expo.dev/versions/latest/sdk/font/); simple UI icons are drawn with [`react-native-svg`](https://docs.expo.dev/versions/latest/sdk/svg/).
