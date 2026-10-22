export type QuikOrderSnapshot = {
  order?: { exchange_order_id?: string; quik_order_num?: string };
  gateway?: {
    order?: { quik_order_id?: string; quik_order_num?: string };
    events?: { raw_fix?: string; payload?: { raw_fix?: string } }[];
  };
};

// ARQA tag 37 is the composite FIX OrderID used for routing/cancellation.
// Tag 5002 is the actual numeric order number displayed in the QUIK table.
export function quikOrderNumber(snapshot?: QuikOrderSnapshot): string | undefined {
  const numeric = (value?: string) => value && /^\d+$/.test(value) ? value : undefined;
  const explicit = numeric(snapshot?.gateway?.order?.quik_order_num) || numeric(snapshot?.order?.quik_order_num);
  if (explicit) return explicit;
  const fixOrderID = snapshot?.gateway?.order?.quik_order_id || snapshot?.order?.exchange_order_id;
  const events = snapshot?.gateway?.events || [];
  for (let i = events.length - 1; i >= 0; i--) {
    const raw = events[i].raw_fix || events[i].payload?.raw_fix;
    if (!raw) continue;
    const fields = new Map(raw.split(/[|\x01]/).map(field => {
      const separator = field.indexOf("=");
      return [field.slice(0, separator), field.slice(separator + 1)] as const;
    }));
    if (fields.get("35") !== "8" || (fixOrderID && fields.get("37") !== fixOrderID)) continue;
    const number = numeric(fields.get("5002"));
    if (number) return number;
  }
  // Tag 37 is a separate identifier, even when it happens to contain only digits.
  return undefined;
}
