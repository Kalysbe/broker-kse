import { test } from 'node:test';
import assert from 'node:assert/strict';
import { formatAmount, formatPrice } from './format.ts';

test('shows the exact security price down to the smallest supported tick', () => {
  assert.equal(formatPrice('0.00000001', 'KZT'), '0,00000001 KZT');
  assert.equal(formatPrice('1234567.0100', 'KGS'), '1 234 567,01 с');
  assert.equal(formatPrice('400.015', 'KZT'), '400,015 KZT');
});
test('formats cash separately, preserves missing values and masks private amounts', () => {
  assert.equal(formatAmount('400.015', 'KZT'), '400,02 KZT');
  assert.equal(formatAmount('-1000000', 'KGS'), '-1 000 000 с');
  assert.equal(formatAmount(undefined), '—');
  assert.equal(formatPrice(undefined), '—');
  assert.equal(formatAmount('1000', 'KZT', true), '••• •••');
});
