import { test } from 'node:test';
import assert from 'node:assert/strict';
import { orderFailureMessage, validateOrderNumbers } from './order.ts';
test('uses exact decimals, supports comma input and respects lot/tick sizes', () => {
  assert.deepEqual(validateOrderNumbers('0,3', '0,1', '0.1', '0.01'), { quantity: '0.3', price: '0.1', gross: '0.03' });
  assert.deepEqual(validateOrderNumbers('10', '1250.05', '5', '0.05'), { quantity: '10', price: '1250.05', gross: '12500.5' });
});
test('rejects fractional lots, invalid ticks, scientific notation and excessive precision', () => {
  for (const [quantity, price, lot, tick] of [['6','10','5','0.01'],['5','1.01','1','0.05'],['1e3','1','1','0.01'],['0','1','1','0.01'],['1','0','1','0.01'],['1','0.000000001','1','0.000000001']]) assert.throws(() => validateOrderNumbers(quantity, price, lot, tick));
});

test('shows actionable order failures without raw adapter codes or gateway diagnostics', () => {
  assert.match(orderFailureMessage('ADAPTER_UNAVAILABLE'), /Связь с биржей/);
  assert.match(orderFailureMessage('INSUFFICIENT_FUNDS'), /Недостаточно доступных средств/);
  const detail = JSON.stringify({ code: 'ADAPTER_ERROR', path: '/api/v1/orders', response: 'internal diagnostic' });
  assert.match(orderFailureMessage(detail), /Проверьте статус заявки/);
  assert.doesNotMatch(orderFailureMessage(detail), /ADAPTER|api\/v1|internal diagnostic/);
  assert.match(orderFailureMessage('UNKNOWN_BACKEND_CODE'), /номер заявки/);
});
