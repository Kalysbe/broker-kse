// Synthetic accounts in the dedicated sbis-mobile-dev database only.
import Decimal from 'decimal.js';
const base = 'http://127.0.0.1:18080';
const accountId = '77777777-7777-7777-7777-777777777777';
const clientId = '55555555-5555-5555-5555-555555555555';
async function request(path, body, token) {
  const response = await fetch(base + path, { method: body ? 'POST' : 'GET', headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: `Bearer ${token}` } : {}) }, ...(body ? { body: JSON.stringify(body) } : {}) });
  const data = await response.json();
  if (!response.ok) throw new Error(`${path}: ${data.error?.code || response.status}`);
  return data;
}
const session = await request('/v1/auth/login', { login: 'mobile-admin', password: 'Mobile_Dev123' });
const users = await request('/v1/users', null, session.access_token);
if (!users.items.some(user => user.login === 'mobile-client')) await request('/v1/users', { login: 'mobile-client', password: 'Mobile_Dev123', email: 'mobile-client@example.invalid', first_name: 'Мобильный', last_name: 'Клиент', roles: ['CLIENT'], client_id: clientId }, session.access_token);
const cash = await request(`/v1/accounts/${accountId}/cash`, null, session.access_token);
if (new Decimal(cash.buckets.AVAILABLE ?? '0').lessThanOrEqualTo(0)) await request('/v1/deposits/card/mock', { account_id: accountId, amount: '100000', card_number: '4242424242424242', expiry: '12/30', cvc: '123', cardholder: 'LOCAL TEST', idempotency_key: 'mobile-local-funding-v1' }, session.access_token);
const client = await request('/v1/auth/login', { login: 'mobile-client', password: 'Mobile_Dev123' });
const me = await request('/v1/auth/me', null, client.access_token);
const accounts = await request('/v1/accounts', null, client.access_token);
if (me.client_id !== clientId || accounts.items.some(account => account.client_id !== clientId)) throw new Error('Client scope validation failed');
console.log(`Local client ready. CLIENT role; ${accounts.items.length} account(s). Credentials: mobile-client / Mobile_Dev123`);
