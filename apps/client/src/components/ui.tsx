import { useState, type ComponentPropsWithRef } from 'react';
import { Pressable, Text, TextInput, View } from 'react-native';
import { styles as s } from '../styles/app';
import { Icon, IconName } from './icon';

export function Button({ label, onPress, disabled, secondary, selected, accessibilityLabel, icon, iconOnly, variant, expanded }: {
  label: string; onPress: () => void; disabled?: boolean; secondary?: boolean; selected?: boolean; accessibilityLabel?: string;
  icon?: IconName; iconOnly?: boolean; variant?: 'ghost' | 'chip'; expanded?: boolean;
}) {
  const [focused, setFocused] = useState(false);
  const light = secondary || !!variant;
  return <Pressable accessibilityRole="button" accessibilityLabel={accessibilityLabel ?? label} accessibilityState={{ disabled: !!disabled, ...(selected !== undefined ? { selected } : {}), ...(expanded !== undefined ? { expanded } : {}) }}
    onFocus={() => setFocused(true)} onBlur={() => setFocused(false)} disabled={disabled} onPress={onPress} style={({ pressed }) => [s.button, light && s.buttonSecondary, variant === 'ghost' && s.buttonGhost, variant === 'chip' && s.buttonChip, selected && s.buttonSelected, iconOnly && s.buttonIcon, disabled && s.disabled, pressed && s.pressed, focused && s.focus]}>
    {icon && <Icon name={icon} color={light ? '#2F5D3A' : '#FFFFFF'} />}
    {!iconOnly && <Text style={[s.buttonText, light && s.buttonSecondaryText, variant === 'chip' && s.chipText]}>{label}</Text>}
  </Pressable>;
}
export function Field({ label, ...props }: ComponentPropsWithRef<typeof TextInput> & { label: string }) {
  const [focused, setFocused] = useState(false);
  return <View style={s.field}><Text style={s.label}>{label}</Text><TextInput accessibilityLabel={label} placeholderTextColor="#687061" onFocus={() => setFocused(true)} onBlur={() => setFocused(false)} style={[s.input, props.multiline && s.inputMultiline, focused && s.focus]} {...props} /></View>;
}
export function ErrorNotice({ text }: { text: string }) {
  return text ? <Text accessibilityRole="alert" style={s.error}>{text}</Text> : null;
}
export function Brand() {
  return <View style={s.brand}><FittyMark /><Text style={s.brandName}>fitty</Text></View>;
}
export function FittyMark({ small = false }: { small?: boolean }) {
  return <View aria-hidden accessibilityElementsHidden style={[s.brandMark, small && s.botMark]}><Text style={[s.brandInitial, small && s.botInitial]}>f</Text></View>;
}
