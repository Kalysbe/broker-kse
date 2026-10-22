import { useEffect, useRef, type ComponentRef, type ReactNode } from 'react';
import { Pressable, ScrollView, StyleSheet, Text, View, type ViewStyle } from 'react-native';
import { router } from 'expo-router';
import { ArrowLeft, ArrowUpRight, ChevronRight, Info, type LucideIcon } from 'lucide-react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { colors, ui } from './theme';

export function Page({ children, title, subtitle, right, back = false, onBack, surface = false, contentStyle }: { children: ReactNode; title?: string; subtitle?: string; right?: ReactNode; back?: boolean; onBack?: () => void; surface?: boolean; contentStyle?: ViewStyle }) {
  const insets = useSafeAreaInsets();
  const scroll = useRef<ComponentRef<typeof ScrollView>>(null);
  useEffect(() => { scroll.current?.scrollTo({ y: 0, animated: false }); }, [title]);
  return <ScrollView ref={scroll} style={[s.page, surface && { backgroundColor: colors.surface }]} contentContainerStyle={[s.pageContent, contentStyle, { paddingBottom: Math.max(28, insets.bottom + 16) }]} keyboardShouldPersistTaps="handled" showsVerticalScrollIndicator={false}>
    {title && <View style={s.heading}>
      <View style={ui.between}><View style={s.headingText}>{back && <IconButton icon={ArrowLeft} label="Назад" onPress={onBack ?? (() => router.canGoBack() ? router.back() : router.replace('/'))} />}<Text style={ui.title}>{title}</Text></View>{right}</View>
      {subtitle && <Text style={ui.subtitle}>{subtitle}</Text>}
    </View>}
    {children}
  </ScrollView>;
}
export function Card({ children, style }: { children: ReactNode; style?: ViewStyle }) { return <View style={[ui.card, style]}>{children}</View>; }
export function Section({ title, action, onPress }: { title: string; action?: string; onPress?: () => void }) {
  return <View style={s.section}><Text style={s.sectionTitle}>{title}</Text>{action && <Pressable accessibilityRole="button" onPress={onPress} style={s.sectionAction}><Text style={s.link}>{action}</Text><ChevronRight size={14} color={colors.navy} /></Pressable>}</View>;
}
export function Button({ children, onPress, secondary = false, danger = false, disabled = false, icon: Icon, style }: { children: ReactNode; onPress: () => void; secondary?: boolean; danger?: boolean; disabled?: boolean; icon?: LucideIcon; style?: ViewStyle }) {
  return <Pressable accessibilityRole="button" accessibilityState={{ disabled }} disabled={disabled} onPress={onPress} style={({ pressed }) => [s.button, secondary ? s.secondary : danger ? s.danger : s.primary, style, (pressed || disabled) && s.dim]}>{Icon && <Icon size={18} color={secondary ? colors.navy : '#FFF'} />}<Text style={[s.buttonText, secondary && s.secondaryText]}>{children}</Text></Pressable>;
}
export function IconButton({ icon: Icon, label, onPress, active = false }: { icon: LucideIcon; label: string; onPress: () => void; active?: boolean }) {
  return <Pressable accessibilityRole="button" accessibilityLabel={label} accessibilityState={{ selected: active }} onPress={onPress} style={({ pressed }) => [s.iconButton, pressed && s.dim]}><Icon size={22} strokeWidth={1.7} color={active ? colors.navy : colors.ink} fill={active ? colors.soft : 'none'} /></Pressable>;
}
export function Notice({ title, children, warning = false }: { title?: string; children: ReactNode; warning?: boolean }) {
  return <View style={[s.notice, warning && s.warning]}><Info size={17} color={warning ? colors.amber : colors.navy} /><View style={s.noticeContent}>{title && <Text style={[s.noticeTitle, warning && s.warningText]}>{title}</Text>}<Text style={[s.noticeText, warning && s.warningText]}>{children}</Text></View></View>;
}
export function Segments<T extends string>({ options, value, onChange }: { options: readonly T[]; value: T; onChange: (value: T) => void }) {
  return <View style={s.segments}>{options.map(option => <Pressable accessibilityRole="tab" accessibilityState={{ selected: value === option }} key={option} onPress={() => onChange(option)} style={[s.segment, value === option && s.segmentSelected]}><Text style={[s.segmentText, value === option && s.segmentTextSelected]}>{option}</Text></Pressable>)}</View>;
}
export function DetailRow({ label, value, green = false }: { label: string; value: string; green?: boolean }) { return <View style={s.detailRow}><Text style={s.detailLabel}>{label}</Text><Text style={[s.detailValue, green && { color: colors.green }]}>{value}</Text></View>; }
export function MenuRow({ icon: Icon, title, detail, onPress, last = false }: { icon: LucideIcon; title: string; detail?: string; onPress: () => void; last?: boolean }) {
  return <Pressable accessibilityRole="button" onPress={onPress} style={[s.menuRow, !last && s.rowBorder]}><View style={s.menuIcon}><Icon size={20} strokeWidth={1.6} color={colors.navy} /></View><View style={s.menuText}><Text style={ui.body}>{title}</Text>{detail && <Text style={ui.caption}>{detail}</Text>}</View><ChevronRight size={17} color={colors.muted} /></Pressable>;
}
export function Empty({ title, detail, action, onPress }: { title: string; detail: string; action?: string; onPress?: () => void }) { return <Card style={s.empty}><View style={s.emptyIcon}><ArrowUpRight size={25} color={colors.navy} /></View><Text style={ui.label}>{title}</Text><Text style={[ui.subtitle, s.center]}>{detail}</Text>{action && onPress && <Button secondary onPress={onPress}>{action}</Button>}</Card>; }

