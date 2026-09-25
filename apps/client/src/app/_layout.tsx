import { Stack } from 'expo-router';
import { StatusBar } from 'expo-status-bar';
import { useFonts } from 'expo-font';
import { ActivityIndicator, View } from 'react-native';
import { styles as s } from '../styles/app';

export default function RootLayout() {
  const [loaded, error] = useFonts({
    InstrumentSans: require('../../assets/fonts/InstrumentSans.ttf'),
    InstrumentSerif: require('../../assets/fonts/InstrumentSerif-Regular.ttf'),
  });
  if (!loaded && !error) return <View style={s.fontLoading}><ActivityIndicator color="#2F5D3A" accessibilityLabel="Fitty wird geladen" /></View>;
  return (
    <>
      <StatusBar style="dark" />
      <Stack screenOptions={{ headerShown: false }} />
    </>
  );
}
