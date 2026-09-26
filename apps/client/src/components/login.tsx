import { useRef, useState } from 'react';
import { Keyboard, Text, TextInput, View } from 'react-native';
import { supabase } from '../lib/supabase';
import { styles as s } from '../styles/app';
import { Brand, Button, ErrorNotice, Field } from './ui';
import { FormScrollView } from './keyboard-layout';

export function Login() {
  const passwordInput = useRef<TextInput>(null);
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  async function login() {
    if (busy || !email.trim() || !password) return;
    Keyboard.dismiss(); setBusy(true); setError('');
    try {
      const result = await supabase!.auth.signInWithPassword({ email: email.trim(), password });
      if (result.error) setError(result.error.status === 400 || result.error.status === 401 ? 'E-Mail oder Passwort stimmen nicht.' : 'Die Anmeldung ist gerade nicht möglich. Bitte erneut versuchen.');
    } catch { setError('Keine Verbindung zur Anmeldung. Bitte erneut versuchen.'); }
    finally { setBusy(false); }
  }
  return <FormScrollView contentContainerStyle={s.loginScroll}><View style={s.login}>
    <Brand />
    <View><Text style={s.eyebrow}>DEIN TAG. DEIN TEMPO.</Text><Text role="heading" aria-level={1} style={s.title}>Willkommen bei Fitty.</Text></View>
    <Text style={s.subtitle}>Ein Platz für dein Essen, deine Bewegung und die kleinen Fragen zwischendurch.</Text>
    <View style={s.card}>
      <Field label="E-Mail" value={email} onChangeText={setEmail} autoCapitalize="none" autoCorrect={false} keyboardType="email-address" autoComplete="email" editable={!busy} returnKeyType="next" submitBehavior="submit" onSubmitEditing={() => passwordInput.current?.focus()} />
      <Field ref={passwordInput} label="Passwort" value={password} onChangeText={setPassword} secureTextEntry autoComplete="current-password" editable={!busy} onSubmitEditing={login} returnKeyType="go" />
      <ErrorNotice text={error} />
      <Button label={busy ? 'Anmelden …' : 'Anmelden'} disabled={busy || !email.trim() || !password} onPress={login} />
    </View>
    <Text style={s.small}>Dein privates Tagebuch. Die Anmeldung ist nur mit einem eingerichteten Konto möglich.</Text>
  </View></FormScrollView>;
}
