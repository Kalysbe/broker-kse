import { useMemo, useState } from 'react';
import { Modal, Pressable, SectionList, StyleSheet, Switch, Text, TextInput, View } from 'react-native';
import { router, useLocalSearchParams } from 'expo-router';
import { z } from 'zod';
import { Search, SlidersHorizontal, Star, X } from 'lucide-react-native';
import { Button, Card, DetailRow, Empty, IconButton, Notice, Page, Segments } from '../components';
import { useConnection, exchangeData, useAccounts, useMarket, usePortfolio, usePreferences } from '../broker';
import { useSession } from '../session';
import { colors, ui } from '../theme';
import { uuidSchema, type Instrument } from '../api/schemas';
import { MarketRow, NoAccess, QueryFeedback, RefreshButton, SecurityAvatar, amount, priceAmount, liveStyles as s } from './common';

export function MarketScreen() {
  const { can } = useSession();
  const market = useMarket(), prefs = usePreferences();
  const [search, setSearch] = useState(''), [category, setCategory] = useState('Все');
  const [filtersOpen, setFiltersOpen] = useState(false), [favoritesOnly, setFavoritesOnly] = useState(false), [descending, setDescending] = useState(false);
  const isBond = (item: Instrument) => /BOND|DEBT/.test(item.type.toUpperCase()) || ['GO', 'GB'].includes(market.quotes.data?.items.find(quote => quote.id === item.id)?.security_type.toUpperCase() ?? '');
  const items = useMemo(() => (market.instruments.data?.items ?? []).filter(item => {
    const bond = isBond(item);
    return `${item.name} ${item.ticker} ${item.isin}`.toLowerCase().includes(search.trim().toLowerCase()) && (!favoritesOnly || prefs.favorites.includes(item.id)) && (category === 'Все' || category === 'Облигации' && bond || category === 'Акции' && !bond);
  }).sort((a, b) => a.name.localeCompare(b.name, 'ru') * (descending ? -1 : 1)), [market.instruments.data, market.quotes.data, category, search, favoritesOnly, descending, prefs.favorites]);
  const sections = [{ title: 'Акции', data: items.filter(item => !isBond(item)) }, { title: 'Облигации', data: items.filter(item => isBond(item)) }].filter(section => section.data.length > 0);
  const refresh = () => { void market.quotes.refetch(); void market.instruments.refetch(); };
  const countLabel = (count: number) => `${count} ${count % 10 === 1 && count % 100 !== 11 ? 'инструмент' : count % 10 >= 2 && count % 10 <= 4 && (count % 100 < 12 || count % 100 > 14) ? 'инструмента' : 'инструментов'}`;
  return <View style={m.page}><View style={m.top}><Text style={ui.title}>Рынок</Text>
    <View style={m.searchRow}><View style={m.search}><Search size={18} strokeWidth={1.5} color={colors.muted} /><TextInput accessibilityLabel="Поиск ценных бумаг" value={search} onChangeText={setSearch} placeholder="Название или тикер" placeholderTextColor={colors.muted} autoCorrect={false} style={m.input} /></View><Pressable accessibilityRole="button" accessibilityLabel="Фильтры рынка" onPress={() => setFiltersOpen(true)} style={m.filterButton}><SlidersHorizontal size={19} strokeWidth={1.5} color={favoritesOnly ? colors.navy : colors.muted} /></Pressable></View>
    <Segments options={['Все', 'Акции', 'Облигации']} value={category} onChange={setCategory} />
    <View style={m.sortRow}><Pressable accessibilityRole="button" accessibilityLabel="Изменить порядок сортировки" onPress={() => setDescending(value => !value)}><Text style={m.sortText}>Бумаги <Text style={m.sortValue}>Название {descending ? '↓' : '↑'}</Text></Text></Pressable><Pressable accessibilityRole="button" onPress={() => setFiltersOpen(true)}><Text style={m.sortValue}>Фильтры{favoritesOnly ? ' · 1' : ''}</Text></Pressable></View>
  </View>
    {!can('instruments.read') ? <View style={m.padding}><NoAccess /></View> : <SectionList sections={sections} keyExtractor={item => item.id} contentContainerStyle={m.list} stickySectionHeadersEnabled={false} initialNumToRender={12} keyboardShouldPersistTaps="handled" onRefresh={refresh} refreshing={market.instruments.isRefetching || market.quotes.isRefetching}
      ListHeaderComponent={<><QueryFeedback query={market.instruments} />{market.quotes.isError && <Notice>Не удалось обновить котировки. Потяните список вниз, чтобы повторить.</Notice>}</>}
      renderSectionHeader={({ section }) => <View style={m.sectionHeading}><Text style={m.sectionTitle}>{section.title}</Text><Text style={ui.caption}>{countLabel(section.data.length)}</Text></View>}
      renderItem={({ item, index, section }) => <View style={[m.listItem, index === 0 && m.firstRow, index === section.data.length - 1 && m.lastRow]}><MarketRow instrument={item} quote={market.quotes.data?.items.find(quote => quote.id === item.id)} last={index === section.data.length - 1} /></View>}
      renderSectionFooter={() => <View style={m.sectionSpace} />}
      ListEmptyComponent={!market.instruments.isPending && !market.instruments.isError ? <Empty title="Бумаги не найдены" detail="Измените поисковый запрос или категорию." action="Сбросить фильтры" onPress={() => { setSearch(''); setCategory('Все'); setFavoritesOnly(false); }} /> : undefined} />}
    <Modal visible={filtersOpen} transparent animationType="fade" onRequestClose={() => setFiltersOpen(false)}><View style={m.backdrop}><View style={m.sheet} accessibilityViewIsModal><View style={ui.between}><Text style={s.title}>Фильтры</Text><IconButton icon={X} label="Закрыть фильтры" onPress={() => setFiltersOpen(false)} /></View><View style={ui.between}><Text style={ui.body}>Только избранное</Text><Switch accessibilityLabel="Только избранное" value={favoritesOnly} onValueChange={setFavoritesOnly} trackColor={{ false: colors.line, true: colors.navy }} thumbColor="#FFF" /></View><Button onPress={() => setFiltersOpen(false)}>Показать результаты</Button><Button secondary onPress={() => { setFavoritesOnly(false); setCategory('Все'); setDescending(false); setFiltersOpen(false); }}>Сбросить фильтры</Button></View></View></Modal>
  </View>;
}
export function InstrumentScreen() {
  const connection = useConnection();
  const readOnly = exchangeData && (!connection.data?.connected || connection.data.read_only !== false || connection.isError);
  const params = z.object({ id: uuidSchema }).safeParse(useLocalSearchParams());
  const market = useMarket(), accounts = useAccounts(), portfolio = usePortfolio(accounts.account), prefs = usePreferences(), { can } = useSession();
  const item = market.instruments.data?.items.find(asset => params.success && asset.id === params.data.id);
  const quote = market.quotes.data?.items.find(asset => asset.id === item?.id);
  const position = portfolio.data?.positions.find(position => position.instrument_id === item?.id);
  if (!params.success) return <Page title="Ценная бумага" back><Empty title="Некорректная ссылка" detail="Выберите бумагу из каталога." action="Открыть рынок" onPress={() => router.replace('/market')} /></Page>;
  if (!item) return <Page title="Ценная бумага" back><QueryFeedback query={market.instruments} />{market.instruments.isSuccess && <Empty title="Инструмент не найден" detail="Обновите каталог или выберите другую бумагу." action="Обновить каталог" onPress={() => { void market.instruments.refetch(); }} />}</Page>;
  const trade = (side: 'BUY' | 'SELL') => router.push({ pathname: '/trade', params: { id: item.id, side } });
  const canTrade = !readOnly && can('orders.submit') && item.status === 'ACTIVE' && accounts.account?.status === 'ACTIVE' && accounts.account.currency === item.currency;
  return <Page title="Ценная бумага" back right={<IconButton icon={Star} label={prefs.favorites.includes(item.id) ? 'Убрать из избранного' : 'В избранное'} active={prefs.favorites.includes(item.id)} onPress={() => prefs.toggleFavorite(item.id)} />}>
    <View style={m.identity}><SecurityAvatar name={item.name} ticker={item.ticker} /><View style={s.flex}><Text style={s.title}>{item.name}</Text><Text style={ui.caption}>{item.ticker} · {item.isin}</Text></View></View>
    <Card><Text style={ui.subtitle}>Последняя цена</Text><Text style={s.total}>{priceAmount(quote?.last, item.currency)}</Text><DetailRow label="Лучшая покупка" value={priceAmount(quote?.bid, item.currency)} /><DetailRow label="Лучшая продажа" value={priceAmount(quote?.ask, item.currency)} /></Card>
    {market.quotes.isError && <QueryFeedback query={market.quotes} />}
    <View style={s.actions}><Button style={s.flex} disabled={!canTrade} onPress={() => trade('BUY')}>Купить</Button><Button style={s.flex} secondary disabled={!canTrade || !position || Number(position.quantity) <= 0} onPress={() => trade('SELL')}>Продать</Button></View>
    {exchangeData && connection.isPending && <Text style={ui.caption}>Проверяем доступность торговли…</Text>}
    {exchangeData && !connection.isPending && readOnly && <><Notice title="Торговля временно недоступна">{connection.isError || !connection.data?.connected ? 'Не удалось установить связь с биржей. Обновите подключение и попробуйте ещё раз.' : 'Брокер временно приостановил приём заявок.'}</Notice><Button secondary onPress={() => { void connection.refetch(); }}>Обновить</Button></>}
    {!can('orders.submit') && <Notice>Для подачи заявок обратитесь к брокеру: торговый доступ ещё не предоставлен.</Notice>}
    {item.status !== 'ACTIVE' && <Notice>Приём заявок по этой бумаге приостановлен.</Notice>}
    {accounts.account && accounts.account.status !== 'ACTIVE' && <Notice>Торговля по этому счёту недоступна. Обратитесь к брокеру.</Notice>}
    {canTrade && portfolio.isSuccess && (!position || Number(position.quantity) <= 0) && <Text style={ui.caption}>Для продажи нужны бумаги в портфеле.</Text>}
    {!accounts.account && <Notice>Для торговли нужен активный брокерский счёт.</Notice>}
    {accounts.account && accounts.account.currency !== item.currency && <Notice>Выберите счёт в валюте {item.currency} в разделе «Портфель», чтобы торговать этой бумагой.</Notice>}
    {position && <Card><Text style={ui.label}>В вашем портфеле</Text><DetailRow label={`${prefs.hiddenBalance ? '•••' : position.quantity} шт.`} value={amount(position.market_value, item.currency, prefs.hiddenBalance)} /></Card>}
    <Card><DetailRow label="Валюта" value={item.currency} /><DetailRow label="Размер лота" value={item.lot_size} /><DetailRow label="Шаг цены" value={item.tick_size} /><DetailRow label="Режим торгов" value={item.board || '—'} /><DetailRow label="Статус" value={item.status === 'ACTIVE' ? 'Доступен для торгов' : item.status} /></Card>
  </Page>;
}
const m = StyleSheet.create({ page: { flex: 1, backgroundColor: colors.background }, top: { paddingHorizontal: 20, paddingTop: 26, paddingBottom: 10, gap: 16 }, searchRow: { flexDirection: 'row', gap: 10 }, search: { flex: 1, flexDirection: 'row', alignItems: 'center', paddingHorizontal: 14, backgroundColor: '#FFF', borderRadius: 14, gap: 10 }, input: { flex: 1, minHeight: 52, color: colors.ink, fontSize: 14 }, filterButton: { width: 48, minHeight: 52, backgroundColor: '#FFF', borderRadius: 14, alignItems: 'center', justifyContent: 'center' }, sortRow: { flexDirection: 'row', alignItems: 'center', gap: 18, paddingVertical: 4 }, sortText: { color: colors.muted, fontSize: 11 }, sortValue: { color: colors.ink, fontSize: 11, fontWeight: '500' }, sectionHeading: { flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', marginBottom: 12 }, sectionTitle: { color: colors.ink, fontSize: 15, fontWeight: '600' }, firstRow: { borderTopLeftRadius: 20, borderTopRightRadius: 20 }, lastRow: { borderBottomLeftRadius: 20, borderBottomRightRadius: 20 }, sectionSpace: { height: 23 }, backdrop: { flex: 1, backgroundColor: 'rgba(16,35,55,.35)', justifyContent: 'flex-end', alignItems: 'center' }, sheet: { backgroundColor: '#FFF', borderTopLeftRadius: 24, borderTopRightRadius: 24, padding: 24, paddingBottom: 40, width: '100%', maxWidth: 440, gap: 20 }, padding: { paddingHorizontal: 20 }, list: { paddingHorizontal: 20, paddingBottom: 30 }, listHeading: { gap: 10, marginBottom: 12 }, listItem: { backgroundColor: '#FFF', paddingHorizontal: 14 }, identity: { flexDirection: 'row', alignItems: 'center', gap: 12 } });

