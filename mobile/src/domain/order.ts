import Decimal from 'decimal.js';
const Exact = Decimal.clone({ precision: 50 });
export type OrderNumbers = { quantity: string; price: string; gross: string };
export function validateOrderNumbers(quantityText: string, priceText: string, lot: string, tick: string): OrderNumbers {
  const pattern = /^\d+(?:[.,]\d{1,8})?$/;
  if (!pattern.test(quantityText.trim()) || !pattern.test(priceText.trim())) throw new Error('Введите количество и цену: до 8 знаков после запятой.');
  const quantity = new Exact(quantityText.trim().replace(',', '.')), price = new Exact(priceText.trim().replace(',', '.'));
  const lotSize = new Exact(lot), tickSize = new Exact(tick);
  if (!quantity.greaterThan(0) || quantity.greaterThan('1000000000000') || !price.greaterThan(0) || price.greaterThan('1000000000000')) throw new Error('Количество и цена должны быть больше нуля и не превышать 1 000 000 000 000.');
  if (!lotSize.greaterThan(0) || !tickSize.greaterThan(0)) throw new Error('Для инструмента не настроены размер лота и шаг цены. Обратитесь к брокеру.');
  if (!quantity.mod(lotSize).isZero()) throw new Error(`Количество должно быть кратно размеру лота: ${lot}.`);
  if (!price.mod(tickSize).isZero()) throw new Error(`Цена должна быть кратна шагу цены: ${tick}.`);
  return { quantity: quantity.toFixed(), price: price.toFixed(), gross: quantity.mul(price).toFixed() };
}
export const terminalStatuses = new Set(['FILLED', 'CANCELLED', 'REJECTED', 'EXPIRED', 'FAILED']);
export function orderFailureMessage(reason: string) {
  let code = reason;
  try { const detail = JSON.parse(reason); if (typeof detail?.code === 'string') code = detail.code; } catch {}
  return ({
    ADAPTER_UNAVAILABLE: 'Связь с биржей временно недоступна. Проверьте статус заявки перед повторной отправкой.',
    ADAPTER_ERROR: 'Не удалось получить подтверждение биржи. Проверьте статус заявки перед повторной отправкой.',
    INSUFFICIENT_FUNDS: 'Недостаточно доступных средств для покупки с учётом комиссии.',
    INSUFFICIENT_POSITION: 'Недостаточно свободных бумаг для продажи.',
    PRICE_REQUIRED: 'Укажите лимитную цену.',
    LIMIT_EXCEEDED: 'Заявка превышает торговый лимит. Измените параметры или обратитесь к брокеру.',
    MARKET_CLOSED: 'Приём заявок сейчас закрыт. Проверьте расписание торгов.',
    ACCOUNT_BLOCKED: 'Торговля по этому счёту недоступна. Обратитесь к брокеру.',
  } as Record<string, string>)[code] ?? 'Уточните причину у брокера, указав номер заявки.';
}
export const orderStatusLabel = (status: string) => ({ QUEUED: 'В очереди', NEW: 'Создана', VALIDATING: 'Проверяется', VALIDATED: 'Проверена', PENDING: 'Ожидает отправки', ROUTING: 'Отправляется', SENT: 'Отправлена', ACKNOWLEDGED: 'Принята биржей', PARTIALLY_FILLED: 'Частично исполнена', FILLED: 'Исполнена', CANCEL_PENDING: 'Ожидает отмены', CANCELLED: 'Отменена', REJECTED: 'Отклонена', EXPIRED: 'Истёк срок', FAILED: 'Ошибка обработки' } as Record<string, string>)[status] ?? 'Статус уточняется';
