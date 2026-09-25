import { StyleSheet } from 'react-native';
import { fonts } from './app';

const body = { fontFamily: fonts.body, color: '#253222' };

export const styles = StyleSheet.create({
  overlay: { flex: 1, padding: 16, justifyContent: 'center', alignItems: 'center', backgroundColor: '#14251CB8' },
  sheet: { width: '100%', maxWidth: 600, maxHeight: '100%', flexShrink: 1, backgroundColor: '#FBFAF4', borderRadius: 20, overflow: 'hidden' },
  header: { flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', gap: 12, padding: 20, borderBottomWidth: 1, borderBottomColor: '#E0DECF' },
  headingGroup: { flex: 1, minWidth: 0, gap: 4 },
  eyebrow: { ...body, color: '#62695B', fontSize: 10, fontWeight: '600', letterSpacing: 1.1, textTransform: 'uppercase' },
  title: { fontFamily: fonts.heading, color: '#1E241C', fontSize: 29, lineHeight: 33 },
  scroll: { flexShrink: 1 },
  content: { padding: 20, gap: 18 },
  field: { gap: 7 },
  label: { ...body, fontSize: 13, lineHeight: 19, fontWeight: '600' },
  input: { ...body, minHeight: 48, borderWidth: 1, borderColor: '#B3B7A8', borderRadius: 10, paddingHorizontal: 12, paddingVertical: 11, backgroundColor: '#FFFFFF', fontSize: 16 },
  notes: { minHeight: 86, textAlignVertical: 'top' },
  inputInvalid: { borderColor: '#982D23' },
  hint: { ...body, color: '#62695B', fontSize: 12, lineHeight: 18 },
  text: { ...body, fontSize: 14, lineHeight: 21 },
  notice: { padding: 14, borderRadius: 12, backgroundColor: '#EFEEE2', gap: 8 },
  conflict: { backgroundColor: '#F6EED8' },
  error: { ...body, color: '#982D23', fontSize: 12, lineHeight: 18 },
  numericFields: { flexDirection: 'row', flexWrap: 'wrap', gap: 14 },
  numericField: { flexGrow: 1, flexBasis: 150, minWidth: 0 },
  actions: { flexDirection: 'row', flexWrap: 'wrap', alignItems: 'center', gap: 8 },
  footer: { padding: 16, gap: 10, borderTopWidth: 1, borderTopColor: '#E0DECF', backgroundColor: '#FFFFFF' },
  footerActions: { flexDirection: 'row', flexWrap: 'wrap', justifyContent: 'flex-end', gap: 8 },
  deleteButton: { minHeight: 44, paddingHorizontal: 16, paddingVertical: 10, borderRadius: 10, backgroundColor: '#982D23', justifyContent: 'center', alignItems: 'center' },
  deleteText: { ...body, color: '#FFFFFF', fontSize: 13, fontWeight: '600' },
  entryName: { ...body, fontSize: 18, lineHeight: 25, fontWeight: '600' },
  disabled: { opacity: 0.5 },
  pressed: { opacity: 0.75 },
  focus: { outlineColor: '#2F5D3A', outlineWidth: 2, outlineStyle: 'solid', outlineOffset: 2 },
});
