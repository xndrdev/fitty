import { StyleSheet } from 'react-native';
import { fonts } from './app';

export const styles = StyleSheet.create({
  overlay: { flex: 1, backgroundColor: '#14251C66' },
  backdrop: { position: 'absolute', top: 0, right: 0, bottom: 0, left: 0 },
  safeArea: { flex: 1, padding: 12 },
  sheet: { width: '100%', maxWidth: 360, maxHeight: '100%', flexShrink: 1, borderRadius: 24, backgroundColor: '#FBFAF4', overflow: 'hidden' },
  header: { flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', paddingLeft: 22, paddingRight: 10, paddingTop: 10, paddingBottom: 8 },
  scroll: { flexGrow: 0, flexShrink: 1 },
  content: { paddingHorizontal: 12, paddingBottom: 12, gap: 2 },
  action: { minHeight: 48, flexDirection: 'row', alignItems: 'center', gap: 14, paddingHorizontal: 12, paddingVertical: 12, borderRadius: 12 },
  label: { fontFamily: fonts.body, fontSize: 16, color: '#1E241C', flexShrink: 1 },
  section: { marginTop: 8, paddingTop: 8, borderTopWidth: 1, borderColor: '#E0DECF', gap: 2 },
  caption: { fontFamily: fonts.body, fontSize: 12, lineHeight: 18, color: '#62695B', paddingHorizontal: 12, paddingVertical: 4 },
  dayActions: { flexDirection: 'row', alignItems: 'center', gap: 8, paddingHorizontal: 12, paddingVertical: 4 },
  today: { flex: 1, minWidth: 0 },
  selected: { backgroundColor: '#E7EEE0' },
  focus: { outlineColor: '#2F5D3A', outlineWidth: 2, outlineStyle: 'solid', outlineOffset: -2 },
  pressed: { opacity: 0.75 },
  disabled: { opacity: 0.5 },
});
