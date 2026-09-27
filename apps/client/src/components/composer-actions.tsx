import { useEffect, useRef, useState } from 'react';
import { Modal, Platform, Pressable, ScrollView, Text, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import { Icon, type IconName } from './icon';
import { Button } from './ui';
import { styles as s } from '../styles/composer-actions';

export type ComposerAction = 'meal' | 'activity' | 'restaurant' | 'photos' | 'camera' | 'discard' | 'favorites';

function Action({ label, icon, disabled, onPress }: { label: string; icon: IconName; disabled?: boolean; onPress: () => void }) {
  const [focused, setFocused] = useState(false);
  return <Pressable accessibilityRole="button" accessibilityLabel={label} accessibilityState={{ disabled: !!disabled }} disabled={disabled}
    onPress={onPress} onFocus={() => setFocused(true)} onBlur={() => setFocused(false)}
    style={({ pressed }) => [s.action, focused && s.focus, pressed && s.pressed, disabled && s.disabled]}>
    <Icon name={icon} size={21} /><Text style={s.label}>{label}</Text>
  </Pressable>;
}

export function ComposerActions({ visible, photosEnabled, photoLimitReached, canDiscard, onClose, onAction }: {
  visible: boolean; photosEnabled: boolean; photoLimitReached: boolean; canDiscard: boolean;
  onClose: () => void; onAction: (action: ComposerAction) => void;
}) {
  const selected = useRef<ComposerAction | null>(null);
  function runAction() {
    const action = selected.current;
    selected.current = null;
    if (action) onAction(action);
  }
  // iOS and web must finish dismissing their modal before focusing another
  // input or opening another modal. Android has no onDismiss callback.
  useEffect(() => { if (!visible && Platform.OS === 'android') runAction(); }, [visible]);
  function choose(action: ComposerAction) { selected.current = action; onClose(); }

  return <Modal visible={visible} transparent animationType="none" onRequestClose={onClose} onDismiss={runAction} accessibilityLabel="Nachricht ergänzen">
    <View style={s.overlay}>
      <Pressable style={s.backdrop} accessibilityRole="button" accessibilityLabel="Menü schließen" onPress={onClose} />
      <SafeAreaView style={s.safeArea} edges={['top', 'bottom', 'left', 'right']} pointerEvents="box-none">
        <View style={s.sheet} accessibilityViewIsModal>
          <View style={s.header}><Text accessibilityRole="header" style={s.title}>Hinzufügen</Text>
            <Button variant="ghost" icon="close" iconOnly label="Schließen" accessibilityLabel="Hinzufügen schließen" onPress={onClose} />
          </View>
          <ScrollView style={s.scroll} contentContainerStyle={s.actions} keyboardShouldPersistTaps="handled">
            {photosEnabled && <View style={s.photoActions}>
              <View style={s.photoAction}><Action label="Fotos" icon="photo" disabled={photoLimitReached} onPress={() => choose('photos')} /></View>
              <View style={s.photoAction}><Action label="Kamera" icon="camera" disabled={photoLimitReached} onPress={() => choose('camera')} /></View>
            </View>}
            {photosEnabled && photoLimitReached && <Text style={s.hint}>Höchstens vier Fotos pro Nachricht.</Text>}
            <Action label="Favoriten" icon="star" onPress={() => choose('favorites')} />
            <Action label="Mahlzeit" icon="meal" onPress={() => choose('meal')} />
            <Action label="Bewegung" icon="activity" onPress={() => choose('activity')} />
            <Action label="Restaurant" icon="restaurant" onPress={() => choose('restaurant')} />
            {canDiscard && <View style={s.discard}><Action label="Entwurf verwerfen" icon="trash" onPress={() => choose('discard')} /></View>}
          </ScrollView>
        </View>
      </SafeAreaView>
    </View>
  </Modal>;
}
