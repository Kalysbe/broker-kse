import { useQuery } from "@tanstack/react-query";
import { Link, useParams } from "react-router-dom";
import { api } from "../api";
import { useOrderStream } from "../useOrderStream";
import { quikOrderNumber } from "../quikOrderNumber";

type Order = { id: string; client_order_id: string; client_code: string; account_id: string; symbol: string; side: string; quantity: string; price: string; remaining_quantity: string; status: string; venue_order_id?: string; exchange_order_id?: string; venue_status?: string; reject_reason?: string };
type Tracking = { order: Order; updates?: { id: number; received_at: string; payload: { status?: string; [key: string]: unknown } }[]; gateway?: { order: { id: string; quik_order_id?: string; fix_cl_ord_id?: string; status: string; quantity?: string; filled_quantity?: string; reject_reason?: string }; events?: { id: number; event_type: string; created_at: string; message?: string; to_status?: string; raw_fix?: string }[]; trades?: unknown[] }; gateway_error?: string; checked_at?: string };

export function LoadOrderTrackingPage() {
  const { id } = useParams();
  const connected = useOrderStream(id);
  const q = useQuery({ queryKey: ["load-order-tracking", id], queryFn: () => api<Tracking>(`/v1/load-tests/orders/${encodeURIComponent(id || "")}${connected ? "" : "?refresh=true"}`), enabled: Boolean(id), refetchInterval: connected ? false : 30000 });
  const data = q.data;
  const local = data?.order;
  const remote = data?.gateway?.order;
  const exchangeID = quikOrderNumber(data);
  return <div className="space-y-5">
    <Link className="text-sm text-blue-600 hover:underline" to="/load-testing?tab=active">← Активные заявки</Link>
    <div className="page-heading"><div><p className="eyebrow">Отслеживание на бирже</p><h1 className="page-title">Заявка {exchangeID || local?.venue_order_id || id}</h1><p className="page-subtitle">{connected ? "Подписка на события заявки подключена." : "Переподключение к событиям. Резервная сверка каждые 30 секунд."}</p></div><button className="btn-primary" onClick={() => void api<Tracking>(`/v1/load-tests/orders/${encodeURIComponent(id || "")}?refresh=true`).then(() => q.refetch())} disabled={q.isFetching}>Обновить</button></div>
    {q.isLoading && <div className="surface p-5">Загрузка заявки…</div>}
    {q.error && <div className="surface p-5 text-red-700">{q.error instanceof Error ? q.error.message : "Не удалось загрузить заявку"}</div>}
    {local && <div className="surface p-5 space-y-4">
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <Value label="Номер заявки QUIK · FIX-тег 5002" value={exchangeID || "Ещё не получен"} />
        <Value label="FIX OrderID · тег 37" value={remote?.quik_order_id || local.exchange_order_id || "Ещё не получен"} />
        <Value label="ID записи шлюза" value={local.venue_order_id || "Ещё не отправлена"} />
        <Value label="Наш ID" value={local.id} />
        <Value label="ID запроса клиента" value={local.client_order_id} />
        <Value label="Клиент / счёт" value={`${local.client_code} / ${local.account_id}`} />
        <Value label="Статус шлюза" value={remote?.status || local.venue_status || "Ещё не проверен"} />
        <Value label="Инструмент / операция" value={`${local.symbol} / ${local.side}`} />
        <Value label="Количество / цена" value={`${remote?.quantity || local.quantity} / ${local.price}`} />
        <Value label="Исполнено" value={remote?.filled_quantity || "—"} />
        <Value label="FIX ClOrdID" value={remote?.fix_cl_ord_id || "—"} />
      </div>
      {data?.checked_at && <p className="text-xs text-slate-500">Проверка: {new Date(data.checked_at).toLocaleString("ru-RU", { timeZone: "Asia/Bishkek" })}</p>}
      {data?.gateway_error && <p className="rounded-xl bg-amber-50 p-3 text-sm text-amber-900">Текущий статус не подтверждён: {data.gateway_error}</p>}
      {(remote?.reject_reason || local.reject_reason) && <p className="rounded-xl bg-red-50 p-3 text-sm text-red-700">{remote?.reject_reason || local.reject_reason}</p>}
    </div>}
    {data?.gateway?.events && <div className="surface p-5 space-y-3"><h2 className="text-lg font-semibold">История событий шлюза</h2>{data.gateway.events.map(event => <details key={event.id} className="rounded-xl border border-slate-200 p-3"><summary className="cursor-pointer text-sm"><span className="text-slate-500">{new Date(event.created_at).toLocaleString("ru-RU", { timeZone: "Asia/Bishkek" })}</span> · <b>{event.event_type}</b> · {event.to_status}</summary><p className="mt-2 text-sm">{event.message}</p>{event.raw_fix && <pre className="mt-2 whitespace-pre-wrap break-all text-xs text-slate-600">{event.raw_fix}</pre>}</details>)}</div>}
    {!!data?.updates?.length && <div className="surface p-5 space-y-3"><h2 className="text-lg font-semibold">События подписки шлюза</h2>{data.updates.map(update => <details key={update.id} className="rounded-xl border border-slate-200 p-3"><summary className="cursor-pointer text-sm">{new Date(update.received_at).toLocaleString("ru-RU")} · {update.payload.status}</summary><pre className="mt-2 whitespace-pre-wrap break-all text-xs">{JSON.stringify(update.payload, null, 2)}</pre></details>)}</div>}
    {data?.gateway && <details className="surface p-5"><summary className="cursor-pointer font-semibold">Полный ответ шлюза</summary><pre className="mt-3 overflow-auto whitespace-pre-wrap break-all text-xs">{JSON.stringify(data.gateway, null, 2)}</pre></details>}
  </div>;
}

function Value({ label, value }: { label: string; value: string }) { return <div><div className="text-xs text-slate-500">{label}</div><div className="mt-1 break-all font-medium">{value}</div></div>; }
