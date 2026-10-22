import { FormEvent, useEffect, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, ApiRequestError } from "../api";
import { useOrderStream } from "../useOrderStream";
import { Link, useSearchParams } from "react-router-dom";

type Run = {
  id?: string; target?: "broker" | "exchange"; status: "IDLE" | "RUNNING" | "DRAINING" | "PASSED" | "FAILED" | "CANCELLED";
  rate?: number; duration_seconds?: number; workers?: number; planned?: number; sent?: number;
  started_at?: string; finished_at?: string; output?: string; error?: string;
  account?: string; client_code?: string; symbol?: string; board?: string; participants?: { account_id: string; client_code: string }[];
  queue?: { received: number; waiting: number; sent_to_gateway: number; failed: number; rejected: number; oldest_wait_seconds: number; estimated_drain_seconds: number; trading_rps: number; read_rps: number };
};
type RequestItem = { venue_order_id?: string; exchange_order_id?: string; venue_status?: string; venue_checked_at?: string; venue_check_error?: string; remaining_quantity?: string; quantity?: string; account_id?: string; client_code?: string; sequence: number; client_order_id: string; server_order_id?: string; side: string; price: string; http_code: number; latency_ms: number; error?: string; reject_reason?: string; verified: boolean; exchange_reached: boolean; status?: string };
type RequestPage = { items: RequestItem[]; total: number; page: number; page_size: number };

const presets = {
  broker: { account: "1-3101-2040", client_code: "Kalys", symbol: "GBA02280810", board: "LIST_GBA", price: "100" },
  exchange: { account: "1-3101-2040", client_code: "Kalys", symbol: "GBA02280810", board: "LIST_GBA", price: "90" },
};

