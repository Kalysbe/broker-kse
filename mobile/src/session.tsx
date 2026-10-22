import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';
import { Platform } from 'react-native';
import * as SecureStore from 'expo-secure-store';
import { useQueryClient } from '@tanstack/react-query';
import { ApiClient, ApiError, type SessionStorage } from './api/client';
import { userSchema, type User } from './api/schemas';

const tokenStorage: SessionStorage = Platform.OS === 'web'
  ? { get: async () => null, set: async () => {}, clear: async () => {} }
  : { get: () => SecureStore.getItemAsync('sbis.session.v1'), set: value => SecureStore.setItemAsync('sbis.session.v1', value, { keychainAccessible: SecureStore.WHEN_UNLOCKED_THIS_DEVICE_ONLY }), clear: () => SecureStore.deleteItemAsync('sbis.session.v1') };
type Session = { api: ApiClient; user: User | null; initializing: boolean; startupError: unknown; retry: () => void; login: (login: string, password: string, totp?: string) => Promise<void>; logout: () => Promise<void>; can: (permission: string) => boolean };
const Context = createContext<Session | null>(null);
export function SessionProvider({ children }: { children: ReactNode }) {
  const cache = useQueryClient();
  const [user, setUser] = useState<User | null>(null);
  const [initializing, setInitializing] = useState(true);
  const [startupError, setStartupError] = useState<unknown>(null);
  const [attempt, setAttempt] = useState(0);
  const api = useMemo(() => new ApiClient(process.env.EXPO_PUBLIC_API_URL || (Platform.OS === 'android' ? 'http://10.0.2.2:18080' : 'http://127.0.0.1:18080'), tokenStorage, () => { setUser(null); void cache.cancelQueries(); cache.clear(); }), [cache]);
  useEffect(() => {
    let active = true;
    setInitializing(true); setStartupError(null);
    (async () => { try { if (await api.restore()) { const me = await api.request('/v1/auth/me', userSchema); if (active) setUser(me); } } catch (error) { if (active && !(error instanceof ApiError && [401, 403].includes(error.status))) setStartupError(error); } finally { if (active) setInitializing(false); } })();
    return () => { active = false; };
  }, [api, attempt]);
  const value: Session = { api, user, initializing, startupError, retry: () => setAttempt(prev => prev + 1),
    login: async (login, password, totp) => { await api.signIn(login, password, totp); const me = await api.request('/v1/auth/me', userSchema); await cache.cancelQueries(); cache.clear(); setUser(me); setStartupError(null); },
    logout: async () => { setUser(null); setStartupError(null); await api.signOut(); }, can: permission => user?.permissions.includes(permission) ?? false,
  };
  return <Context.Provider value={value}>{children}</Context.Provider>;
}
export function useSession() { const value = useContext(Context); if (!value) throw new Error('SessionProvider missing'); return value; }

