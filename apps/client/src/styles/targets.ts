import { StyleSheet } from 'react-native';
import { fonts } from './app';

const text = { fontFamily: fonts.body, color: '#253222' };

export const styles = StyleSheet.create({
  section: { gap: 14, borderWidth: 1, borderColor: '#E0DECF', borderRadius: 18, padding: 18, backgroundColor: '#FFFFFF' },
  heading: { fontFamily: fonts.heading, fontSize: 28, lineHeight: 33, color: '#1E241C' },
  hint: { ...text, fontSize: 12, lineHeight: 18, color: '#62695B' },
  success: { ...text, fontSize: 13, lineHeight: 19, color: '#2F5D3A' },
  fields: { flexDirection: 'row', flexWrap: 'wrap', gap: 14 },
  field: { gap: 7, flexGrow: 1, flexBasis: 180, minWidth: 0 },
  label: { ...text, fontSize: 13, lineHeight: 19, fontWeight: '600' },
  input: { ...text, fontSize: 16, minHeight: 48, minWidth: 0, borderWidth: 1, borderColor: '#B3B7A8', borderRadius: 10, paddingHorizontal: 12, paddingVertical: 11, backgroundColor: '#FFFFFF' },
  invalid: { borderColor: '#982D23' },
  error: { ...text, fontSize: 12, lineHeight: 18, color: '#982D23' },
  focus: { outlineColor: '#2F5D3A', outlineWidth: 2, outlineStyle: 'solid', outlineOffset: 2 },
  notice: { padding: 14, borderRadius: 12, backgroundColor: '#F6EED8', gap: 10 },
  actions: { flexDirection: 'row', flexWrap: 'wrap', gap: 8 },
});
