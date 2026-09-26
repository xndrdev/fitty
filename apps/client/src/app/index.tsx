import { useEffect, useState } from 'react';
import type { Session } from '@supabase/supabase-js';
import { AppState, Platform, Text, View } from 'react-native';
import { KeyboardScreen } from '../components/keyboard-layout';
import { Login } from '../components/login';
import { Workspace } from '../components/workspace';
import { Button } from '../components/ui';
import { supabase } from '../lib/supabase';
import { styles as s } from '../styles/app';

export default function HomeScreen() {
  const [session, setSession] = useState<Session | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  useEffect(() => {
    if (!supabase) { setLoading(false); return; }
    let active = true;
    supabase.auth.getSession().then(({ data, error }) => {
      if (active) { setSession(data.session); setError(!!error); setLoading(false); }
    }).catch(() => { if (active) { setError(true); setLoading(false); } });
    const { data } = supabase.auth.onAuthStateChange((_event, next) => { if (active) { setSession(next); setLoading(false); } });
    const subscription = Platform.OS !== 'web' ? AppState.addEventListener('change', state => {
      if (state === 'active') supabase!.auth.startAutoRefresh(); else supabase!.auth.stopAutoRefresh();
    }) : null;
    if (Platform.OS !== 'web') supabase.auth.startAutoRefresh();
    return () => { active = false; data.subscription.unsubscribe(); subscription?.remove(); if (Platform.OS !== 'web') supabase!.auth.stopAutoRefresh(); };
  }, []);

  return <KeyboardScreen>
    {!supabase ? <View style={s.pageScroll}><Text style={s.heading}>Fitty ist noch nicht eingerichtet.</Text><Text style={s.subtitle}>Bitte zuerst das lokale Setup aus der README ausführen und die App neu starten.</Text></View>
      : loading ? <View style={s.pageScroll}><Text style={s.subtitle}>Fitty wird geladen …</Text></View>
      : error ? <View style={s.pageScroll}><Text style={s.error}>Die Sitzung konnte nicht geladen werden.</Text><Button label="Zur Anmeldung" onPress={() => { setError(false); setSession(null); }} /></View>
      : session ? <Workspace key={session.user.id} userId={session.user.id} email={session.user.email ?? ''} /> : <Login />}
  </KeyboardScreen>;
}
