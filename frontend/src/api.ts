const API = import.meta.env.VITE_API_URL || "";
import { socketRequest, closeSocket, subscribeSocket } from "./websocket";
import { recordEndpoint } from "./dataSources";
export const apiBaseURL = API;
const TOKEN_KEY = "access_token";
subscribeSocket(message => { if (message.type === "unauthorized") logoutToLogin(); });

const PUBLIC_AUTH = ["/v1/auth/login", "/v1/auth/refresh", "/v1/auth/password-reset"];

export type ApiError = {
  error: {
    code: string;
    message: string;
    request_id: string;
    fields?: Record<string, string>;
  };
};

export class ApiRequestError extends Error {
  code: string;
  fields?: Record<string, string>;
  constructor(code: string, message: string, fields?: Record<string, string>) {
    super(message);
    this.name = "ApiRequestError";
    this.code = code;
    this.fields = fields;
  }
}

const listeners = new Set<() => void>();
let storageBound = false;

function notifyAuth() {
  listeners.forEach((fn) => fn());
}

function bindStorage() {
  if (storageBound || typeof window === "undefined") return;
  storageBound = true;
  window.addEventListener("storage", (e) => {
    if (e.key === TOKEN_KEY || e.key === null) notifyAuth();
  });
}

export function subscribeAuth(listener: () => void) {
  bindStorage();
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

export function getAccessToken(): string | null {
  return localStorage.getItem(TOKEN_KEY);
}

export function setAccessToken(token: string) {
  localStorage.setItem(TOKEN_KEY, token);
  notifyAuth();
}

export function jwtExpiryMs(token: string): number | null {
  try {
    const part = token.split(".")[1];
    if (!part) return null;
    const b64 = part.replace(/-/g, "+").replace(/_/g, "/");
    const padded = b64.padEnd(b64.length + ((4 - (b64.length % 4)) % 4), "=");
    const payload = JSON.parse(atob(padded)) as { exp?: number };
    return typeof payload.exp === "number" ? payload.exp * 1000 : null;
  } catch {
    return null;
  }
}

export function sessionTtlMs(): number | null {
  const token = getAccessToken();
  if (!token) return null;
  const exp = jwtExpiryMs(token);
  if (exp == null) return null;
  return exp - Date.now();
}

export function getSessionSnapshot(): string {
  const token = getAccessToken();
  if (!token) return "";
  const exp = jwtExpiryMs(token);
  if (exp != null && Date.now() >= exp) return "";
  return token;
}

export function logoutToLogin() {
  closeSocket();
  const had = Boolean(localStorage.getItem(TOKEN_KEY));
  localStorage.removeItem(TOKEN_KEY);
  if (had) notifyAuth();
  if (window.location.pathname !== "/login") {
    window.location.replace("/login");
  }
}

function isPublicAuth(path: string) {
  return PUBLIC_AUTH.some((p) => path === p || path.startsWith(`${p}?`) || path.startsWith(`${p}/`));
}

export async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
  const sourcePage = window.location.pathname;
  const sourceMethod = (init.method || "GET").toUpperCase();
  const transport = isPublicAuth(path) ? "REST" : "WebSocket";
  const headers = new Headers(init.headers);
  headers.set("Content-Type", "application/json");
  if (!isPublicAuth(path)) {
    if (getAccessToken() && !getSessionSnapshot()) {
      logoutToLogin();
      throw new Error("UNAUTHORIZED");
    }
    const token = getAccessToken();
    if (token) headers.set("Authorization", `Bearer ${token}`);
  }
  recordEndpoint(sourcePage, sourceMethod, path, "Загрузка…", transport);
  let res: Response;
  try {
    if (isPublicAuth(path)) res = await fetch(`${API}${path}`, { ...init, headers });
    else {
      const result = await socketRequest(API, getAccessToken() || "", path, { ...init, headers });
      res = new Response(result.status === 204 ? null : JSON.stringify(result.data), { status: result.status || 500, headers: { "Content-Type": "application/json" } });
    }
  }
  catch (error) { recordEndpoint(sourcePage, sourceMethod, path, "Ошибка соединения", transport); throw error; }
  recordEndpoint(sourcePage, sourceMethod, path, res.ok ? "Данные получены" : `Ошибка ${res.status}`, transport);
  if (res.status === 204) return undefined as T;
  const data = await res.json();
  if (!res.ok) {
    if (res.status === 401 && !isPublicAuth(path)) {
      logoutToLogin();
    }
    const err = data as ApiError;
    throw new ApiRequestError(
      err.error?.code || "REQUEST_FAILED",
      err.error?.message || err.error?.code || "REQUEST_FAILED",
      err.error?.fields,
    );
  }
  return data as T;
}
