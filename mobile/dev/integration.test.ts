// Run only against `npm run dev:backend`: this fixture has no external trading gateway.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { randomUUID } from 'node:crypto';
import { z } from 'zod';
import { ApiClient, ApiError, type SessionStorage } from '../src/api/client.ts';
import { accountSchema, cashSchema, instrumentSchema, ledgerSchema, listSchema, notificationSchema, orderSchema, portfolioSchema, quoteSchema, userSchema } from '../src/api/schemas.ts';

const server = 'http://127.0.0.1:18080';
const accountId = '77777777-7777-7777-7777-777777777777';
const instrumentId = '33333333-3333-3333-3333-333333333333';
const foreignAccountId = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';
test('isolated Go backend: client scope, schema contracts, refresh, idempotent order and cancel', async () => {
  let saved: string | null = null;
  const storage: SessionStorage = { get: async () => saved, set: async value => { saved = value; }, clear: async () => { saved = null; } };
  const api = new ApiClient(server, storage);
  await api.signIn('mobile-client', 'Mobile_Dev123');
  try {
    const me = await api.request('/v1/auth/me', userSchema);
    assert.deepEqual(me.roles, ['CLIENT']);
    assert.equal(me.client_id, '55555555-5555-5555-5555-555555555555');
    const accounts = await api.request('/v1/accounts', listSchema(accountSchema));
    assert.ok(accounts.items.some(item => item.id === accountId));
    assert.ok(accounts.items.every(item => item.client_id === me.client_id));
    assert.ok(!accounts.items.some(item => item.id === foreignAccountId));
    for (const resource of ['cash', 'portfolio', 'ledger']) await assert.rejects(api.request(`/v1/accounts/${foreignAccountId}/${resource}`, z.unknown()), error => error instanceof ApiError && error.status === 403);
    await Promise.all([
      api.request(`/v1/accounts/${accountId}/cash`, cashSchema),
      api.request(`/v1/accounts/${accountId}/portfolio`, portfolioSchema),
      api.request(`/v1/accounts/${accountId}/ledger`, listSchema(ledgerSchema)),
      api.request('/v1/instruments', listSchema(instrumentSchema)),
      api.request('/v1/market', listSchema(quoteSchema)),
      api.request('/v1/notifications', listSchema(notificationSchema)),
    ]);
    await assert.rejects(api.request(`/v1/accounts/${randomUUID()}/cash`, z.unknown()), error => error instanceof ApiError && [403, 404].includes(error.status));
    const old = JSON.parse(saved!);
    saved = JSON.stringify({ ...old, expiresAt: 0 });
    const restored = new ApiClient(server, storage);
    assert.equal(await restored.restore(), true);
    await restored.request('/v1/auth/me', userSchema);
    assert.notEqual(JSON.parse(saved!).refreshToken, old.refreshToken);
    const body = { client_order_id: randomUUID(), account_id: accountId, instrument_id: instrumentId, side: 'BUY', order_type: 'LIMIT', time_in_force: 'DAY', quantity: '1', price: '400' };
    await assert.rejects(restored.request('/v1/orders', orderSchema, { method: 'POST', body: { ...body, account_id: foreignAccountId } }), error => error instanceof ApiError && error.status === 403);
    const [created, concurrent] = await Promise.all([restored.request('/v1/orders', orderSchema, { method: 'POST', body }), restored.request('/v1/orders', orderSchema, { method: 'POST', body })]);
    assert.equal(concurrent.id, created.id, 'Concurrent submissions must create a single order');
    assert.equal(created.account_id, accountId);
    assert.ok(!['REJECTED', 'FAILED'].includes(created.status), `Unexpected local order state: ${created.status} ${created.reject_reason || ''}`);
    const duplicate = await restored.request('/v1/orders', orderSchema, { method: 'POST', body });
    assert.equal(duplicate.id, created.id);
    const parallel = await Promise.all([restored.request('/v1/orders', orderSchema, { method: 'POST', body }), restored.request('/v1/orders', orderSchema, { method: 'POST', body })]);
    assert.ok(parallel.every(order => order.id === created.id));
    await assert.rejects(restored.request('/v1/orders', orderSchema, { method: 'POST', body: { ...body, quantity: '2' } }), error => error instanceof ApiError && error.code === 'IDEMPOTENCY_CONFLICT');
    const orders = await restored.request('/v1/orders', listSchema(orderSchema));
    assert.equal(orders.items.filter(item => item.client_order_id === body.client_order_id).length, 1);
    if (duplicate.can_cancel) {
      const cancelled = await restored.request(`/v1/orders/${created.id}/cancel`, orderSchema, { method: 'POST' });
      assert.ok(['CANCELLED', 'CANCEL_PENDING', 'FILLED'].includes(cancelled.status));
    } else {
      await assert.rejects(restored.request(`/v1/orders/${created.id}/cancel`, orderSchema, { method: 'POST' }), error => error instanceof ApiError && error.code === 'ORDER_INVALID_TRANSITION');
    }
    const before = await restored.request(`/v1/accounts/${accountId}/cash`, cashSchema);
    const rejected = await restored.request('/v1/orders', orderSchema, { method: 'POST', body: { ...body, client_order_id: randomUUID(), price: '100000000' } });
    assert.equal(rejected.status, 'REJECTED');
    assert.equal(rejected.reject_reason, 'INSUFFICIENT_FUNDS');
    const after = await restored.request(`/v1/accounts/${accountId}/cash`, cashSchema);
    assert.deepEqual(after.buckets, before.buckets, 'Rejected buy must preserve all cash buckets');
    await restored.signOut();
    assert.equal(saved, null);
    await assert.rejects(restored.request('/v1/accounts', listSchema(accountSchema)), error => error instanceof ApiError && error.status === 401);
  } finally { await api.signOut(); }
});
