import { useState } from 'react';
import { ScrollView, Text, View } from 'react-native';
import { supabase } from '../lib/supabase';
import { styles as s } from '../styles/app';
import { Brand, Button, ErrorNotice, Field } from './ui';

export function Login() {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  async function login() {
    if (busy || !email.trim() || !password) return;
    setBusy(true); setError('');
    try {
      const result = await supabase!.auth.signInWithPassword({ email: email.trim(), password });
      if (result.error) setError(result.error.status === 400 || result.error.status === 401 ? 'E-Mail oder Passwort stimmen nicht.' : 'Die Anmeldung ist gerade nicht möglich. Bitte erneut versuchen.');
    } catch { setError('Keine Verbindung zur Anmeldung. Bitte erneut versuchen.'); }
    finally { setBusy(false); }
  }
  return <ScrollView contentContainerStyle={s.loginScroll} keyboardShouldPersistTaps="handled"><View style={s.login}>
    <Brand />
    <View><Text style={s.eyebrow}>DEIN TAG. DEIN TEMPO.</Text><Text role="heading" aria-level={1} style={s.title}>Willkommen bei Fitty.</Text></View>
    <Text style={s.subtitle}>Ein Platz für dein Essen, deine Bewegung und die kleinen Fragen zwischendurch.</Text>
    <View style={s.card}>
      <Field label="E-Mail" value={email} onChangeText={setEmail} autoCapitalize="none" keyboardType="email-address" autoComplete="email" editable={!busy} />
      <Field label="Passwort" value={password} onChangeText={setPassword} secureTextEntry autoComplete="current-password" editable={!busy} onSubmitEditing={login} returnKeyType="go" />
      <ErrorNotice text={error} />
      <Button label={busy ? 'Anmelden …' : 'Anmelden'} disabled={busy || !email.trim() || !password} onPress={login} />
    </View>
    <Text style={s.small}>Dein privates Tagebuch. Die Anmeldung ist nur mit einem eingerichteten Konto möglich.</Text>
  </View></ScrollView>;
}
