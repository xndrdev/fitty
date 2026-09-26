import { StyleSheet } from 'react-native';
import { fonts } from './app';

export const styles = StyleSheet.create({
  overlay: { flex: 1, justifyContent: 'flex-end', backgroundColor: '#14251C66' },
  backdrop: { position: 'absolute', top: 0, right: 0, bottom: 0, left: 0 },
  safeArea: { flex: 1, justifyContent: 'flex-end', padding: 12 },
  sheet: { width: '100%', maxWidth: 440, maxHeight: '100%', alignSelf: 'center', flexShrink: 1, borderRadius: 24, backgroundColor: '#FBFAF4', overflow: 'hidden' },
  header: { flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', paddingLeft: 22, paddingRight: 10, paddingTop: 10, paddingBottom: 4 },
  title: { fontFamily: fonts.body, fontSize: 17, fontWeight: '600', color: '#1E241C' },
  scroll: { flexGrow: 0, flexShrink: 1 },
  actions: { paddingHorizontal: 12, paddingBottom: 12, gap: 2 },
  action: { minHeight: 48, flexDirection: 'row', alignItems: 'center', gap: 14, paddingHorizontal: 12, paddingVertical: 12, borderRadius: 12 },
  label: { fontFamily: fonts.body, fontSize: 16, color: '#1E241C', flexShrink: 1 },
  photoActions: { flexDirection: 'row', gap: 8, paddingBottom: 8, marginBottom: 6, borderBottomWidth: 1, borderColor: '#E0DECF' },
  photoAction: { flex: 1, backgroundColor: '#EFEEE5', borderRadius: 12 },
  hint: { fontFamily: fonts.body, fontSize: 12, lineHeight: 18, color: '#62695B', paddingHorizontal: 12 },
  discard: { marginTop: 6, paddingTop: 6, borderTopWidth: 1, borderColor: '#E0DECF' },
  focus: { outlineColor: '#2F5D3A', outlineWidth: 2, outlineStyle: 'solid', outlineOffset: -2 },
  pressed: { backgroundColor: '#E7EEE0' },
  disabled: { opacity: 0.5 },
});
