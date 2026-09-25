import { useEffect, useState } from 'react';
import { Pressable, Text, View } from 'react-native';

import { checkConnection } from '../lib/api';
import { styles } from '../styles/home';

type Connection = 'checking' | 'online' | 'offline';

const labels: Record<Connection, string> = {
  checking: 'Verbindung wird geprüft …',
  online: 'Mit Fitty verbunden',
  offline: 'Fitty-Server nicht erreichbar',
};

export function ConnectionStatus() {
  const [connection, setConnection] = useState<Connection>('checking');
  const [attempt, setAttempt] = useState(0);
  const [focused, setFocused] = useState(false);

  useEffect(() => {
    const controller = new AbortController();
    let active = true;
    setConnection('checking');

    const timeout = setTimeout(() => controller.abort(), 5000);
    checkConnection(controller.signal)
      .then(() => {
        if (active) setConnection('online');
      })
      .catch(() => {
        if (active) setConnection('offline');
      })
      .finally(() => clearTimeout(timeout));

    return () => {
      active = false;
      clearTimeout(timeout);
      controller.abort();
    };
  }, [attempt]);

  return (
    <View style={styles.connection}>
      <Text accessibilityLiveRegion="polite" style={styles.connectionLabel}>
        {labels[connection]}
      </Text>
      <Pressable
        accessibilityRole="button"
        accessibilityLabel="Verbindung zum Fitty-Server erneut prüfen"
        accessibilityState={{ disabled: connection === 'checking' }}
        disabled={connection === 'checking'}
        onPress={() => setAttempt((value) => value + 1)}
        onFocus={() => setFocused(true)}
        onBlur={() => setFocused(false)}
        style={[
          styles.retry,
          focused && styles.retryFocused,
          connection === 'checking' && styles.retryDisabled,
        ]}
      >
        <Text style={styles.retryLabel}>Erneut prüfen</Text>
      </Pressable>
    </View>
  );
}
