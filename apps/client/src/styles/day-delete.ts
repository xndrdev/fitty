import { StyleSheet } from 'react-native';
import { fonts } from './app';

const body = { fontFamily: fonts.body, color: '#253222' };

export const styles = StyleSheet.create({
  dayActions: { maxWidth: '100%', flexShrink: 1 },
  overlay: { flex: 1, padding: 16, justifyContent: 'center', alignItems: 'center', backgroundColor: '#14251CB8' },
  sheet: { width: '100%', maxWidth: 540, maxHeight: '100%', flexShrink: 1, backgroundColor: '#FBFAF4', borderRadius: 20, overflow: 'hidden' },
  header: { flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', gap: 12, padding: 20, borderBottomWidth: 1, borderBottomColor: '#E0DECF' },
  headingGroup: { flex: 1, minWidth: 0, gap: 6 },
  date: { ...body, color: '#62695B', fontSize: 12, lineHeight: 18 },
  title: { fontFamily: fonts.heading, color: '#1E241C', fontSize: 29, lineHeight: 33 },
  scroll: { flexShrink: 1 },
  content: { padding: 20, gap: 16 },
  counts: { backgroundColor: '#EFEEE2', padding: 16, borderRadius: 12, gap: 10 },
  count: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center', gap: 16 },
  text: { ...body, fontSize: 14, lineHeight: 21 },
  number: { ...body, fontSize: 16, fontWeight: '600' },
  hint: { ...body, color: '#62695B', fontSize: 12, lineHeight: 18 },
  warning: { ...body, color: '#982D23', fontSize: 14, lineHeight: 21, fontWeight: '600' },
  footer: { padding: 16, gap: 12, borderTopWidth: 1, borderTopColor: '#E0DECF', backgroundColor: '#FFFFFF' },
  actions: { flexDirection: 'row', flexWrap: 'wrap', justifyContent: 'flex-end', gap: 8 },
  deleteButton: { minHeight: 44, paddingHorizontal: 16, paddingVertical: 10, borderRadius: 10, backgroundColor: '#982D23', justifyContent: 'center', alignItems: 'center' },
  deleteText: { ...body, color: '#FFFFFF', fontSize: 13, fontWeight: '600' },
  disabled: { opacity: 0.5 },
  pressed: { opacity: 0.75 },
  focus: { outlineColor: '#2F5D3A', outlineWidth: 2, outlineStyle: 'solid', outlineOffset: 2 },
});
