import { ActivityIndicator, Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';
import { router } from 'expo-router';
import { ChevronRight, RefreshCw } from 'lucide-react-native';
import { formatAmount, formatPrice } from '../domain/format';
import { Button, Card, Empty } from '../components';
import { errorMessage } from '../api/client';
import type { Account, Instrument, Quote } from '../api/schemas';
import { useAccounts } from '../broker';
import { colors, ui } from '../theme';

export const amount = formatAmount;
export const priceAmount = formatPrice;
export function decimalText(value: string | undefined) { return value?.replace('.', ',') ?? '—'; }
export function dateText(value: string) { const date = new Date(value); return Number.isNaN(date.getTime()) ? value : date.toLocaleString('ru-RU', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' }); }
export function ledgerLabel(value: string) { return ({ DEPOSIT: 'Пополнение', WITHDRAWAL: 'Вывод средств', RESERVE: 'Резерв по заявке', RELEASE: 'Освобождение резерва', COMMISSION: 'Комиссия', TRADE: 'Расчёт по сделке', SETTLEMENT: 'Расчёт по сделке', BUY: 'Покупка', SELL: 'Продажа' } as Record<string, string>)[value] ?? 'Движение средств'; }
type QueryState = { isPending: boolean; isError: boolean; error: unknown; refetch: () => unknown; isFetching?: boolean };
export function QueryFeedback({ query }: { query: QueryState }) {
  if (query.isPending) return <Card style={s.loading}><ActivityIndicator color={colors.navy} /><Text style={ui.subtitle}>Загружаем данные…</Text></Card>;
  if (query.isError) return <Card style={s.error}><Text accessibilityRole="alert" style={ui.body}>{errorMessage(query.error)}</Text><Button secondary onPress={() => { void query.refetch(); }}>Повторить</Button></Card>;
  return null;
}
export function RefreshButton({ onPress, busy = false }: { onPress: () => void; busy?: boolean }) { return <Pressable accessibilityRole="button" accessibilityLabel="Обновить данные" disabled={busy} onPress={onPress} style={s.refresh}>{busy ? <ActivityIndicator size="small" color={colors.navy} /> : <RefreshCw size={19} strokeWidth={1.6} color={colors.navy} />}</Pressable>; }
export function NoAccess() { return <Empty title="Доступ ограничен" detail="У вашей учётной записи нет прав на этот раздел. Обратитесь к брокеру." />; }
export function AccountPicker({ accounts, account, onSelect }: { accounts: Account[]; account?: Account; onSelect: (id: string) => void }) {
  return <View style={s.picker}><Text style={ui.caption}>Брокерский счёт</Text><ScrollView horizontal showsHorizontalScrollIndicator={false} contentContainerStyle={s.accountList}>{accounts.map(item => <Pressable key={item.id} accessibilityRole="button" accessibilityState={{ selected: item.id === account?.id }} onPress={() => onSelect(item.id)} style={[s.accountChip, item.id === account?.id && s.selectedChip]}><Text style={[s.accountText, item.id === account?.id && s.selectedText]}>{item.account_number} · {item.currency}</Text></Pressable>)}</ScrollView>{account && account.status !== 'ACTIVE' && <Text style={s.blocked}>Счёт {account.status}. Торговля недоступна.</Text>}</View>;
}
export function AccountFeedback({ query }: { query: ReturnType<typeof useAccounts> }) {
  return query.isPending || query.isError ? <QueryFeedback query={query} /> : !query.account ? <Empty title="Торговый счёт ещё не открыт" detail="После открытия счёта у брокера здесь появятся ваши активы и история операций." /> : null;
}
export function SecurityAvatar({ name, ticker }: { name: string; ticker: string }) { return <View style={s.avatar}><Text style={s.initials}>{(ticker || name).slice(0, 2).toUpperCase()}</Text></View>; }
export function MarketRow({ instrument, quote, quantity, marketValue, hidden = false, last = false }: { instrument: Pick<Instrument, 'id' | 'name' | 'ticker' | 'currency'>; quote?: Quote; quantity?: string; marketValue?: string | null; hidden?: boolean; last?: boolean }) {
  return <Pressable accessibilityRole="button" accessibilityLabel={`Открыть ${instrument.name}`} onPress={() => router.push({ pathname: '/instrument/[id]', params: { id: instrument.id } })} style={[s.marketRow, !last && s.border]}><SecurityAvatar name={instrument.name} ticker={instrument.ticker} /><View style={s.name}><Text numberOfLines={2} style={s.securityName}>{instrument.name}</Text><Text style={ui.caption}>{quantity === undefined ? instrument.ticker : `${hidden ? '•••' : decimalText(quantity)} шт. · ${instrument.ticker}`}</Text></View><View style={s.price}><Text style={s.priceText}>{marketValue !== undefined ? amount(marketValue, instrument.currency, hidden) : priceAmount(quote?.last, instrument.currency, hidden)}</Text></View></Pressable>;
}
export const liveStyles = StyleSheet.create({
  actions: { flexDirection: 'row', gap: 10 }, flex: { flex: 1 }, listCard: { paddingVertical: 0, paddingHorizontal: 15 },
  total: { color: colors.ink, fontSize: 35, fontWeight: '600', letterSpacing: -1.1, marginVertical: 12 },
  mutedNote: { color: colors.muted, fontSize: 12, lineHeight: 19 }, errorText: { color: colors.red, fontSize: 13, lineHeight: 20 },
  empty: { gap: 12 }, title: { fontSize: 20, fontWeight: '600', color: colors.ink },
});
const s = StyleSheet.create({ loading: { alignItems: 'center', gap: 13, paddingVertical: 30 }, error: { gap: 16 }, refresh: { minHeight: 44, minWidth: 44, alignItems: 'center', justifyContent: 'center' }, picker: { gap: 9 }, accountList: { gap: 8 }, accountChip: { minHeight: 44, paddingHorizontal: 13, paddingVertical: 12, borderRadius: 12, backgroundColor: '#FFF', borderWidth: 1, borderColor: colors.line }, selectedChip: { backgroundColor: colors.soft, borderColor: '#D4DFEB' }, accountText: { fontSize: 12, color: colors.muted }, selectedText: { color: colors.navy, fontWeight: '600' }, blocked: { color: colors.red, fontSize: 12 }, avatar: { width: 43, height: 43, borderRadius: 13, alignItems: 'center', justifyContent: 'center', backgroundColor: colors.soft }, initials: { color: colors.navy, fontSize: 13, fontWeight: '700' }, marketRow: { flexDirection: 'row', alignItems: 'center', gap: 10, paddingVertical: 17 }, border: { borderBottomWidth: 1, borderBottomColor: colors.line }, name: { flex: 1, gap: 4 }, securityName: { color: colors.ink, fontSize: 13, fontWeight: '600' }, price: { alignItems: 'flex-end', gap: 6, maxWidth: '35%' }, priceText: { color: colors.ink, fontSize: 13, fontWeight: '600' }, source: { fontSize: 9, color: colors.muted } });

