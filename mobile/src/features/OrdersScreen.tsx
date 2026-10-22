import { useRef, useState } from 'react';
import { Modal, Pressable, SectionList, StyleSheet, Text, View } from 'react-native';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { X } from 'lucide-react-native';
import { Button, Card, DetailRow, Empty, IconButton, Notice, Segments } from '../components';
import { errorMessage } from '../api/client';
import { orderSchema, type Order } from '../api/schemas';
import { useConnection, exchangeData, useAccounts, useMarket, useOrders } from '../broker';
import { useSession } from '../session';
import { orderFailureMessage, orderStatusLabel, terminalStatuses } from '../domain/order';
import { colors, ui } from '../theme';
import { AccountFeedback, AccountPicker, NoAccess, QueryFeedback, RefreshButton, amount, priceAmount, dateText, decimalText, liveStyles as s } from './common';

export default function OrdersScreen() {
  const connection = useConnection();
  const readOnly = exchangeData && (!connection.data?.connected || connection.data.read_only !== false || connection.isError);
  const { api, can } = useSession();
  const accounts = useAccounts(), query = useOrders(accounts.account), market = useMarket();
  const [filter, setFilter] = useState('Все'), [cancelId, setCancelId] = useState<string | null>(null);
  const selected = query.orders.find(order => order.id === cancelId);
  const guard = useRef(false), cache = useQueryClient();
  const cancel = useMutation({ mutationFn: (id: string) => api.request(`/v1/orders/${id}/cancel`, orderSchema, { method: 'POST' }), retry: false,
    onSuccess: async () => { setCancelId(null); await Promise.all(['orders', 'cash', 'portfolio', 'ledger'].map(key => cache.invalidateQueries({ queryKey: [key] }))); } });
  const confirm = async () => { if (!selected || guard.current) return; guard.current = true; try { await cancel.mutateAsync(selected.id); } catch {} finally { guard.current = false; } };
  const groups = [
    { title: 'Активные', data: query.orders.filter(order => !terminalStatuses.has(order.status)) },
    { title: 'Исполненные', data: query.orders.filter(order => order.status === 'FILLED') },
    { title: 'Отменённые', data: query.orders.filter(order => order.status === 'CANCELLED') },
    { title: 'Другие заявки', data: query.orders.filter(order => terminalStatuses.has(order.status) && !['FILLED', 'CANCELLED'].includes(order.status)) },
  ];
  const sections = groups.filter(group => group.data.length && (filter === 'Все' || filter === group.title));
  const row = ({ item }: { item: Order }) => { const instrument = market.instruments.data?.items.find(asset => asset.id === item.instrument_id); return <Card style={o.order}>
    <View style={ui.between}><Text style={[ui.label, s.flex]}>{instrument?.name || item.symbol || item.instrument_id}</Text><Text style={o.side}>{item.side === 'BUY' ? 'Покупка' : 'Продажа'}</Text></View><Text style={ui.caption}>{instrument?.ticker || item.symbol} · {item.order_type === 'LIMIT' ? 'Лимитная' : item.order_type}</Text>
    <Text style={[o.status, (item.status === 'REJECTED' || item.status === 'FAILED') && o.rejected]}>{orderStatusLabel(item.status)}</Text><DetailRow label="Количество" value={`${decimalText(item.quantity)} шт.`} /><DetailRow label="Исполнено" value={`${decimalText(item.filled_quantity)} шт.`} /><DetailRow label="Лимитная цена" value={priceAmount(item.price ?? undefined, instrument?.currency ?? accounts.account?.currency)} />
    {item.reject_reason && <Text accessibilityRole="alert" style={s.errorText}>{orderFailureMessage(item.reject_reason)}</Text>}<Text style={ui.caption}>{dateText(item.created_at)}</Text>
    {!readOnly && item.can_cancel && can('orders.cancel') && <Button secondary style={o.cancel} onPress={() => { cancel.reset(); setCancelId(item.id); }}>Отменить заявку</Button>}
  </Card>; };
  return <View style={o.screen}><View style={o.top}><View style={ui.between}><Text style={ui.title}>Мои заявки</Text><RefreshButton busy={query.isFetching} onPress={() => { void query.refetch(); }} /></View>{can('accounts.read') && <AccountFeedback query={accounts} />}{accounts.accounts.length > 1 && <AccountPicker {...accounts} onSelect={accounts.selectAccount} />}<View style={o.summary}>{groups.slice(0, 3).map(group => <Pressable key={group.title} accessibilityRole="button" accessibilityLabel={`${group.title}: ${group.data.length}`} accessibilityState={{ selected: filter === group.title }} onPress={() => setFilter(value => value === group.title ? 'Все' : group.title)} style={[o.summaryTile, filter === group.title && o.summarySelected]}><Text style={o.summaryCount}>{query.isPending ? '—' : group.data.length}</Text><Text style={o.summaryLabel}>{group.title}</Text></Pressable>)}</View></View>
    {!can('orders.read') ? <View style={o.top}><NoAccess /></View> : accounts.account && <SectionList sections={sections} stickySectionHeadersEnabled={false} renderSectionHeader={({ section }) => <Text style={o.groupTitle}>{section.title}</Text>} keyExtractor={item => item.id} renderItem={row} contentContainerStyle={o.list} initialNumToRender={8} refreshing={query.isRefetching} onRefresh={() => { void query.refetch(); }} ListHeaderComponent={<QueryFeedback query={query} />} ListEmptyComponent={query.isSuccess ? <Empty title="Заявок пока нет" detail="Заявки по выбранному счёту появятся здесь после отправки." /> : undefined} />}
    <Modal visible={!!selected} transparent animationType="fade" onRequestClose={() => { if (!cancel.isPending) setCancelId(null); }}><View style={o.backdrop}><View style={o.sheet} accessibilityViewIsModal><View style={ui.between}><Text style={s.title}>Отменить заявку?</Text>{!cancel.isPending && <IconButton icon={X} label="Закрыть" onPress={() => setCancelId(null)} />}</View><Text style={ui.body}>{selected?.side === 'BUY' ? 'Покупка' : 'Продажа'} {selected?.quantity} шт.</Text><Notice>Биржа может исполнить заявку, пока запрос отмены обрабатывается. Окончательный статус появится в списке заявок.</Notice>{cancel.isError && <Text accessibilityRole="alert" style={s.errorText}>{errorMessage(cancel.error)}</Text>}<Button danger disabled={cancel.isPending || !selected?.can_cancel} onPress={() => { void confirm(); }}>{cancel.isPending ? 'Отправляем запрос…' : 'Отправить отмену'}</Button><Button secondary disabled={cancel.isPending} onPress={() => setCancelId(null)}>Оставить заявку</Button></View></View></Modal>
  </View>;
}
const o = StyleSheet.create({ summary: { flexDirection: 'row', gap: 9 }, summaryTile: { flex: 1, backgroundColor: '#FFF', borderRadius: 16, paddingHorizontal: 12, paddingVertical: 16, minHeight: 79, borderWidth: 1, borderColor: 'transparent' }, summarySelected: { borderColor: colors.navy }, summaryCount: { fontSize: 25, lineHeight: 31, fontWeight: '600', color: colors.ink }, summaryLabel: { fontSize: 10, color: colors.muted, marginTop: 5 }, groupTitle: { color: colors.ink, fontSize: 15, fontWeight: '600', marginTop: 5, marginBottom: 10 }, screen: { flex: 1, backgroundColor: colors.background }, top: { padding: 20, gap: 15 }, list: { paddingHorizontal: 20, paddingBottom: 28, gap: 15 }, order: { gap: 5 }, side: { color: colors.navy, fontSize: 12, fontWeight: '600' }, status: { color: colors.navy, fontSize: 12, fontWeight: '600', backgroundColor: colors.soft, alignSelf: 'flex-start', paddingHorizontal: 10, paddingVertical: 7, borderRadius: 8, marginTop: 12, marginBottom: 7 }, rejected: { color: colors.red, backgroundColor: colors.redSoft }, cancel: { marginTop: 16 }, backdrop: { flex: 1, backgroundColor: 'rgba(16,35,55,.38)', justifyContent: 'center', alignItems: 'center', padding: 22 }, sheet: { padding: 22, borderRadius: 23, gap: 17, backgroundColor: '#FFF', width: '100%', maxWidth: 400 } });
