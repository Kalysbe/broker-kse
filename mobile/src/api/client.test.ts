import { test } from 'node:test';
import assert from 'node:assert/strict';
import { z } from 'zod';
import { ApiClient, ApiError, normalizeServer, type SessionStorage } from './client.ts';

const url = 'http://127.0.0.1:18080';
const session = (expiresAt = Date.now() + 60000) => ({ server: url, accessToken: 'old-access', refreshToken: 'old-refresh', expiresAt });
function memory(initial: unknown = session()) { let raw = initial === null ? null : JSON.stringify(initial); const storage: SessionStorage = { get: async () => raw, set: async value => { raw = value; }, clear: async () => { raw = null; } }; return { storage, read: () => raw }; }
const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
const tokens = { access_token: 'new-access', refresh_token: 'new-refresh', expires_in: 900, token_type: 'Bearer' };
const unauthorized = () => json({ error: { code: 'UNAUTHORIZED', message: 'Unauthorized' } }, 401);

test('accepts local development origins and rejects credentials or remote HTTP', () => {
  assert.equal(normalizeServer(url + '/'), url);
  assert.equal(normalizeServer('https://broker.example'), 'https://broker.example');
  for (const invalid of ['http://broker.example', 'https://name:secret@broker.example', 'https://broker.example/path', 'https://broker.example?token=secret', 'file:///tmp/x']) assert.throws(() => normalizeServer(invalid));
});

test('allows only the configured private LAN origin for Wi-Fi development', () => {
  const previous = process.env.EXPO_PUBLIC_API_URL;
  try {
    process.env.EXPO_PUBLIC_API_URL = 'http://192.168.20.96:18080';
    assert.equal(normalizeServer('http://192.168.20.96:18080/'), 'http://192.168.20.96:18080');
    for (const invalid of ['http://192.168.20.97:18080', 'http://192.168.20.96:8080', 'http://10.1.2.3:18080', 'http://name:secret@192.168.20.96:18080', 'http://192.168.20.96:18080/path']) assert.throws(() => normalizeServer(invalid));
    process.env.EXPO_PUBLIC_API_URL = 'http://203.0.113.20:18080';
    assert.throws(() => normalizeServer(process.env.EXPO_PUBLIC_API_URL!));
  } finally { if (previous === undefined) delete process.env.EXPO_PUBLIC_API_URL; else process.env.EXPO_PUBLIC_API_URL = previous; }
});
test('parallel expired requests share a single refresh and use the rotated tokens', async () => {
  const store = memory(session(0)); let refreshes = 0;
  const fetcher: typeof fetch = async (input, init) => {
    if (String(input).endsWith('/auth/refresh')) { refreshes++; await new Promise(resolve => setTimeout(resolve, 5)); return json(tokens); }
    assert.equal(new Headers(init?.headers).get('Authorization'), 'Bearer new-access'); return json({ ok: true });
  };
  const api = new ApiClient(url, store.storage, () => {}, fetcher); await api.restore();
  await Promise.all([api.request('/v1/a', z.object({ ok: z.boolean() })), api.request('/v1/b', z.object({ ok: z.boolean() }))]);
  assert.equal(refreshes, 1); assert.equal(JSON.parse(store.read()!).refreshToken, 'new-refresh');
});
test('401 refreshes once and retries the request; 403 on a resource does not log out', async () => {
  const store = memory(); let expired = 0, requests = 0;
  const api = new ApiClient(url, store.storage, () => expired++, async (input, init) => {
    if (String(input).endsWith('/auth/refresh')) return json(tokens);
    if (String(input).endsWith('/forbidden')) return json({ error: { code: 'FORBIDDEN', message: 'Forbidden' } }, 403);
    requests++; return new Headers(init?.headers).get('Authorization') === 'Bearer new-access' ? json({ ok: true }) : unauthorized();
  });
  await api.restore(); await api.request('/v1/account', z.object({ ok: z.boolean() }));
  assert.equal(requests, 2); await assert.rejects(api.request('/v1/forbidden', z.unknown()), error => error instanceof ApiError && error.status === 403); assert.equal(expired, 0);
});
test('logout during refresh cannot restore the old session', async () => {
  const store = memory(session(0)); let release!: () => void, started!: () => void;
  const startedPromise = new Promise<void>(resolve => { started = resolve; });
  const releasePromise = new Promise<void>(resolve => { release = resolve; });
  const api = new ApiClient(url, store.storage, () => {}, async input => {
    if (String(input).endsWith('/auth/refresh')) { started(); await releasePromise; return json(tokens); }
    return new Response(null, { status: 204 });
  });
  await api.restore(); const pending = api.request('/v1/accounts', z.unknown());
  const rejected = assert.rejects(pending, error => error instanceof ApiError && error.code === 'SESSION_CHANGED');
  await startedPromise; await api.signOut(); release(); await rejected;
  assert.equal(store.read(), null); await assert.rejects(api.request('/v1/accounts', z.unknown()), error => error instanceof ApiError && error.status === 401);
});
test('network failure during refresh preserves the session; rejected refresh clears it', async () => {
  for (const rejected of [false, true]) {
    const store = memory(session(0)); let expired = 0;
    const api = new ApiClient(url, store.storage, () => expired++, async () => { if (rejected) return unauthorized(); throw new TypeError('offline'); });
    await api.restore(); await assert.rejects(api.request('/v1/accounts', z.unknown()));
    assert.equal(expired, rejected ? 1 : 0); assert.equal(store.read() === null, rejected);
  }
});
test('rejects invalid response shapes and does not reuse tokens for another server', async () => {
  const wrongHost = memory({ ...session(), server: 'https://other.example' });
  const api = new ApiClient(url, wrongHost.storage); assert.equal(await api.restore(), false); assert.equal(wrongHost.read(), null);
  const store = memory(); const invalid = new ApiClient(url, store.storage, () => {}, async () => json({ buckets: { AVAILABLE: 50 } }));
  await invalid.restore(); await assert.rejects(invalid.request('/v1/cash', z.object({ buckets: z.record(z.string(), z.string()) })), error => error instanceof ApiError && error.code === 'INVALID_RESPONSE');
});
test('sign-in never persists the password or TOTP', async () => {
  const store = memory(null); const api = new ApiClient(url, store.storage, () => {}, async (_input, init) => {
    assert.equal(JSON.parse(String(init?.body)).password, ' secret password '); return json(tokens);
  });
  await api.signIn(' user ', ' secret password ', '123456');
  assert.ok(store.read()); assert.ok(!store.read()!.includes('password')); assert.ok(!store.read()!.includes('123456'));
});

test('calls platform fetch without binding it to the client instance', async () => {
  const store = memory();
  const fetcher: typeof fetch = async function (this: unknown) { assert.equal(this, undefined); return json({ ok: true }); };
  const api = new ApiClient(url, store.storage, () => {}, fetcher);
  await api.restore(); await api.request('/v1/check', z.object({ ok: z.boolean() }));
});

