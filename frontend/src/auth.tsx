import { createContext, useContext, useEffect, useMemo, useSyncExternalStore, type ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import { api, getAccessToken, getSessionSnapshot, logoutToLogin, sessionTtlMs, subscribeAuth } from "./api";

export type BrokerInfo = {
  id?: string;
  code?: string;
  name?: string;
  status?: string;
  license_no?: string | null;
  firm_name?: string | null;
  quik_firm?: string | null;
};

export type User = {
  id: string;
  login: string;
  email?: string;
  phone?: string | null;
  first_name: string;
  last_name: string;
  status?: string;
  totp_enabled?: boolean;
  roles: string[];
  permissions: string[];
  client_id?: string | null;
  last_login_at?: string | null;
  last_login_ip?: string | null;
  broker?: BrokerInfo | null;
};

const Ctx = createContext<{
  user?: User;
  authenticated: boolean;
  can: (p: string) => boolean;
  logout: () => void;
}>({ authenticated: false, can: () => false, logout: logoutToLogin });

export function AuthProvider({ children }: { children: ReactNode }) {
  const token = useSyncExternalStore(subscribeAuth, getSessionSnapshot, () => "");

  useEffect(() => {
    if (getAccessToken() && !token) logoutToLogin();
  }, [token]);

  useEffect(() => {
    if (!token) return;
    const ttl = sessionTtlMs();
    if (ttl == null) return;
    const id = window.setTimeout(logoutToLogin, Math.max(0, ttl));
    return () => window.clearTimeout(id);
  }, [token]);

  useEffect(() => {
    const check = () => {
      if (getAccessToken() && !getSessionSnapshot()) logoutToLogin();
    };
    document.addEventListener("visibilitychange", check);
    window.addEventListener("focus", check);
    return () => {
      document.removeEventListener("visibilitychange", check);
      window.removeEventListener("focus", check);
    };
  }, []);

  const q = useQuery({
    queryKey: ["me", token],
    queryFn: () => api<User>("/v1/auth/me"),
    enabled: !!token,
    retry: false,
  });
  const value = useMemo(
    () => ({
      user: q.data,
      authenticated: !!token,
      can: (p: string) => !!q.data?.permissions?.includes(p),
      logout: logoutToLogin,
    }),
    [q.data, token],
  );
  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}

export function useAuth() {
  return useContext(Ctx);
}
