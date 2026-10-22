import { Text, View, StyleSheet } from 'react-native';
import { router } from 'expo-router';
import { Bell, Eye, EyeOff, History, Plus, Minus } from 'lucide-react-native';
import Decimal from 'decimal.js';
import { Button, Card, DetailRow, Empty, IconButton, MenuRow, Notice, Page, Section } from '../components';
import { useConnection, exchangeData, useAccounts, useCash, useLedger, useMarket, usePortfolio, usePreferences } from '../broker';
import { useSession } from '../session';
import { colors, ui } from '../theme';
import { AccountFeedback, AccountPicker, MarketRow, NoAccess, QueryFeedback, RefreshButton, amount, dateText, ledgerLabel, liveStyles as s } from './common';

export function HomeScreen() {
  const connection = useConnection();
  const readOnly = exchangeData && (!connection.data?.connected || connection.data.read_only !== false || connection.isError);
  const { user, can } = useSession();
  const accounts = useAccounts(), { account } = accounts;
  const cash = useCash(account), portfolio = usePortfolio(account), market = useMarket(), ledger = useLedger(account);
  const prefs = usePreferences();
  const total = cash.data?.buckets.AVAILABLE !== undefined && portfolio.data?.securities_value != null ? new Decimal(cash.data.buckets.AVAILABLE ?? '0').plus(cash.data.buckets.BLOCKED ?? '0').plus(cash.data.buckets.PENDING ?? '0').plus(portfolio.data.securities_value).toString() : undefined;
  const favorites = (market.instruments.data?.items ?? []).filter(item => prefs.favorites.includes(item.id));
  return <Page><View style={ui.between}><View><Text style={h.greeting}>Здравствуйте{user?.first_name ? `, ${user.first_name}` : ''}</Text><Text style={h.subtitle}>Государственный брокер</Text></View>{can('notifications.manage') && <IconButton icon={Bell} label="Уведомления" onPress={() => router.push('/notifications')} />}</View>
    {!can('accounts.read') ? <NoAccess /> : <><AccountFeedback query={accounts} />{account && <>
      {accounts.accounts.length > 1 && <AccountPicker {...accounts} onSelect={accounts.selectAccount} />}
      <Card><View style={ui.between}><Text style={ui.subtitle}>Стоимость портфеля</Text><IconButton icon={prefs.hiddenBalance ? EyeOff : Eye} label="Скрыть или показать суммы" onPress={prefs.toggleBalance} /></View><Text style={s.total}>{amount(total, account.currency, prefs.hiddenBalance)}</Text><DetailRow label="Доступно для торговли" value={amount(cash.data?.buckets.AVAILABLE, account.currency, prefs.hiddenBalance)} /><DetailRow label="Ценные бумаги" value={amount(portfolio.data?.securities_value, account.currency, prefs.hiddenBalance)} /><Text style={ui.caption}>Стоимость бумаг указана по средней цене.</Text></Card>
      <QueryFeedback query={cash} /><QueryFeedback query={portfolio} />
      <View style={s.actions}><Button icon={Plus} disabled={readOnly} style={s.flex} onPress={() => router.push('/market')}>Купить</Button><Button icon={Minus} disabled={readOnly} secondary style={s.flex} onPress={() => router.push('/portfolio')}>Продать</Button></View>
      {readOnly && <Text style={ui.caption}>{connection.isPending ? 'Проверяем доступность торговли…' : 'Торговля временно недоступна. Попробуйте позже.'}</Text>}
    </>}
    <Section title="Избранное" action="Все инструменты" onPress={() => router.push('/market')} />
    {favorites.length ? <Card style={s.listCard}>{favorites.map((item, index) => <MarketRow key={item.id} instrument={item} quote={market.quotes.data?.items.find(quote => quote.id === item.id)} last={index === favorites.length - 1} />)}</Card> : <Empty title="Ваш список наблюдения" detail="Откройте ценную бумагу на рынке и нажмите на звезду, чтобы добавить её сюда." action="Выбрать инструменты" onPress={() => router.push('/market')} />}
    {account && can('cash.read') && <><Section title="Последние операции" action="История" onPress={() => router.push('/history')} /><QueryFeedback query={ledger} />{ledger.data && (ledger.data.items.length ? <Card>{ledger.data.items.filter(item => item.bucket === 'AVAILABLE').slice(0, 3).map(item => <View key={item.id}><DetailRow label={ledgerLabel(item.entry_type)} value={amount(new Decimal(item.credit).minus(item.debit).toString(), account.currency, prefs.hiddenBalance)} /><Text style={ui.caption}>{dateText(item.created_at)}</Text></View>)}</Card> : <Text style={ui.subtitle}>Операций пока нет.</Text>)}</>}
    </>}
  </Page>;
}
export function PortfolioScreen() {
  const { can } = useSession();
  const accounts = useAccounts(), { account } = accounts;
  const portfolio = usePortfolio(account), cash = useCash(account), market = useMarket();
  const prefs = usePreferences();
  return <Page title="Портфель" right={<RefreshButton busy={portfolio.isFetching || cash.isFetching} onPress={() => { void portfolio.refetch(); void cash.refetch(); }} />}>
    {!can('portfolio.read') || !can('accounts.read') ? <NoAccess /> : <><AccountFeedback query={accounts} />{account && <>
      <AccountPicker {...accounts} onSelect={accounts.selectAccount} /><QueryFeedback query={portfolio} /><QueryFeedback query={cash} />
      <Card><View style={ui.between}><Text style={ui.label}>Деньги на счёте</Text><IconButton icon={prefs.hiddenBalance ? EyeOff : Eye} label="Скрыть или показать суммы" onPress={prefs.toggleBalance} /></View><Text style={s.total}>{amount(cash.data?.buckets.AVAILABLE, account.currency, prefs.hiddenBalance)}</Text><DetailRow label="Зарезервировано" value={amount(cash.data?.buckets.BLOCKED, account.currency, prefs.hiddenBalance)} /><DetailRow label="Ожидает обработки" value={amount(cash.data?.buckets.PENDING, account.currency, prefs.hiddenBalance)} /></Card>
      <Section title="Мои ценные бумаги" action="На рынок" onPress={() => router.push('/market')} />
      {portfolio.data && (portfolio.data.positions.length ? <Card style={s.listCard}>{portfolio.data.positions.map((position, index, list) => { const metadata = market.instruments.data?.items.find(item => item.id === position.instrument_id); return <MarketRow key={position.instrument_id} instrument={metadata ?? { id: position.instrument_id, ticker: position.ticker, name: position.ticker, currency: account.currency }} quantity={position.quantity} marketValue={position.market_value} hidden={prefs.hiddenBalance} last={index === list.length - 1} />; })}</Card> : <Empty title="В портфеле пока нет бумаг" detail="После исполнения сделки бумаги появятся здесь." action="Открыть рынок" onPress={() => router.push('/market')} />)}
      <Card style={s.listCard}><MenuRow icon={History} title="История операций" onPress={() => router.push('/history')} last /></Card>
    </>}</>}
  </Page>;
}
export function HistoryScreen() {
  const { can } = useSession();
  const accounts = useAccounts(), { account } = accounts;
  const ledger = useLedger(account);
  const entries = ledger.data?.items.filter(item => item.bucket === 'AVAILABLE') ?? [];
  return <Page title="История операций" back>{!can('cash.read') ? <NoAccess /> : <><AccountFeedback query={accounts} />{account && <><AccountPicker {...accounts} onSelect={accounts.selectAccount} /><QueryFeedback query={ledger} />{ledger.data && (entries.length ? entries.map(item => <Card key={item.id}><DetailRow label={ledgerLabel(item.entry_type)} value={amount(new Decimal(item.credit).minus(item.debit).toString(), account.currency)} /><Text style={ui.caption}>{dateText(item.created_at)}</Text></Card>) : <Empty title="Операций пока нет" detail="Здесь будут движения доступных средств по выбранному счёту." />)}</>}</>}</Page>;
}
const h = StyleSheet.create({ greeting: { color: colors.ink, fontSize: 22, fontWeight: '700', letterSpacing: -0.5, lineHeight: 29 }, subtitle: { color: colors.muted, fontSize: 12, lineHeight: 19, marginTop: 3 } });