export function LoadTestingPage() {
  const [searchParams] = useSearchParams();
  const qc = useQueryClient();
  const connected = useOrderStream();
  const [target, setTarget] = useState<"broker" | "exchange">("broker");
  const [rate, setRate] = useState(100);
  const [duration, setDuration] = useState(10);
  const [workers, setWorkers] = useState(100);
  const [adjustLimit, setAdjustLimit] = useState(true);
  const [details, setDetails] = useState(presets.broker);
  const [confirmed, setConfirmed] = useState(false);
  const [requestPage, setRequestPage] = useState(1);
  const [requestTab, setRequestTab] = useState<"all" | "errors" | "active">(searchParams.get("tab") === "active" ? "active" : "all");
  const q = useQuery({ queryKey: ["load-test"], queryFn: () => api<Run>("/v1/load-tests/current"), refetchInterval: (query) => isActiveRun(query.state.data) ? 1000 : 3000 });
  const start = useMutation({
    mutationFn: () => api<Run>("/v1/load-tests", { method: "POST", body: JSON.stringify({ target, rate, duration_seconds: duration, workers, verify_workers: Math.min(workers, 100), adjust_rate_limit: adjustLimit, confirm: confirmed, ...details }) }),
    onSuccess: (data) => { setConfirmed(false); setRequestPage(1); qc.setQueryData(["load-test"], data); },
  });
  const stop = useMutation({ mutationFn: () => api("/v1/load-tests/current/cancel", { method: "POST" }), onSuccess: () => void q.refetch() });
  useEffect(() => { setDetails(presets[target]); }, [target]);
  const run = q.data;
  const requestKind = requestTab === "errors" ? "&kind=errors" : "";
  const requests = useQuery({ queryKey: ["load-test-requests", run?.id, requestTab, requestPage], queryFn: () => api<RequestPage>(requestTab === "active" ? `/v1/load-tests/active?page=${requestPage}&page_size=50` : `/v1/load-tests/current/requests?page=${requestPage}&page_size=50${requestKind}`), enabled: requestTab === "active" || Boolean(run?.id), refetchInterval: connected ? (requestTab !== "active" && isActiveRun(run) ? 1000 : false) : 30000 });
  const activeCount = useQuery({ queryKey: ["active-load-orders-count"], queryFn: () => api<RequestPage>("/v1/load-tests/active?page=1&page_size=1"), refetchInterval: connected ? false : 30000 });
  const errorCount = useQuery({ queryKey: ["load-test-request-errors", run?.id], queryFn: () => api<RequestPage>("/v1/load-tests/current/requests?page=1&page_size=1&kind=errors"), enabled: Boolean(run?.id), refetchInterval: isActiveRun(run) || Boolean(run?.queue?.waiting) ? 1000 : false });
  const metrics = useMemo(() => parseMetrics(run?.output || ""), [run?.output]);
  const planned = run?.planned || rate * duration;
  const sent = Math.max(run?.sent || 0, metrics.sent || 0);
  const progress = planned ? Math.min(100, sent / planned * 100) : 0;
  const error = start.error instanceof ApiRequestError ? start.error.message : start.error ? "Не удалось запустить тест" : "";
  const requestPages = Math.max(1, Math.ceil((requests.data?.total || 0) / 50));

  function submit(event: FormEvent) { event.preventDefault(); start.mutate(); }
  return (
    <div className="space-y-5">
      <div className="page-heading">
        <div><p className="eyebrow">Контроль производительности</p><h1 className="page-title">Нагрузочное тестирование</h1><p className="page-subtitle">Отправляйте поток тестовых заявок и сразу проверяйте потери, дубли и задержки.</p></div>
        <StatusBadge status={run?.status || "IDLE"} />
      </div>

      <div className="rounded-2xl border border-amber-200 bg-amber-50 px-5 py-4 text-sm text-amber-900">
        <b>Важно:</b> это не имитация интерфейса. Система отправляет реальные заявки с тестовым токеном. Используйте только согласованное тестовое окно биржи.
      </div>

      <div className="grid gap-5 xl:grid-cols-[420px_1fr]">
        <form className="surface p-5 space-y-5" onSubmit={submit}>
          <div><h2 className="text-lg font-semibold text-slate-950">Параметры нагрузки</h2><p className="mt-1 text-sm text-slate-500">Токены хранятся на сервере и не попадают в браузер.</p></div>
          <div className="grid grid-cols-2 gap-2 rounded-xl bg-slate-100 p-1">
            <TargetButton active={target === "broker"} onClick={() => setTarget("broker")}>Наша система</TargetButton>
            <TargetButton active={target === "exchange"} onClick={() => setTarget("exchange")}>Биржа через очередь</TargetButton>
          </div>
          <p className="text-sm text-slate-500">Заявки сначала сохраняются в нашей комнате ожидания. К бирже отправляется до 20 торговых запросов/с; чтение ограничено отдельно — до 100/с.</p>
          <div className="grid grid-cols-3 gap-3">
            <Field label="Заявок/сек"><input type="number" min={1} max={100000} value={rate} onChange={(e) => setRate(+e.target.value)} /></Field>
            <Field label="Секунд"><input type="number" min={1} max={300} value={duration} onChange={(e) => setDuration(+e.target.value)} /></Field>
            <Field label="Воркеров"><input type="number" min={1} max={5000} value={workers} onChange={(e) => setWorkers(+e.target.value)} /></Field>
          </div>
          <div className="rounded-xl border border-slate-200 p-4 space-y-3">
            <div className="flex items-center justify-between"><span className="text-sm font-semibold">Заявка</span><span className="text-xs text-slate-400">BUY / SELL по очереди</span></div>
            <div className="grid grid-cols-2 gap-3">
              <Field label="Торговый счёт"><input value={details.account} onChange={(e) => setDetails({ ...details, account: e.target.value })} /></Field>
              <div className="col-span-2 rounded-lg bg-blue-50 p-3 text-sm text-blue-900"><b>Клиенты в случайном порядке</b><div className="mt-1">Kalys · Imar · Temirlan · Emil</div><div className="mt-1 text-xs">Каждая группа из четырёх заявок включает всех клиентов. Общий счёт: 1-3101-2040.</div></div>
              <Field label="Инструмент"><input value={details.symbol} onChange={(e) => setDetails({ ...details, symbol: e.target.value })} /></Field>
              <Field label="Режим"><input value={details.board} onChange={(e) => setDetails({ ...details, board: e.target.value })} /></Field>
              <Field label="Базовая цена"><input value={details.price} onChange={(e) => setDetails({ ...details, price: e.target.value })} /></Field>
            </div>
            <div className="rounded-lg bg-violet-50 px-3 py-2 text-xs text-violet-700">Количество каждой заявки — случайное целое число от 100 до 1000 включительно. Цена каждой заявки автоматически меняется в диапазоне от −5% до +5% от базовой.</div>
          </div>
          <label className="flex gap-3 rounded-xl bg-blue-50 p-3 text-sm text-blue-900"><input type="checkbox" checked={adjustLimit} onChange={(e) => setAdjustLimit(e.target.checked)} /><span><b>Поднять входящий лимит тестового токена</b><br/><span className="text-blue-700">Позволяет наполнить очередь. Лимиты биржи остаются 20/с и 100/с.</span></span></label>
          <label className="flex items-start gap-3 text-sm text-slate-700"><input type="checkbox" checked={confirmed} onChange={(e) => setConfirmed(e.target.checked)} /><span>Подтверждаю отправку <b>{(rate * duration).toLocaleString("ru-RU")}</b> тестовых заявок.</span></label>
          {error && <div className="rounded-xl bg-red-50 p-3 text-sm text-red-700">{error}</div>}
          <button className="btn-primary w-full py-3" disabled={!confirmed || isActiveRun(run) || Boolean(run?.queue?.waiting) || start.isPending}>{start.isPending ? "Запускаю…" : "Запустить тест"}</button>
        </form>

        <div className="space-y-5">
          <div className="surface overflow-hidden">
            <div className="flex flex-wrap items-center justify-between gap-3 border-b border-slate-100 p-5">
              <div><h2 className="text-lg font-semibold">Ход теста</h2><p className="text-sm text-slate-500">{run?.status === "IDLE" ? "Тест ещё не запускался" : `${run?.rate} заявок/сек · ${run?.duration_seconds} сек · ${run?.workers} воркеров`}</p></div>
              {run?.status === "RUNNING" && <button className="rounded-xl border border-red-200 px-4 py-2 text-sm font-semibold text-red-600 hover:bg-red-50" onClick={() => stop.mutate()} disabled={stop.isPending}>Остановить генератор</button>}
            </div>
            <div className="p-5">
              <div className="mb-2 flex justify-between text-sm"><span>{sent.toLocaleString("ru-RU")} из {planned.toLocaleString("ru-RU")}</span><b>{progress.toFixed(0)}%</b></div>
              <div className="h-3 overflow-hidden rounded-full bg-slate-100"><div className={`h-full rounded-full transition-all duration-500 ${run?.status === "FAILED" ? "bg-red-500" : "bg-blue-600"}`} style={{ width: `${progress}%` }} /></div>
            </div>
          </div>
          <div className="surface p-5 space-y-4">
            <div><h2 className="text-lg font-semibold">Комната ожидания запросов</h2><p className="mt-1 text-sm text-slate-500">Входящий поток → сохранённая очередь → шлюз биржи. Принятые заявки продолжают отправляться после остановки генератора и перезапуска сервиса.</p></div>
            <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
              <Metric label="Принято в очередь" value={run?.queue?.received} tone="blue" />
              <Metric label="Ожидают отправки" value={run?.queue?.waiting} tone="amber" />
              <Metric label="Передано шлюзу" value={run?.queue?.sent_to_gateway} tone="emerald" />
              <Metric label="Ошибки обработки" value={run?.queue ? run.queue.failed + run.queue.rejected : undefined} tone="red" />
            </div>
            <div className="flex flex-wrap gap-x-6 gap-y-2 text-sm text-slate-600"><span>Торговые: <b>{run?.queue?.trading_rps ?? 20}/с</b></span><span>Чтение: <b>{run?.queue?.read_rps ?? 100}/с</b></span><span>Самая старая заявка: <b>{Math.floor(run?.queue?.oldest_wait_seconds ?? 0)} сек</b></span><span>Минимальное время разгрузки: <b>≈ {run?.queue?.estimated_drain_seconds ?? 0} сек</b></span></div>
            {run?.status === "DRAINING" && <p className="rounded-xl bg-amber-50 p-3 text-sm text-amber-900">Генератор закончил работу. Очередь продолжает отправку в пределах лимита; ожидание не считается ошибкой.</p>}
            {run?.error && <p className="rounded-xl bg-red-50 p-3 text-sm text-red-700">{run.error}</p>}
          </div>
          <div className="grid grid-cols-2 gap-3 lg:grid-cols-4 2xl:grid-cols-7">
            <Metric label="Отправлено" value={metrics.sent ?? sent} tone="blue" />
            <Metric label="Принято API" value={run?.queue?.received ?? metrics.accepted} tone="emerald" />
            <Metric label="Передано шлюзу" value={run?.queue?.sent_to_gateway ?? metrics.exchange} tone="violet" />
            <Metric label="В нашей очереди" value={run?.queue?.waiting ?? metrics.pending} tone={run?.queue?.waiting ? "amber" : "emerald"} />
            <Metric label="Ошибки адаптера" value={run?.queue?.failed ?? metrics.adapterFailed} tone={run?.queue?.failed ? "red" : "emerald"} />
            <Metric label="Потеряно" value={metrics.lost} tone={metrics.lost ? "red" : "emerald"} />
            <Metric label="HTTP-отказы" value={metrics.rejected} tone={metrics.rejected ? "red" : "emerald"} />
          </div>
          {metrics.reason && <div className="rounded-2xl border border-red-200 bg-red-50 p-5 text-red-900"><div className="font-semibold">Почему тест не прошёл</div><div className="mt-1 text-sm">{metrics.reason}</div></div>}
          <div className="grid gap-3 sm:grid-cols-3">
            <Latency label="Обычная задержка, p50" value={metrics.p50} />
            <Latency label="95% запросов, p95" value={metrics.p95} />
            <Latency label="Пиковая задержка, p99" value={metrics.p99} />
          </div>
          <div className="surface overflow-hidden">
            <div className="flex flex-wrap items-center justify-between gap-3 border-b border-slate-100 p-5">
              <div><h2 className="text-lg font-semibold">Журнал запросов</h2><p className="text-sm text-slate-500">{requestTab === "active" ? "Активных заявок из всех тестов" : requestTab === "errors" ? "Ошибочных запросов" : "Всего запросов"}: {(requests.data?.total || 0).toLocaleString("ru-RU")}</p></div>
              <div className="flex items-center gap-2 text-sm"><button className="grid-page" disabled={requestPage <= 1} onClick={() => setRequestPage((p) => p - 1)}>←</button><span>Страница {requestPage} из {requestPages}</span><button className="grid-page" disabled={requestPage >= requestPages} onClick={() => setRequestPage((p) => p + 1)}>→</button></div>
            </div>
            <div className="flex gap-1 border-b border-slate-200 bg-slate-50 px-5 pt-3">
              <button className={`rounded-t-xl px-4 py-2.5 text-sm font-semibold ${requestTab === "all" ? "border border-b-white border-slate-200 bg-white text-blue-700" : "text-slate-500 hover:text-slate-900"}`} onClick={() => { setRequestTab("all"); setRequestPage(1); }}>Все запросы</button>
              <button className={`rounded-t-xl px-4 py-2.5 text-sm font-semibold ${requestTab === "active" ? "border border-b-white border-slate-200 bg-white text-emerald-700" : "text-slate-500 hover:text-slate-900"}`} onClick={() => { setRequestTab("active"); setRequestPage(1); }}>Активные заявки <span className="ml-1 rounded-full bg-emerald-100 px-2 py-0.5 text-xs text-emerald-700">{activeCount.data?.total || 0}</span></button>
              <button className={`rounded-t-xl px-4 py-2.5 text-sm font-semibold ${requestTab === "errors" ? "border border-b-white border-slate-200 bg-white text-red-700" : "text-slate-500 hover:text-slate-900"}`} onClick={() => { setRequestTab("errors"); setRequestPage(1); }}>Ошибки <span className="ml-1 rounded-full bg-red-100 px-2 py-0.5 text-xs text-red-700">{(errorCount.data?.total || 0).toLocaleString("ru-RU")}</span></button>
            </div>
            <div className="overflow-x-auto">
              <table className="w-full min-w-[1050px] text-sm">
                <thead><tr className="border-b border-slate-200 bg-slate-50 text-left text-xs uppercase tracking-wide text-slate-500"><th className="px-4 py-3">№</th><th className="px-4 py-3">ID заявки</th><th className="px-4 py-3">Клиент</th><th className="px-4 py-3">Операция</th><th className="px-4 py-3 text-right">Кол-во</th><th className="px-4 py-3 text-right">Цена</th><th className="px-4 py-3">Содержимое</th><th className="px-4 py-3">HTTP</th><th className="px-4 py-3">Статус</th>{requestTab === "errors" && <th className="px-4 py-3">Описание ошибки</th>}<th className="px-4 py-3 text-right">Задержка</th></tr></thead>
                <tbody>
                  {(requests.data?.items || []).map((item) => <tr key={item.server_order_id || item.client_order_id} className={`border-b border-slate-100 align-top ${requestTab === "errors" ? "bg-red-50/20 hover:bg-red-50/50" : "hover:bg-blue-50/30"}`}><td className="px-4 py-3 text-slate-400">{item.sequence + 1}</td><td className="max-w-[210px] truncate px-4 py-3 font-mono text-xs" title={item.client_order_id}>{item.server_order_id ? <Link className="text-blue-600 hover:underline" to={`/load-testing/orders/${encodeURIComponent(item.server_order_id)}`}>{item.client_order_id}</Link> : item.client_order_id}{item.venue_order_id && <Link className="mt-1 block text-blue-600 hover:underline" to={`/load-testing/orders/${encodeURIComponent(item.server_order_id || "")}`}>{item.exchange_order_id ? `FIX OrderID (37): ${item.exchange_order_id}` : `Шлюз: ${item.venue_order_id}`}</Link>}</td><td className="px-4 py-3 font-medium">{item.client_code || run?.client_code || "—"}</td><td className={`px-4 py-3 font-semibold ${item.side === "BUY" ? "text-emerald-700" : "text-red-600"}`}>{item.side === "BUY" ? "Покупка" : "Продажа"}</td><td className="px-4 py-3 text-right font-medium">{item.quantity || "—"}</td><td className="px-4 py-3 text-right font-medium">{item.price}</td><td className="px-4 py-3"><details><summary className="cursor-pointer text-blue-600">JSON</summary><pre className="mt-2 max-w-md whitespace-pre-wrap rounded-lg bg-slate-950 p-3 text-[11px] text-slate-200">{JSON.stringify({ account_id: item.account_id || run?.account, client_code: item.client_code || run?.client_code, client_order_id: item.client_order_id, symbol: run?.symbol, board: run?.board, side: item.side, order_type: "LIMIT", time_in_force: "DAY", quantity: item.quantity, price: item.price }, null, 2)}</pre></details></td><td className="px-4 py-3"><span className={`rounded-full px-2 py-1 text-xs font-semibold ${item.http_code >= 200 && item.http_code < 300 ? "bg-emerald-50 text-emerald-700" : "bg-red-50 text-red-700"}`}>{item.http_code || (requestTab === "active" ? "—" : "Ошибка")}</span></td><td className="px-4 py-3"><div className="font-medium">{requestTab === "active" ? item.venue_status : requestStatus(item)}</div>{item.venue_checked_at && <div className="mt-1 text-xs text-slate-500">Проверено: {new Date(item.venue_checked_at).toLocaleString("ru-RU", { timeZone: "Asia/Bishkek" })}</div>}</td>{requestTab === "errors" && <td className="max-w-sm px-4 py-3 text-sm text-red-700">{errorDescription(item)}<details className="mt-2"><summary className="cursor-pointer">Полная запись ошибки</summary><pre className="max-w-md whitespace-pre-wrap break-all text-xs">{formatDiagnostic(item.reject_reason || item.error || "")}</pre></details></td>}<td className="px-4 py-3 text-right tabular-nums">{requestTab === "active" ? "—" : `${item.latency_ms.toFixed(2)} мс`}</td></tr>)}
                  {!requests.isLoading && !(requests.data?.items.length) && <tr><td colSpan={requestTab === "errors" ? 11 : 10} className="px-4 py-10 text-center text-slate-400">{requestTab === "active" ? "Подтверждённых активных заявок пока нет" : requestTab === "errors" ? "Ошибок пока нет" : "Запросы появятся здесь после запуска теста"}</td></tr>}
                </tbody>
              </table>
            </div>
          </div>
          <details className="surface p-5" open={run?.status === "FAILED"}><summary className="cursor-pointer font-semibold">Технический журнал</summary><pre className="mt-4 max-h-64 overflow-auto whitespace-pre-wrap rounded-xl bg-slate-950 p-4 text-xs leading-5 text-slate-200">{run?.output || "После запуска здесь появится журнал генератора нагрузки."}{run?.error ? `\n${run.error}` : ""}</pre></details>
        </div>
      </div>
    </div>
  );
}

