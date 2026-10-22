import { createContext, useContext, useEffect, useState, type ReactNode } from 'react';
import AsyncStorage from '@react-native-async-storage/async-storage';
import { useQuery } from '@tanstack/react-query';
import { useSession } from './session';
import { connectionSchema, accountSchema, cashSchema, instrumentSchema, ledgerSchema, listSchema, notificationSchema, orderSchema, portfolioSchema, quoteSchema, type Account } from './api/schemas';

export const exchangeData = process.env.EXPO_PUBLIC_DATA_SOURCE === 'kfb';
export function useConnection() {
  const { api, user } = useSession();
  return useQuery({ queryKey: ['connection', user?.id], queryFn: ({ signal }) => api.request('/v1/integrations/kfb/health', connectionSchema, { signal }), enabled: !!user && exchangeData, refetchInterval: 15000 });
}
type Preferences = { favorites: string[]; hiddenBalance: boolean; toggleFavorite: (id: string) => void; toggleBalance: () => void; accountId: string | null; selectAccount: (id: string) => void };
const Context = createContext<Preferences | null>(null);
export function BrokerProvider({ children }: { children: ReactNode }) {
  const { user } = useSession();
  const [favorites, setFavorites] = useState<string[]>([]);
  const [hiddenBalance, setHiddenBalance] = useState(false);
  const [accountId, setAccountId] = useState<string | null>(null);
  const [ready, setReady] = useState(false);
  const [hydratedFor, setHydratedFor] = useState<string | null>(null);
  useEffect(() => {
    let active = true; setReady(false); setHydratedFor(null); setFavorites([]); setHiddenBalance(false); setAccountId(null);
    if (user) void AsyncStorage.getItem(`sbis.preferences.${user.id}`).then(raw => {
      if (!active || !raw) return;
      try { const data = JSON.parse(raw); if (Array.isArray(data.favorites)) setFavorites(data.favorites.filter((id: unknown) => typeof id === 'string')); setHiddenBalance(data.hiddenBalance === true); } catch {}
    }).catch(() => {}).finally(() => { if (active) { setHydratedFor(user.id); setReady(true); } });
    return () => { active = false; };
  }, [user?.id]);
  useEffect(() => { if (user && ready && hydratedFor === user.id) void AsyncStorage.setItem(`sbis.preferences.${user.id}`, JSON.stringify({ favorites, hiddenBalance })).catch(() => {}); }, [user?.id, ready, hydratedFor, favorites, hiddenBalance]);
  return <Context.Provider value={{ favorites: hydratedFor === user?.id ? favorites : [], hiddenBalance: !ready || hydratedFor !== user?.id || hiddenBalance, accountId, selectAccount: setAccountId, toggleFavorite: id => setFavorites(prev => prev.includes(id) ? prev.filter(item => item !== id) : [...prev, id]), toggleBalance: () => setHiddenBalance(prev => !prev) }}>{children}</Context.Provider>;
}
export function usePreferences() { const value = useContext(Context); if (!value) throw new Error('BrokerProvider missing'); return value; }
export function useAccounts() {
  const { api, user, can } = useSession();
  const prefs = usePreferences();
  const query = useQuery({ queryKey: ['accounts', user?.id], queryFn: ({ signal }) => api.request('/v1/accounts', listSchema(accountSchema), { signal }), enabled: !!user && can('accounts.read') });
  const accounts = query.data?.items ?? [];
  const account = accounts.find(item => item.id === prefs.accountId) ?? accounts.find(item => item.status === 'ACTIVE') ?? accounts[0];
  return { ...query, accounts, account, selectAccount: prefs.selectAccount };
}
export function usePortfolio(account?: Account) {
  const { api, user, can } = useSession();
  return useQuery({ queryKey: ['portfolio', user?.id, account?.id], queryFn: ({ signal }) => api.request(`/v1/accounts/${account!.id}/${exchangeData ? 'exchange-portfolio' : 'portfolio'}`, portfolioSchema, { signal }), enabled: !!account && can('portfolio.read'), refetchInterval: 15000 });
}
export function useCash(account?: Account) {
  const { api, user, can } = useSession();
  const portfolio = usePortfolio(exchangeData ? account : undefined);
  const local = useQuery({ queryKey: ['cash', user?.id, account?.id], queryFn: ({ signal }) => api.request(`/v1/accounts/${account!.id}/cash`, cashSchema, { signal }), enabled: !exchangeData && !!account && can('cash.read'), refetchInterval: 15000 });
  return exchangeData ? { ...portfolio, data: portfolio.data ? { account_id: portfolio.data.account_id, buckets: portfolio.data.cash } : undefined } : local;
}
export function useMarket() {
  const { api, user, can } = useSession();
  const quotes = useQuery({ queryKey: ['market', user?.id], queryFn: ({ signal }) => api.request('/v1/market', listSchema(quoteSchema), { signal }), enabled: !!user && can('marketdata.read'), refetchInterval: 15000 });
  const instruments = useQuery({ queryKey: ['instruments', user?.id], queryFn: ({ signal }) => api.request('/v1/instruments', listSchema(instrumentSchema), { signal }), enabled: !!user && can('instruments.read') && (!exchangeData || quotes.isSuccess), staleTime: 60000 });
  const liveIds = new Set(quotes.data?.items.filter(item => item.source === 'KFB').map(item => item.id) ?? []);
  return { quotes, instruments: exchangeData && instruments.data ? { ...instruments, data: { ...instruments.data, items: instruments.data.items.filter(item => liveIds.has(item.id)) } } : instruments };
}
export function useOrders(account?: Account) {
  const { api, user, can } = useSession();
  const query = useQuery({ queryKey: ['orders', user?.id], queryFn: ({ signal }) => api.request('/v1/orders', listSchema(orderSchema), { signal }), enabled: !!user && can('orders.read'), refetchInterval: 5000 });
  return { ...query, orders: query.data?.items.filter(order => account && order.account_id === account.id) ?? [] };
}
export function useLedger(account?: Account) {
  const { api, user, can } = useSession();
  return useQuery({ queryKey: ['ledger', user?.id, account?.id], queryFn: ({ signal }) => api.request(`/v1/accounts/${account!.id}/ledger`, listSchema(ledgerSchema), { signal }), enabled: !!account && can('cash.read') });
}
export function useNotifications() {
  const { api, user, can } = useSession();
  return useQuery({ queryKey: ['notifications', user?.id], queryFn: ({ signal }) => api.request('/v1/notifications', listSchema(notificationSchema), { signal }), enabled: !!user && can('notifications.manage') });
}
