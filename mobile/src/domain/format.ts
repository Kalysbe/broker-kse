import Decimal from 'decimal.js';

function format(value: string | null | undefined, currency: string, hidden: boolean, cash: boolean) {
  if (hidden) return '••• •••';
  if (value == null) return '—';
  try {
    const decimal = new Decimal(value);
    const [integer, fraction] = (cash ? decimal.toFixed(2) : decimal.toFixed()).split('.');
    const digits = fraction?.replace(/0+$/, '');
    return `${integer.replace(/\B(?=(\d{3})+(?!\d))/g, ' ')}${digits ? `,${digits}` : ''} ${currency === 'KGS' ? 'с' : currency}`;
  } catch { return '—'; }
}
export const formatAmount = (value: string | null | undefined, currency = 'KGS', hidden = false) => format(value, currency, hidden, true);
// A security's tick can be smaller than the currency's minor unit. Confirmations
// must show the exact limit price, rather than rounding it to two decimals.
export const formatPrice = (value: string | null | undefined, currency = 'KGS', hidden = false) => format(value, currency, hidden, false);
