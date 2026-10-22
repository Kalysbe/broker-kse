import type { z } from 'zod';
import { storedSessionSchema, tokenPairSchema, type StoredSession } from './schemas.ts';

export interface SessionStorage { get(): Promise<string | null>; set(value: string): Promise<void>; clear(): Promise<void> }
export class ApiError extends Error {
  code: string; status: number; requestId?: string;
  constructor(code: string, message: string, status = 0, requestId?: string) { super(message); this.name = 'ApiError'; this.code = code; this.status = status; this.requestId = requestId; }
}
export function normalizeServer(value: string) {
  const url = new URL(value);
  const local = ['localhost', '127.0.0.1', '[::1]', '10.0.2.2'].includes(url.hostname);
  const octets = url.hostname.split('.').map(Number);
  const privateIPv4 = octets.length === 4 && octets.every(n => Number.isInteger(n) && n >= 0 && n <= 255) && (octets[0] === 10 || octets[0] === 172 && octets[1] >= 16 && octets[1] <= 31 || octets[0] === 192 && octets[1] === 168);
  const configuredLAN = privateIPv4 && url.origin === process.env.EXPO_PUBLIC_API_URL?.replace(/\/$/, '');
  if ((url.protocol !== 'https:' && !(url.protocol === 'http:' && (local || configuredLAN))) || url.username || url.password || url.search || url.hash || url.pathname !== '/') throw new Error('API URL must use HTTPS; HTTP is allowed only for the configured local development server.');
  return url.origin;
}
type Options = { method?: string; body?: unknown; signal?: AbortSignal; authenticated?: boolean };
export class ApiClient {
  readonly baseUrl: string;
  private session: StoredSession | null = null;
  private generation = 0;
  private refreshFlight: Promise<void> | null = null;
  private persistence: Promise<void> = Promise.resolve();
  private storage: SessionStorage;
  private onExpired: () => void;
  private fetcher: typeof fetch;
  private timeoutMs: number;
  constructor(baseUrl: string, storage: SessionStorage, onExpired: () => void = () => {}, fetcher: typeof fetch = fetch, timeoutMs = 15000) { this.baseUrl = normalizeServer(baseUrl); this.storage = storage; this.onExpired = onExpired; this.fetcher = fetcher; this.timeoutMs = timeoutMs; }
  private persist(session: StoredSession | null) {
    const next = this.persistence.catch(() => {}).then(() => session ? this.storage.set(JSON.stringify(session)) : this.storage.clear());
    this.persistence = next;
    return next;
  }
  async restore() {
    const generation = this.generation;
    const raw = await this.storage.get();
    if (!raw || generation !== this.generation) return false;
    let parsed: ReturnType<typeof storedSessionSchema.safeParse>;
    try { parsed = storedSessionSchema.safeParse(JSON.parse(raw)); } catch { parsed = { success: false } as typeof parsed; }
    if (!parsed.success || parsed.data.server !== this.baseUrl) { await this.persist(null); return false; }
    this.session = parsed.data;
    return true;
  }
  async signIn(login: string, password: string, totp?: string) {
    const generation = ++this.generation;
    this.session = null;
    this.refreshFlight = null;
    await this.persist(null);
    const body = await this.fetchJSON('/v1/auth/login', { method: 'POST', body: { login: login.trim(), password, ...(totp ? { totp: totp.trim() } : {}) } });
    const pair = tokenPairSchema.safeParse(body);
    if (!pair.success) throw new ApiError('INVALID_RESPONSE', 'Сервер вернул некорректную сессию.');
    if (generation !== this.generation) throw new ApiError('SESSION_CHANGED', 'Сессия была изменена.');
    const session = { server: this.baseUrl, accessToken: pair.data.access_token, refreshToken: pair.data.refresh_token, expiresAt: Date.now() + pair.data.expires_in * 1000 };
    await this.persist(session);
    if (generation !== this.generation) throw new ApiError('SESSION_CHANGED', 'Сессия была изменена.');
    this.session = session;
  }
  async signOut() {
    const token = this.session?.accessToken;
    ++this.generation;
    this.session = null;
    this.refreshFlight = null;
    this.onExpired();
    await this.persist(null);
    if (token) await this.fetchJSON('/v1/auth/logout', { method: 'POST' }, token).catch(() => {});
  }
  private async expire(generation: number) {
    if (generation !== this.generation) return;
    ++this.generation; this.session = null; this.onExpired();
    await this.persist(null);
  }
  private async refresh(generation: number) {
    if (this.refreshFlight) return this.refreshFlight;
    const refreshToken = this.session?.refreshToken;
    if (!refreshToken) throw new ApiError('UNAUTHORIZED', 'Войдите в приложение.', 401);
    const flight = (async () => {
      try {
        const json = await this.fetchJSON('/v1/auth/refresh', { method: 'POST', body: { refresh_token: refreshToken } });
        const pair = tokenPairSchema.safeParse(json);
        if (!pair.success) throw new ApiError('INVALID_RESPONSE', 'Сервер вернул некорректную сессию.');
        if (generation !== this.generation) throw new ApiError('SESSION_CHANGED', 'Сессия была изменена.');
        const session = { server: this.baseUrl, accessToken: pair.data.access_token, refreshToken: pair.data.refresh_token, expiresAt: Date.now() + pair.data.expires_in * 1000 };
        await this.persist(session);
        if (generation !== this.generation) throw new ApiError('SESSION_CHANGED', 'Сессия была изменена.');
        this.session = session;
      } catch (error) {
        if (error instanceof ApiError && (error.status === 401 || error.status === 403)) await this.expire(generation);
        throw error;
      }
    })();
    this.refreshFlight = flight;
    try { await flight; } finally { if (this.refreshFlight === flight) this.refreshFlight = null; }
  }
  async request<T>(path: string, schema: z.ZodType<T>, options: Options = {}): Promise<T> {
    const generation = this.generation;
    const authenticated = options.authenticated !== false;
    if (authenticated && !this.session) throw new ApiError('UNAUTHORIZED', 'Войдите в приложение.', 401);
    if (authenticated && this.session!.expiresAt <= Date.now() + 5000) await this.refresh(generation);
    const token = authenticated ? this.session!.accessToken : undefined;
    let json: unknown;
    try { json = await this.fetchJSON(path, options, token); }
    catch (error) {
      if (!(error instanceof ApiError) || error.status !== 401 || !authenticated) throw error;
      if (generation !== this.generation) throw new ApiError('SESSION_CHANGED', 'Сессия была изменена.');
      if (token === this.session?.accessToken) await this.refresh(generation);
      try { json = await this.fetchJSON(path, options, this.session?.accessToken); }
      catch (retryError) { if (retryError instanceof ApiError && retryError.status === 401) await this.expire(generation); throw retryError; }
    }
    if (generation !== this.generation) throw new ApiError('SESSION_CHANGED', 'Сессия была изменена.');
    const parsed = schema.safeParse(json);
    if (!parsed.success) throw new ApiError('INVALID_RESPONSE', 'Формат данных сервера не поддерживается. Обновите приложение или обратитесь в поддержку.');
    return parsed.data;
  }
  private async fetchJSON(path: string, options: Options, token?: string): Promise<unknown> {
    if (!path.startsWith('/v1/') || path.includes('://')) throw new ApiError('INVALID_PATH', 'Некорректный путь API.');
    const controller = new AbortController();
    let timedOut = false;
    const abort = () => controller.abort();
    options.signal?.addEventListener('abort', abort, { once: true });
    if (options.signal?.aborted) controller.abort();
    const timer = setTimeout(() => { timedOut = true; controller.abort(); }, this.timeoutMs);
    try {
      // Browser fetch is a platform method: do not invoke it with ApiClient as `this`.
      const fetcher = this.fetcher;
      const response = await fetcher(`${this.baseUrl}${path}`, { method: options.method || 'GET', headers: { Accept: 'application/json', ...(options.body !== undefined ? { 'Content-Type': 'application/json' } : {}), ...(token ? { Authorization: `Bearer ${token}` } : {}) }, ...(options.body !== undefined ? { body: JSON.stringify(options.body) } : {}), signal: controller.signal });
      if (response.status === 204) return undefined;
      let json: unknown;
      try { json = await response.json(); } catch { throw new ApiError('INVALID_RESPONSE', 'Сервер вернул ответ в неизвестном формате.', response.status); }
      if (!response.ok) {
        const body = json as { error?: { code?: unknown; message?: unknown; request_id?: unknown } };
        throw new ApiError(typeof body?.error?.code === 'string' ? body.error.code : 'REQUEST_FAILED', typeof body?.error?.message === 'string' ? body.error.message : `Ошибка сервера (${response.status}).`, response.status, typeof body?.error?.request_id === 'string' ? body.error.request_id : undefined);
      }
      return json;
    } catch (error) {
      if (error instanceof ApiError) throw error;
      if (options.signal?.aborted) throw new ApiError('ABORTED', 'Запрос отменён.');
      throw new ApiError(timedOut ? 'TIMEOUT' : 'NETWORK', timedOut ? 'Сервер не ответил вовремя. Проверьте соединение и повторите.' : 'Не удалось подключиться к серверу. Проверьте соединение.');
    } finally { clearTimeout(timer); options.signal?.removeEventListener('abort', abort); }
  }
}
export function errorMessage(error: unknown) {
  if (error instanceof ApiError) {
    const messages: Record<string, string> = { INVALID_CREDENTIALS: 'Неверный логин или пароль.', TOTP_REQUIRED: 'Введите действующий код из приложения аутентификации.', USER_INACTIVE: 'Учётная запись неактивна. Обратитесь к брокеру.', FORBIDDEN: 'Недостаточно прав для этого действия.', UNAUTHORIZED: 'Сессия истекла. Войдите снова.', INSUFFICIENT_FUNDS: 'Недостаточно доступных средств.', ACCOUNT_BLOCKED: 'Торговый счёт заблокирован.' };
    return messages[error.code] ?? error.message;
  }
  return error instanceof Error ? error.message : 'Не удалось выполнить действие. Попробуйте ещё раз.';
}

