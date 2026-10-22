const labels: Record<string, string> = {
  NEW: "Создана",
  VALIDATING: "Проверяется",
  VALIDATED: "Проверена",
  PENDING: "Ожидает отправки",
  ROUTING: "Отправляется на биржу",
  SENT: "Отправлена на биржу",
  ACKNOWLEDGED: "Принята биржей",
  PARTIALLY_FILLED: "Исполнена частично",
  FILLED: "Исполнена",
  CANCEL_PENDING: "Отмена обрабатывается",
  CANCELLED: "Отменена",
  REJECTED: "Отклонена",
  EXPIRED: "Срок истёк",
  FAILED: "Ошибка исполнения",
  ACTIVE: "Активен",
  BLOCKED: "Заблокирован",
  CLOSED: "Закрыт",
  COMPLETED: "Завершено",
  REQUESTED: "Ожидает подтверждения",
  PROCESSING: "Обрабатывается",
  APPROVED: "Подтверждено",
};

function tone(status: string) {
  if (["FILLED", "COMPLETED", "ACTIVE", "APPROVED"].includes(status)) return "bg-emerald-50 text-emerald-700 ring-emerald-200";
  if (["REJECTED", "FAILED", "BLOCKED"].includes(status)) return "bg-rose-50 text-rose-700 ring-rose-200";
  if (["CANCELLED", "EXPIRED", "CLOSED"].includes(status)) return "bg-slate-100 text-slate-600 ring-slate-200";
  if (["ACKNOWLEDGED", "SENT", "ROUTING"].includes(status)) return "bg-blue-50 text-blue-700 ring-blue-200";
  return "bg-amber-50 text-amber-700 ring-amber-200";
}

export function StatusBadge({ status }: { status: unknown }) {
  const code = String(status || "").toUpperCase();
  return <span title={code} className={`inline-flex items-center gap-1.5 whitespace-nowrap rounded-full px-2.5 py-1 text-xs font-semibold ring-1 ring-inset ${tone(code)}`}><span className="h-1.5 w-1.5 rounded-full bg-current opacity-70" />{labels[code] || String(status || "—")}</span>;
}

export function SideBadge({ side }: { side: unknown }) {
  const buy = String(side).toUpperCase() === "BUY";
  return <span className={`text-xs font-bold ${buy ? "text-emerald-700" : "text-rose-700"}`}>{buy ? "Покупка" : "Продажа"}</span>;
}

export function formatRejectReason(reason: unknown) {
  const text = String(reason || "");
  const priceRange = text.match(/Order price ([\d.]+) is out of set interval from ([\d.]+) to ([\d.]+)/i);
  if (priceRange) return `Цена ${priceRange[1]} вне допустимого диапазона ${priceRange[2]}–${priceRange[3]}`;
  if (/insufficient funds/i.test(text)) return "Недостаточно денежных средств";
  if (/insufficient position/i.test(text)) return "Недостаточно ценных бумаг";
  return text;
}