const s = StyleSheet.create({
  rowBorder: { borderBottomWidth: 1, borderBottomColor: colors.line },
  page: { flex: 1, backgroundColor: colors.background }, pageContent: { padding: 20, paddingBottom: 28, gap: 16 },
  heading: { gap: 4, marginTop: 6, marginBottom: 5 }, headingText: { flexDirection: 'row', alignItems: 'center', flex: 1, gap: 7, flexWrap: 'wrap' },
  section: { flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', marginTop: 7, marginBottom: -6, gap: 6 },
  sectionTitle: { fontSize: 17, fontWeight: '600', color: colors.ink, letterSpacing: -0.3 }, sectionAction: { flexDirection: 'row', alignItems: 'center', minHeight: 44, gap: 3 }, link: { color: colors.navy, fontSize: 12, fontWeight: '500' },
  button: { minHeight: 52, paddingHorizontal: 16, paddingVertical: 14, borderRadius: 14, flexDirection: 'row', alignItems: 'center', justifyContent: 'center', gap: 8 },
  primary: { backgroundColor: colors.navy }, secondary: { backgroundColor: colors.soft }, danger: { backgroundColor: colors.red }, buttonText: { color: '#FFF', fontSize: 15, fontWeight: '600' }, secondaryText: { color: colors.navy }, dim: { opacity: 0.5 },
  iconButton: { minWidth: 44, minHeight: 44, alignItems: 'center', justifyContent: 'center' },
  notice: { backgroundColor: colors.soft, borderRadius: 14, padding: 14, flexDirection: 'row', gap: 9, alignItems: 'flex-start' }, warning: { backgroundColor: colors.amberSoft, borderWidth: 1, borderColor: colors.amberLine }, noticeContent: { flex: 1, gap: 4 }, noticeTitle: { color: colors.navy, fontSize: 12, fontWeight: '600', lineHeight: 17 }, noticeText: { color: colors.navy, fontSize: 11, lineHeight: 17 }, warningText: { color: colors.amber },
  segments: { flexDirection: 'row', padding: 4, backgroundColor: colors.soft, borderRadius: 13, gap: 3 }, segment: { flex: 1, minHeight: 44, alignItems: 'center', justifyContent: 'center', borderRadius: 10 }, segmentSelected: { backgroundColor: '#FFF' }, segmentText: { color: colors.muted, fontSize: 12, fontWeight: '500' }, segmentTextSelected: { color: colors.navy, fontWeight: '600' },
  detailRow: { flexDirection: 'row', alignItems: 'flex-start', justifyContent: 'space-between', gap: 14, paddingVertical: 9 }, detailLabel: { color: colors.muted, fontSize: 13, lineHeight: 19, flex: 1 }, detailValue: { color: colors.ink, fontSize: 13, fontWeight: '500', lineHeight: 19, textAlign: 'right', flexShrink: 1 },
  menuRow: { flexDirection: 'row', alignItems: 'center', gap: 12, paddingVertical: 17, minHeight: 62 }, menuIcon: { width: 32, alignItems: 'center' }, menuText: { flex: 1, gap: 2 },
  empty: { alignItems: 'center', paddingVertical: 32, gap: 12 }, emptyIcon: { width: 52, height: 52, borderRadius: 26, backgroundColor: colors.soft, alignItems: 'center', justifyContent: 'center', marginBottom: 5 }, center: { textAlign: 'center' },
});

