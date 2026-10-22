import { StyleSheet } from 'react-native';

export const colors = {
  background: '#F4F6FA', surface: '#FFFFFF', ink: '#182C43', navy: '#193E65',
  muted: '#7C8797', line: '#E9EDF3', soft: '#EAF0F7', green: '#238268',
  greenSoft: '#EAF5F0', red: '#C4565C', redSoft: '#FBEFF0', amber: '#936F30',
  amberSoft: '#FFF8E8', amberLine: '#F0E3BD',
};

export const ui = StyleSheet.create({
  row: { flexDirection: 'row', alignItems: 'center' },
  between: { flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', gap: 12 },
  card: { backgroundColor: colors.surface, borderRadius: 22, padding: 20 },
  title: { color: colors.ink, fontSize: 24, fontWeight: '700', letterSpacing: -0.7 },
  subtitle: { color: colors.muted, fontSize: 13, lineHeight: 20 },
  label: { color: colors.ink, fontSize: 15, fontWeight: '600' },
  caption: { color: colors.muted, fontSize: 12, lineHeight: 18 },
  body: { color: colors.ink, fontSize: 14, lineHeight: 21 },
  number: { color: colors.ink, fontSize: 16, fontWeight: '600', fontVariant: ['tabular-nums'] },
  divider: { height: 1, backgroundColor: colors.line, marginVertical: 15 },
  input: { backgroundColor: colors.surface, borderRadius: 14, borderWidth: 1, borderColor: colors.line, minHeight: 54, paddingHorizontal: 16, color: colors.ink, fontSize: 17 },
});

