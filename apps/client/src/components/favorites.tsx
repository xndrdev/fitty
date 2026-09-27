import { useEffect, useRef, useState } from 'react';
import { Modal, Platform, Pressable, ScrollView, Text, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import type { Favorite, FavoriteMatch } from '../lib/favorites';
import type { useFavorites } from '../hooks/use-favorites';
import { styles as s } from '../styles/favorites';
import { styles as menu } from '../styles/composer-actions';
import { Button, ErrorNotice } from './ui';
import { Icon } from './icon';

function FavoriteChoice({ favorite, compact, onPress }: { favorite: Favorite; compact?: boolean; onPress: () => void }) {
  const [focused, setFocused] = useState(false);
  const entry = favorite.entry;
  const detail = [entry.amount, entry.calories === null ? null : `${entry.calories.toLocaleString('de-DE')} kcal`].filter(Boolean).join(' · ');
  return <Pressable accessibilityRole="button" accessibilityLabel={`${entry.label} einfügen${detail ? `, ${detail}` : ''}`}
    onPress={onPress} onFocus={() => setFocused(true)} onBlur={() => setFocused(false)}
    style={({ pressed }) => [compact ? s.suggestion : s.choose, focused && menu.focus, pressed && menu.pressed]}>
    {compact && <Icon name="star" size={16} />}
    <View style={s.text}><Text numberOfLines={compact ? 1 : 2} style={s.label}>{entry.label}</Text>
      {!!detail && <Text numberOfLines={compact ? 1 : 2} style={s.detail}>{detail}</Text>}</View>
  </Pressable>;
}

export function FavoriteSuggestions({ matches, onChoose }: { matches: FavoriteMatch[]; onChoose: (match: FavoriteMatch) => void }) {
  if (!matches.length) return null;
  return <ScrollView horizontal style={s.suggestions} contentContainerStyle={s.suggestionList} keyboardShouldPersistTaps="handled"
    showsHorizontalScrollIndicator={false} accessibilityLabel="Passende Favoriten" testID="favorite-suggestions">
    {matches.map(match => <FavoriteChoice key={match.favorite.id} compact favorite={match.favorite} onPress={() => onChoose(match)} />)}
  </ScrollView>;
}

export function FavoritesSheet({ visible, model, onClose, onChoose }: {
  visible: boolean; model: ReturnType<typeof useFavorites>; onClose: () => void; onChoose: (favorite: Favorite) => void;
}) {
  const selected = useRef<Favorite | null>(null);
  function chooseAfterDismiss() {
    const favorite = selected.current;
    selected.current = null;
    if (favorite) onChoose(favorite);
  }
  useEffect(() => { if (!visible && Platform.OS === 'android') chooseAfterDismiss(); }, [visible]);
  return <Modal visible={visible} transparent animationType="none" onRequestClose={onClose} onDismiss={chooseAfterDismiss} accessibilityLabel="Favoriten">
    <View style={menu.overlay}>
      <Pressable style={menu.backdrop} accessibilityRole="button" accessibilityLabel="Favoriten schließen" onPress={onClose} />
      <SafeAreaView style={menu.safeArea} edges={['top', 'bottom', 'left', 'right']} pointerEvents="box-none">
        <View style={menu.sheet} accessibilityViewIsModal>
          <View style={menu.header}><Text accessibilityRole="header" style={menu.title}>Favoriten</Text>
            <Button variant="ghost" icon="close" iconOnly label="Schließen" onPress={onClose} /></View>
          <ScrollView style={menu.scroll} contentContainerStyle={s.list} keyboardShouldPersistTaps="handled">
            <View style={s.hint}><Text style={s.detail}>Antippen fügt die gespeicherte Portion in deine Nachricht ein. Du kannst sie vor dem Senden anpassen.</Text></View>
            <ErrorNotice text={model.error} />
            {!!model.error && <Button secondary label="Favoriten erneut laden" onPress={model.reload} />}
            {!model.ready && !model.error && <Text style={s.detail}>Favoriten werden geladen …</Text>}
            {model.ready && !model.favorites.length && <View style={s.hint}><Text style={s.detail}>Markiere ein Gericht über den Stern bei deinen Tageseinträgen.</Text></View>}
            {model.ready && model.favorites.map(favorite => <View key={favorite.id} style={s.row}>
              <FavoriteChoice favorite={favorite} onPress={() => { selected.current = favorite; onClose(); }} />
              <Button variant="ghost" icon="star" iconOnly selected label="Favorit entfernen" accessibilityLabel={`${favorite.entry.label} aus Favoriten entfernen`}
                disabled={!!model.busy} loading={model.busy === favorite.id} onPress={() => void model.remove(favorite)} />
            </View>)}
          </ScrollView>
        </View>
      </SafeAreaView>
    </View>
  </Modal>;
}