function parseMetrics(output: string) {
  const summary = /ИТОГ: отправлено=(\d+) принято=(\d+) отклонено_HTTP=(\d+).*?подтверждено_биржей=(\d+) в_очереди=(\d+) ошибки_адаптера=(\d+) потеряно=(\d+) дубли=(\d+)/.exec(output);
  const latency = /Задержка POST: p50=([\d.]+)ms p95=([\d.]+)ms p99=([\d.]+)ms/.exec(output);
  const reason = /Причина отказа \(\d+\): (.+)/.exec(output);
  return { sent: summary ? +summary[1] : undefined, accepted: summary ? +summary[2] : undefined, rejected: summary ? +summary[3] : undefined, exchange: summary ? +summary[4] : undefined, pending: summary ? +summary[5] : undefined, adapterFailed: summary ? +summary[6] : undefined, lost: summary ? +summary[7] : undefined, duplicates: summary ? +summary[8] : undefined, reason: reason?.[1], p50: latency?.[1], p95: latency?.[2], p99: latency?.[3] };
}
function Field({ label, children }: { label: string; children: React.ReactElement }) { return <label className="block text-xs font-medium text-slate-500">{label}{<span className="mt-1 block [&>input]:w-full [&>input]:border [&>input]:border-slate-200 [&>input]:bg-white [&>input]:px-3 [&>input]:py-2.5 [&>input]:text-sm [&>input]:text-slate-900 [&>input]:outline-none [&>input]:focus:border-blue-400 [&>input]:focus:ring-4 [&>input]:focus:ring-blue-100">{children}</span>}</label>; }
function TargetButton({ active, onClick, children }: { active: boolean; onClick: () => void; children: string }) { return <button type="button" onClick={onClick} className={`rounded-lg px-3 py-2.5 text-sm font-semibold transition ${active ? "bg-white text-blue-700 shadow-sm" : "text-slate-500"}`}>{children}</button>; }
function Metric({ label, value, tone }: { label: string; value?: number; tone: string }) { const colors: Record<string,string> = { blue: "bg-blue-50 text-blue-700", emerald: "bg-emerald-50 text-emerald-700", violet: "bg-violet-50 text-violet-700", amber: "bg-amber-50 text-amber-700", red: "bg-red-50 text-red-700" }; return <div className={`rounded-2xl p-4 ${colors[tone]}`}><div className="text-xs opacity-75">{label}</div><div className="mt-2 text-2xl font-bold">{value == null ? "—" : value.toLocaleString("ru-RU")}</div></div>; }
function Latency({ label, value }: { label: string; value?: string }) { return <div className="surface p-4"><div className="text-xs text-slate-500">{label}</div><div className="mt-1 text-xl font-semibold">{value ? `${value} мс` : "—"}</div></div>; }
function StatusBadge({ status }: { status: Run["status"] }) { const map = { IDLE: ["Готов к запуску", "bg-slate-100 text-slate-600"], RUNNING: ["Тест выполняется", "bg-blue-100 text-blue-700"], DRAINING: ["Очередь обрабатывается", "bg-amber-100 text-amber-700"], PASSED: ["Пройден", "bg-emerald-100 text-emerald-700"], FAILED: ["Есть ошибки", "bg-red-100 text-red-700"], CANCELLED: ["Остановлен", "bg-amber-100 text-amber-700"] }; return <span className={`rounded-full px-3 py-1.5 text-sm font-semibold ${map[status][1]}`}>{map[status][0]}</span>; }
function requestStatus(item: RequestItem) { if (item.error) return "Отклонён"; const labels: Record<string,string> = { QUEUED: "В очереди", ROUTING: "Маршрутизация", SENT: "Отправлен", ACKNOWLEDGED: "Принят биржей", REJECTED: "Отклонён биржей", FAILED: "Ошибка адаптера", FILLED: "Исполнен", PARTIALLY_FILLED: "Частично исполнен", CANCELLED: "Отменён" }; return labels[item.status || ""] || item.status || (item.http_code ? "Принят API" : "Отправляется"); }
function errorDescription(item: RequestItem) { const reason = item.reject_reason || item.error || ""; const descriptions: Record<string,string> = { ADAPTER_ERROR: "Исходная причина не сохранена. Сторона сбоя неизвестна.", ADAPTER_UNAVAILABLE: "Нет доступного торгового адаптера для отправки заявки.", REJECTED: "Биржа отклонила заявку. Подробная причина указана в ответе биржи.", RATE_LIMITED: "Превышен разрешённый лимит запросов в секунду.", VALIDATION_ERROR: "Заявка не прошла проверку входных данных." }; if (descriptions[reason]) return `${reason}: ${descriptions[reason]}`; if (reason) { try { const parsed = JSON.parse(reason); const code = parsed?.code || parsed?.error?.code || parsed?.details?.code; const sources: Record<string,string> = { OUR_SYSTEM: "Наша система", GATEWAY: "Шлюз биржи (приём биржей не подтверждён)", UNKNOWN: "Сторона не установлена" }; const message = parsed?.message ? `${sources[parsed.source] || parsed.source || ""} · ${parsed.stage || ""} · ${parsed.message}` : parsed?.error?.message || parsed?.error; return `${code ? `${code}: ` : ""}${message || reason}`; } catch { return reason; } } return item.status === "FAILED" ? descriptions.ADAPTER_ERROR : "Причина не передана внешней системой"; }

function formatDiagnostic(reason: string) { try { return JSON.stringify(JSON.parse(reason), null, 2); } catch { return reason; } }

function isActiveRun(run?: Run) { return run?.status === 'RUNNING' || run?.status === 'DRAINING'; }
