import { StyleSheet } from 'react-native';
import { fonts } from './app';

export const styles = StyleSheet.create({
  suggestions: { flexGrow: 0, flexShrink: 0 },
  suggestionList: { gap: 8, paddingVertical: 2, paddingHorizontal: 2 },
  suggestion: { minHeight: 48, maxWidth: 220, flexDirection: 'row', alignItems: 'center', gap: 9, paddingVertical: 8, paddingHorizontal: 12, borderRadius: 14, backgroundColor: '#E7EEE4' },
  text: { flexShrink: 1, gap: 2 },
  label: { fontFamily: fonts.body, fontSize: 13, lineHeight: 18, fontWeight: '600', color: '#253222' },
  detail: { fontFamily: fonts.body, fontSize: 12, lineHeight: 17, color: '#62695B' },
  row: { flexDirection: 'row', alignItems: 'center', gap: 4, borderBottomWidth: 1, borderBottomColor: '#E0DECF', paddingVertical: 4 },
  choose: { minHeight: 52, flex: 1, padding: 10, borderRadius: 12, gap: 3 },
  list: { paddingHorizontal: 12, paddingBottom: 16, gap: 6 },
  hint: { padding: 10 },
});
