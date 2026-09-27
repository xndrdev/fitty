import { useEffect, useRef, useState } from 'react';
import { Modal, Platform, Pressable, ScrollView, Text, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import { dateLabel } from '../lib/chat-api';
import { Icon, type IconName } from './icon';
import { Brand, Button } from './ui';
import { styles as s } from '../styles/workspace-menu';

export type WorkspacePane = 'chat' | 'history' | 'profile' | 'progress';
export type WorkspaceAction = WorkspacePane | 'previous' | 'next' | 'today' | 'date' | 'delete' | 'logout';

function Action({ label, icon, selected, disabled, onPress }: {
  label: string; icon: IconName; selected?: boolean; disabled?: boolean; onPress: () => void;
}) {
  const [focused, setFocused] = useState(false);
  return <Pressable accessibilityRole="button" accessibilityLabel={label}
    accessibilityState={{ disabled: !!disabled, ...(selected !== undefined ? { selected } : {}) }}
    disabled={disabled} onPress={onPress} onFocus={() => setFocused(true)} onBlur={() => setFocused(false)}
    style={({ pressed }) => [s.action, selected && s.selected, focused && s.focus, pressed && s.pressed, disabled && s.disabled]}>
    <Icon name={icon} size={21} /><Text style={s.label}>{label}</Text>
  </Pressable>;
}

export function WorkspaceMenu({ visible, pane, date, today, disabled, onClose, onAction }: {
  visible: boolean; pane: WorkspacePane; date: string; today: string; disabled: boolean;
  onClose: () => void; onAction: (action: WorkspaceAction) => void;
}) {
  const selected = useRef<WorkspaceAction | null>(null);
  function runAction() {
    const action = selected.current;
    selected.current = null;
    if (action) onAction(action);
  }
  // iOS and web release the current modal before the next action takes focus.
  useEffect(() => { if (!visible && Platform.OS === 'android') runAction(); }, [visible]);
  function choose(action: WorkspaceAction) {
    if (disabled || selected.current) return;
    selected.current = action; onClose();
  }

  return <Modal visible={visible} transparent animationType="none" onRequestClose={onClose} onDismiss={runAction} accessibilityLabel="Navigation">
    <View style={s.overlay}>
      <Pressable style={s.backdrop} accessibilityRole="button" accessibilityLabel="Menü schließen" onPress={onClose} />
      <SafeAreaView style={s.safeArea} edges={['top', 'bottom', 'left', 'right']} pointerEvents="box-none">
        <View style={s.sheet} accessibilityViewIsModal>
          <View style={s.header}><Brand /><Button variant="ghost" icon="close" iconOnly label="Menü schließen" onPress={onClose} /></View>
          <ScrollView style={s.scroll} contentContainerStyle={s.content} keyboardShouldPersistTaps="handled">
            <Action label="Tageschat" icon="meal" selected={pane === 'chat'} disabled={disabled} onPress={() => choose('chat')} />
            <Action label="Verlauf" icon="history" selected={pane === 'history'} disabled={disabled} onPress={() => choose('history')} />
            <Action label="Ziele & Vorlieben" icon="user" selected={pane === 'profile'} disabled={disabled} onPress={() => choose('profile')} />
            <Action label="Fortschrittsfotos" icon="photo" selected={pane === 'progress'} disabled={disabled} onPress={() => choose('progress')} />
            {!!date && <View style={s.section}>
              <Text style={s.caption}>{dateLabel(date, true)}</Text>
              <View style={s.dayActions}>
                <Button secondary icon="left" iconOnly label="Vorheriger Tag" disabled={disabled || date === '1900-01-01'} onPress={() => choose('previous')} />
                <View style={s.today}><Button secondary label="Heute" selected={date === today} disabled={disabled || !today} onPress={() => choose('today')} /></View>
                <Button secondary icon="right" iconOnly label="Nächster Tag" disabled={disabled || date === '9999-12-31'} onPress={() => choose('next')} />
              </View>
              <Action label="Datum wählen" icon="calendar" disabled={disabled} onPress={() => choose('date')} />
              <Action label="Tageschat löschen" icon="trash" disabled={disabled} onPress={() => choose('delete')} />
            </View>}
            <View style={s.section}><Action label="Abmelden" icon="logout" disabled={disabled} onPress={() => choose('logout')} /></View>
          </ScrollView>
        </View>
      </SafeAreaView>
    </View>
  </Modal>;
}
