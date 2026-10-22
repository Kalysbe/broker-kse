import { useEffect, useRef, useState } from 'react';
import { BackHandler, Keyboard, Pressable, StyleSheet, Text, TextInput, View } from 'react-native';
import { router, useLocalSearchParams } from 'expo-router';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import * as Crypto from 'expo-crypto';
import Decimal from 'decimal.js';
import { z } from 'zod';
import { Check, CheckCircle2 } from 'lucide-react-native';
import { Button, Card, DetailRow, Empty, Notice, Page, Segments } from '../components';
import { ApiError, errorMessage } from '../api/client';
import { orderSchema, uuidSchema, type Order } from '../api/schemas';
import { useConnection, exchangeData, useAccounts, useCash, useMarket, useOrders, usePortfolio } from '../broker';
import { useSession } from '../session';
import { orderFailureMessage, orderStatusLabel, terminalStatuses, validateOrderNumbers, type OrderNumbers } from '../domain/order';
import { colors, ui } from '../theme';
import { AccountFeedback, AccountPicker, NoAccess, QueryFeedback, amount, priceAmount, liveStyles as s } from './common';

export default function TradeScreen() {
  const connection = useConnection();
  const readOnly = exchangeData && (!connection.data?.connected || connection.data.read_only !== false || connection.isError);
  const params = z.object({ id: uuidSchema, side: z.enum(['BUY', 'SELL']).optional() }).safeParse(useLocalSearchParams());
  const { api, can } = useSession(), cache = useQueryClient();
  const accounts = useAccounts(), account = accounts.account, market = useMarket(), cash = useCash(account), portfolio = usePortfolio(account), orders = useOrders(account);
  const instrument = market.instruments.data?.items.find(item => params.success && item.id === params.data.id);
  const quote = market.quotes.data?.items.find(item => item.id === instrument?.id);
  const [side, setSide] = useState<'BUY' | 'SELL'>(params.success && params.data.side === 'SELL' ? 'SELL' : 'BUY');
  const [quantity, setQuantity] = useState(''), [price, setPrice] = useState('');
  const [step, setStep] = useState(0), [agreed, setAgreed] = useState(false), [error, setError] = useState('');
  const [result, setResult] = useState<Order | null>(null), [uncertain, setUncertain] = useState(false);
  const draft = useRef<{ signature: string; id: string } | null>(null), guard = useRef(false);
  useEffect(() => { if (instrument && !quantity) setQuantity(instrument.lot_size); }, [instrument?.id]);
  useEffect(() => { const initial = (side === 'BUY' ? quote?.ask : quote?.bid) || quote?.last; if (initial && !price && new Decimal(initial).isPositive()) setPrice(initial); }, [quote?.last, quote?.ask, quote?.bid]);
  const submit = useMutation({ mutationFn: (body: unknown) => api.request('/v1/orders', orderSchema, { method: 'POST', body }), retry: false, onSuccess: async order => { setResult(order); await Promise.all(['orders', 'cash', 'portfolio', 'ledger'].map(key => cache.invalidateQueries({ queryKey: [key] }))); } });
  useEffect(() => { if (!submit.isPending) return; const handler = BackHandler.addEventListener('hardwareBackPress', () => true); return () => handler.remove(); }, [submit.isPending]);
  if (!params.success) return <Page title="Новая заявка" back><Empty title="Некорректная ссылка" detail="Выберите бумагу на рынке." action="Открыть рынок" onPress={() => router.replace('/market')} /></Page>;
  if (readOnly) return <Page title="Новая заявка" back>{connection.isPending ? <QueryFeedback query={connection} /> : <><Notice>Торговые операции временно недоступны. Обновите подключение и попробуйте ещё раз.</Notice><Button secondary onPress={() => { void connection.refetch(); }}>Обновить</Button></>}</Page>;
  if (!can('orders.submit') || !can('orders.read') || !can('cash.read') || !can('portfolio.read')) return <Page title="Новая заявка" back><NoAccess /></Page>;
  if (!instrument) return <Page title="Новая заявка" back><QueryFeedback query={market.instruments} />{market.instruments.isSuccess && <Empty title="Бумага недоступна" detail="Выберите доступный инструмент на рынке." action="На рынок" onPress={() => router.replace('/market')} />}</Page>;
  let numbers: OrderNumbers | null = null;
  try { numbers = validateOrderNumbers(quantity, price, instrument.lot_size, instrument.tick_size); } catch {}
  const position = portfolio.data?.positions.find(item => item.instrument_id === instrument.id);
  const reserved = orders.orders.filter(order => order.instrument_id === instrument.id && order.side === 'SELL' && !terminalStatuses.has(order.status)).reduce((sum, order) => sum.plus(new Decimal(order.quantity).minus(order.filled_quantity)), new Decimal(0));
  const freeQuantity = Decimal.max(0, new Decimal(position?.quantity ?? '0').minus(reserved)).toFixed();
  const validate = () => {
    const values = validateOrderNumbers(quantity, price, instrument.lot_size, instrument.tick_size);
    if (!account || account.status !== 'ACTIVE') throw new Error('Выберите активный торговый счёт.');
    if (account.currency !== instrument.currency) throw new Error(`Для этой бумаги нужен счёт в валюте ${instrument.currency}.`);
    if (instrument.status !== 'ACTIVE') throw new Error('Инструмент недоступен для торгов.');
    if (!cash.data || !portfolio.data || !orders.isSuccess || cash.isError || portfolio.isError || orders.isError) throw new Error('Дождитесь обновления данных счёта и заявок.');
    if (side === 'BUY' && new Decimal(values.gross).greaterThan(cash.data.buckets.AVAILABLE ?? '0')) throw new Error('Недостаточно доступных средств даже без учёта комиссии.');
    if (side === 'SELL' && new Decimal(values.quantity).greaterThan(freeQuantity)) throw new Error('Недостаточно свободных бумаг для продажи.');
    return values;
  };
  const next = () => { try { validate(); setError(''); Keyboard.dismiss(); setStep(prev => prev + 1); } catch (problem) { setError(errorMessage(problem)); } };
  const send = async () => {
    if (guard.current || !agreed) return;
    guard.current = true; setError('');
    try {
      const values = validate();
      const payload = { account_id: account!.id, instrument_id: instrument.id, side, order_type: 'LIMIT', time_in_force: 'DAY', quantity: values.quantity, price: values.price };
      const signature = JSON.stringify(payload);
      if (draft.current?.signature !== signature) draft.current = { signature, id: Crypto.randomUUID() };
      await submit.mutateAsync({ ...payload, client_order_id: draft.current.id });
      setUncertain(false);
    } catch (problem) { setError(errorMessage(problem)); if (problem instanceof ApiError && ['NETWORK', 'TIMEOUT', 'INVALID_RESPONSE'].includes(problem.code)) setUncertain(true); }
    finally { guard.current = false; }
  };
  if (result) return <Page title={orderStatusLabel(result.status)}><Card style={t.result}><CheckCircle2 size={43} strokeWidth={1.5} color={result.status === 'REJECTED' || result.status === 'FAILED' ? colors.red : colors.navy} /><Text style={s.title}>{orderStatusLabel(result.status)}</Text><Text style={ui.body}>{result.reject_reason ? orderFailureMessage(result.reject_reason) : 'Запрос обработан. Следите за текущим статусом в разделе «Заявки».'}</Text><DetailRow label="Номер заявки" value={result.id} /></Card><Button onPress={() => router.replace('/orders')}>Открыть заявки</Button><Button secondary onPress={() => router.replace('/')}>На главную</Button></Page>;
  return <Page title={['Новая заявка', 'Проверка заявки', 'Подтверждение'][step]} back onBack={() => { if (submit.isPending) return; if (uncertain) { router.replace('/orders'); return; } if (step > 0) { setStep(prev => prev - 1); setAgreed(false); setError(''); } else if (router.canGoBack()) router.back(); else router.replace('/market'); }}>
    <View style={t.progress}>{[0, 1, 2].map(value => <View key={value} style={[t.line, value <= step && t.activeLine]} />)}</View><Text style={ui.caption}>Шаг {step + 1} из 3 · Лимитная заявка</Text>
    <AccountFeedback query={accounts} />
    {step === 0 && <><AccountPicker {...accounts} onSelect={accounts.selectAccount} /><Segments options={['Покупка', 'Продажа']} value={side === 'BUY' ? 'Покупка' : 'Продажа'} onChange={value => { setSide(value === 'Покупка' ? 'BUY' : 'SELL'); setError(''); }} /></>}
    <Card><Text style={ui.label}>{instrument.name}</Text><Text style={ui.caption}>{instrument.ticker} · {instrument.currency}</Text>{step > 0 && <><DetailRow label="Счёт" value={account?.account_number || '—'} /><DetailRow label="Операция" value={side === 'BUY' ? 'Покупка' : 'Продажа'} /><DetailRow label="Количество" value={`${numbers?.quantity ?? '—'} шт.`} /><DetailRow label="Лимитная цена" value={priceAmount(numbers?.price, instrument.currency)} /></>}</Card>
    {step === 0 && <><QueryFeedback query={cash} /><QueryFeedback query={portfolio} /><QueryFeedback query={orders} /><Text style={ui.caption}>{side === 'BUY' ? `Доступно: ${amount(cash.data?.buckets.AVAILABLE, account?.currency)}` : `Свободно для продажи: ${freeQuantity} шт.`}</Text><View style={t.field}><Text style={ui.label}>Количество, шт.</Text><TextInput accessibilityLabel="Количество бумаг" value={quantity} onChangeText={setQuantity} inputMode="decimal" keyboardType="decimal-pad" style={ui.input} maxLength={24} /><Text style={ui.caption}>Размер лота: {instrument.lot_size}</Text></View><View style={t.field}><Text style={ui.label}>Лимитная цена, {instrument.currency}</Text><TextInput accessibilityLabel="Лимитная цена" value={price} onChangeText={setPrice} inputMode="decimal" keyboardType="decimal-pad" style={ui.input} maxLength={24} /><Text style={ui.caption}>Шаг цены: {instrument.tick_size}</Text></View></>}
    {step === 0 && ![quote?.ask, quote?.bid, quote?.last].some(value => Number(value) > 0) && <Text style={ui.caption}>Текущая котировка отсутствует. Укажите лимитную цену вручную.</Text>}
    <Card><DetailRow label="Сумма без комиссии" value={amount(numbers?.gross, instrument.currency)} /><DetailRow label="Комиссия" value="По тарифу брокера" /></Card>
    <Notice warning title="Исполнение не гарантировано">Покупка возможна по цене не выше указанной, продажа — не ниже. Заявка может исполниться частично. Доступный остаток, комиссия и ограничения проверяются перед отправкой заявки.</Notice>
    {step === 2 && <Pressable accessibilityRole="checkbox" accessibilityState={{ checked: agreed }} disabled={submit.isPending} onPress={() => setAgreed(value => !value)} style={t.consent}><View style={[t.check, agreed && t.checked]}>{agreed && <Check size={15} color="#FFF" />}</View><Text style={t.consentText}>Проверил параметры заявки и понимаю инвестиционные риски.</Text></Pressable>}
    {!!error && <Text accessibilityRole="alert" style={s.errorText}>{error}</Text>}
    {uncertain && <Notice title="Результат запроса уточняется">Проверьте список заявок или повторите запрос с теми же параметрами. Повторная отправка использует тот же идентификатор заявки.</Notice>}
    <Button disabled={submit.isPending || !account || step === 2 && !agreed} onPress={step < 2 ? next : () => { void send(); }}>{submit.isPending ? 'Отправляем заявку…' : step === 0 ? 'Проверить заявку' : step === 1 ? 'Далее' : uncertain ? 'Уточнить результат' : 'Отправить заявку'}</Button>
    {uncertain && <Button secondary onPress={() => router.replace('/orders')}>Проверить мои заявки</Button>}
  </Page>;
}
const t = StyleSheet.create({ progress: { flexDirection: 'row', gap: 5 }, line: { flex: 1, height: 3, borderRadius: 2, backgroundColor: '#D7DEE7' }, activeLine: { backgroundColor: colors.navy }, field: { gap: 10 }, consent: { flexDirection: 'row', gap: 10, alignItems: 'flex-start', minHeight: 44, paddingVertical: 10 }, check: { width: 22, height: 22, borderWidth: 1, borderColor: '#A9B7C8', borderRadius: 6, alignItems: 'center', justifyContent: 'center' }, checked: { backgroundColor: colors.navy, borderColor: colors.navy }, consentText: { flex: 1, color: colors.ink, fontSize: 13, lineHeight: 20 }, result: { gap: 20, alignItems: 'center', paddingVertical: 35 } });
