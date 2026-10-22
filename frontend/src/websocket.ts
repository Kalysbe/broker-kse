export type SocketMessage = { type: string; path?: string; id?: string; status?: number; data?: unknown; event_type?: string; order_id?: string; upstream_connected?: boolean };
type Pending = { resolve: (value: SocketMessage) => void; reject: (error: Error) => void; timer: ReturnType<typeof setTimeout> };
const listeners = new Set<(message: SocketMessage) => void>();
const pending = new Map<string, Pending>();
const watches = new Set<string>();
let socket: WebSocket | undefined;
let opening: Promise<void> | undefined;
let activeToken = "";
let activeBase = "";
let ready = false;
let retry: ReturnType<typeof setTimeout> | undefined;
let retryDelay = 1000;
let sequence = 0;
export function socketConnected() { return ready; }
const emit = (message: SocketMessage) => listeners.forEach(listener => listener(message));

export function subscribeSocket(listener: (message: SocketMessage) => void) {
  listeners.add(listener);
  return () => { listeners.delete(listener); };
}

export function closeSocket() {
  activeToken = ""; ready = false; opening = undefined;
  clearTimeout(retry); watches.clear();
  const previous = socket; socket = undefined; previous?.close();
  pending.forEach(item => { clearTimeout(item.timer); item.reject(new Error("Соединение закрыто")); }); pending.clear();
}

async function connect(base: string, token: string): Promise<void> {
  if (activeToken !== token || activeBase !== base) { closeSocket(); activeToken = token; activeBase = base; }
  if (ready && socket?.readyState === WebSocket.OPEN) return;
  if (opening) return opening;
  const url = new URL(`${base}/v1/realtime/ws`, window.location.origin);
  url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
  const ws = new WebSocket(url); socket = ws;
  opening = new Promise<void>((resolve, reject) => {
    const timeout = setTimeout(() => { reject(new Error("WebSocket: время подключения истекло")); ws.close(); }, 10000);
    ws.onopen = () => ws.send(JSON.stringify({ type: "auth", token }));
    ws.onmessage = event => {
      if (socket !== ws) return;
      let message: SocketMessage;
      try { message = JSON.parse(event.data); } catch { ws.close(); return; }
      if (message.type === "unauthorized") { emit(message); reject(new Error("UNAUTHORIZED")); closeSocket(); return; }
      if (message.type === "ready") {
        clearTimeout(timeout); ready = true; opening = undefined; retryDelay = 1000; resolve();
        watches.forEach(path => { ws.send(JSON.stringify({ type: "request", id: `replay-${++sequence}`, method: "GET", path, watch: true })); });
      }
      if (message.type === "response" && message.id) {
        const request = pending.get(message.id);
        if (request) { clearTimeout(request.timer); pending.delete(message.id); request.resolve(message); }
        else if (message.id.startsWith("replay-")) emit({ ...message, type: "snapshot" });
      }
      emit(message);
    };
    ws.onclose = () => {
      clearTimeout(timeout);
      if (socket !== ws) { reject(new Error("Соединение закрыто")); return; }
      socket = undefined; ready = false; opening = undefined; reject(new Error("WebSocket отключён"));
      pending.forEach(item => { clearTimeout(item.timer); item.reject(new Error("Соединение прервано; результат запроса неизвестен")); }); pending.clear();
      emit({ type: "disconnected", upstream_connected: false });
      if (activeToken) { retry = setTimeout(() => { void connect(activeBase, activeToken).catch(() => undefined); }, retryDelay); retryDelay = Math.min(retryDelay * 2, 30000); }
    };
    ws.onerror = () => ws.close();
  });
  return opening;
}

export async function socketRequest(base: string, token: string, path: string, init: RequestInit): Promise<SocketMessage> {
  await connect(base, token);
  if (!ready || !socket || activeToken !== token) throw new Error("WebSocket недоступен");
  const method = (init.method || "GET").toUpperCase();
  const watch = method === "GET" && (path === "/v1/market" || path === "/v1/positions" || path === "/v1/orders" || /^\/v1\/market\/[^/]+\/book$/.test(path));
  if (watch) {
    if (path.endsWith("/book")) watches.forEach(previous => { if (previous.endsWith("/book") && previous !== path) { socket?.send(JSON.stringify({ type: "unwatch", path: previous })); watches.delete(previous); } });
    watches.add(path);
  }
  const id = String(++sequence);
  const headers: Record<string, string> = {};
  const input = new Headers(init.headers);
  ["Idempotency-Key", "X-Request-ID", "X-Trace-ID"].forEach(name => { const value = input.get(name); if (value) headers[name] = value; });
  const body = typeof init.body === "string" ? JSON.parse(init.body) : undefined;
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => { pending.delete(id); reject(new Error("Время ожидания ответа истекло; результат запроса неизвестен")); }, 35000);
    pending.set(id, { resolve, reject, timer });
    try { socket!.send(JSON.stringify({ type: "request", id, method, path, body, headers, watch })); }
    catch (error) { clearTimeout(timer); pending.delete(id); reject(error); }
  });
}
