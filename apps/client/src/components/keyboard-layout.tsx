import { createContext, useContext, useEffect, useRef, useState, type ReactNode, type RefObject } from 'react';
import { Keyboard, KeyboardAvoidingView, Platform, ScrollView, type ScrollViewProps, TextInput, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import { styles as s } from '../styles/app';
import { Button } from './ui';

const KeyboardVisible = createContext(false);
export const useKeyboardVisible = () => useContext(KeyboardVisible);

// Keep avoidance at the screen origin, outside headers and safe-area padding.
// Modal screens need their own instance because they use a separate native window.
export function KeyboardScreen({ children, modal = false }: { children: ReactNode; modal?: boolean }) {
  const [visible, setVisible] = useState(() => Platform.OS !== 'web' && Keyboard.isVisible());
  useEffect(() => {
    if (Platform.OS === 'web') return;
    const show = Keyboard.addListener(Platform.OS === 'ios' ? 'keyboardWillShow' : 'keyboardDidShow', () => setVisible(true));
    const hide = Keyboard.addListener(Platform.OS === 'ios' ? 'keyboardWillHide' : 'keyboardDidHide', () => setVisible(false));
    return () => { show.remove(); hide.remove(); };
  }, []);

  return <KeyboardVisible.Provider value={visible}>
    <KeyboardAvoidingView style={[s.screen, modal && s.modalBackdrop]} enabled={Platform.OS !== 'web'} behavior={Platform.OS === 'ios' ? 'padding' : 'height'}>
      <SafeAreaView style={s.fill} edges={visible ? ['top', 'left', 'right'] : ['top', 'bottom', 'left', 'right']}>
        <View style={s.fill}>{children}</View>
        {visible && <View style={s.keyboardToolbar}>
          <Button variant="ghost" label="Fertig" accessibilityLabel="Tastatur schließen" onPress={Keyboard.dismiss} />
        </View>}
      </SafeAreaView>
    </KeyboardAvoidingView>
  </KeyboardVisible.Provider>;
}

// A resized form must also reveal its focused input, including numeric fields
// and fields above a modal's fixed save buttons.
export function FormScrollView({ scrollRef, children, onScroll, onLayout, onFocus, onBlur, style, ...props }: ScrollViewProps & { scrollRef?: RefObject<ScrollView | null> }) {
  const ownRef = useRef<ScrollView>(null);
  const scroll = scrollRef ?? ownRef;
  const offset = useRef(0);
  const focusedWithin = useRef(false);
  const frame = useRef<number | null>(null);
  function revealInput() {
    if (Platform.OS === 'web' || !focusedWithin.current) return;
    if (frame.current !== null) cancelAnimationFrame(frame.current);
    frame.current = requestAnimationFrame(() => {
      frame.current = null;
      const input = TextInput.State.currentlyFocusedInput();
      if (!input || !scroll.current) return;
      scroll.current.getNativeScrollRef()?.measureInWindow((_x, top, _width, height) => {
        input.measureInWindow((_inputX, inputTop, _inputWidth, inputHeight) => {
          if (!height || !inputHeight) return;
          const bottom = top + height - 12;
          const delta = inputHeight > height - 24 || inputTop < top + 12
            ? inputTop - top - 12 : Math.max(0, inputTop + inputHeight - bottom);
          if (delta) scroll.current?.scrollTo({ y: Math.max(0, offset.current + delta), animated: true });
        });
      });
    });
  }
  useEffect(() => {
    if (Platform.OS === 'web') return;
    const shown = Keyboard.addListener('keyboardDidShow', revealInput);
    return () => { shown.remove(); if (frame.current !== null) cancelAnimationFrame(frame.current); };
  }, []);

  return <ScrollView {...props} ref={scroll} style={[s.formScroll, style]} keyboardShouldPersistTaps="handled"
    keyboardDismissMode={Platform.OS === 'ios' ? 'interactive' : 'on-drag'} scrollEventThrottle={16}
    onFocus={event => {
      focusedWithin.current = true;
      if (Platform.OS === 'web') (event.target as unknown as HTMLElement).scrollIntoView({ block: 'nearest' });
      else revealInput();
      onFocus?.(event);
    }}
    onBlur={event => { focusedWithin.current = false; onBlur?.(event); }}
    onLayout={event => { if (Platform.OS !== 'web' && Keyboard.isVisible()) revealInput(); onLayout?.(event); }}
    onScroll={event => { offset.current = event.nativeEvent.contentOffset.y; onScroll?.(event); }}>
    {children}
  </ScrollView>;
}
