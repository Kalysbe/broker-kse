import { useEffect, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useForm } from "react-hook-form";
import { useTranslation } from "react-i18next";
import { api } from "../api";
import { useOrderStream } from "../useOrderStream";
import { formatRejectReason, SideBadge, StatusBadge } from "../components/StatusBadge";
import { useAuth } from "../auth";
import { FormError } from "../ui";

type MarketItem = {
  id: string;
  ticker: string;
  isin: string;
  name: string;
  board: string;
  currency: string;
  status: string;
  last?: string;
  bid?: string;
  ask?: string;
  volume?: string;
  source?: string;
  venue_symbol?: string;
};

type PositionRow = {
  kind?: string;
  account_id?: string;
  client_code?: string;
  symbol?: string;
  currency?: string;
  quantity?: string;
  available_qty?: string;
  blocked_qty?: string;
};
type Order = {
  id: string;
  client_order_id: string;
  side: string;
  status: string;
  quantity: string;
  filled_quantity?: string;
  price?: string;
  symbol?: string;
  board?: string;
  can_cancel?: boolean;
  reject_reason?: string;
};
type BookLevel = { price: string; quantity: string };
type Book = { bids: BookLevel[]; asks: BookLevel[]; live?: boolean; source?: string; updated_at?: string; last?: string; currency?: string };

export function TradingPage() {
  const { t } = useTranslation();
  const { can } = useAuth();
  const qc = useQueryClient();
  const connected = useOrderStream(undefined, can("orders.read"), true);
  const [q, setQ] = useState("");
  const [selectedId, setSelectedId] = useState("");
  const writable = can("orders.submit");
  const canCancel = can("orders.cancel");

  const market = useQuery({
    queryKey: ["market"],
    queryFn: () => api<{ items: MarketItem[] }>("/v1/market"),
    refetchInterval: false,
  });
  const positions = useQuery({
    queryKey: ["positions"],
    queryFn: () => api<{ items: PositionRow[] }>("/v1/positions"),
    refetchInterval: false,
  });
  const orders = useQuery({
    queryKey: ["orders"],
    queryFn: () => api<{ items: Order[] }>("/v1/orders"),
    refetchInterval: false,
  });
  const book = useQuery({
    queryKey: ["book", selectedId],
    queryFn: () => api<Book>(`/v1/market/${selectedId}/book`),
    enabled: !!selectedId,
    refetchInterval: false,
  });

  const items = market.data?.items || [];
  const filtered = useMemo(() => {
    const s = q.trim().toLowerCase();
    if (!s) return items;
    return items.filter((i) =>
      [i.ticker, i.name, i.isin, i.board].some((v) => (v || "").toLowerCase().includes(s)),
    );
  }, [items, q]);
  const selected = items.find((i) => i.id === selectedId) || filtered[0];

  useEffect(() => {
    if (!selectedId && filtered[0]) setSelectedId(filtered[0].id);
  }, [filtered, selectedId]);

  const form = useForm({
    defaultValues: {
      instrument_id: "",
      side: "BUY",
      order_type: "LIMIT",
      time_in_force: "DAY",
      quantity: "1",
      price: "",
    },
  });
  const venueAccount = useMemo(() => {
    const rows = positions.data?.items || [];
    const sec = rows.find((r) => (r.kind || "").toUpperCase() !== "CASH" && (r.account_id || "").trim());
    if (sec?.account_id) return sec.account_id.trim();
    const any = rows.find((r) => (r.account_id || "").trim());
    return (any?.account_id || "").trim();
  }, [positions.data]);
  const venueClientCode = useMemo(() => {
    const rows = positions.data?.items || [];
    return (rows.find((r) => (r.kind || "").toUpperCase() !== "CASH" && (r.account_id || "").trim() === venueAccount)?.client_code || "").trim();
  }, [positions.data, venueAccount]);
  const exchangeCash = useMemo(() => {
    const rows = positions.data?.items || [];
    return rows.find((r) =>
      (r.kind || "").toUpperCase() === "CASH"
      && (!venueClientCode || (r.client_code || "").trim().toLowerCase() === venueClientCode.toLowerCase()),
    );
  }, [positions.data, venueClientCode]);
  const exchangeSecurity = useMemo(() => {
    const symbol = selected?.venue_symbol || selected?.ticker || "";
    return (positions.data?.items || []).find((r) =>
      (r.kind || "").toUpperCase() !== "CASH"
      && (r.symbol || "").trim().toLowerCase() === symbol.trim().toLowerCase()
      && (!venueClientCode || (r.client_code || "").trim().toLowerCase() === venueClientCode.toLowerCase()),
    );
  }, [positions.data, selected?.ticker, selected?.venue_symbol, venueClientCode]);
  const availableCash = exchangeCash?.available_qty || exchangeCash?.quantity;
  const availableSecurity = exchangeSecurity?.available_qty || exchangeSecurity?.quantity;
  const side = form.watch("side");
  const qty = form.watch("quantity");
  const price = form.watch("price");
  const ot = form.watch("order_type");

  useEffect(() => {
    if (!selected) return;
    form.setValue("instrument_id", selected.id);
    const quote = side === "BUY" ? selected.ask || selected.last : selected.bid || selected.last;
    if (quote) form.setValue("price", quote);
  }, [selected?.id, side]);

  const est = Number(qty || 0) * Number(price || 0);

  const mut = useMutation({
    mutationFn: (v: Record<string, string>) => {
      const body: Record<string, string> = {
        instrument_id: v.instrument_id,
        side: v.side,
        order_type: v.order_type,
        time_in_force: v.time_in_force,
        quantity: v.quantity,
        client_order_id: `UI-${crypto.randomUUID()}`,
      };
      if (v.order_type !== "MARKET" && v.price) body.price = v.price;
      return api("/v1/orders", { method: "POST", body: JSON.stringify(body) });
    },
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["orders"] });
      void qc.invalidateQueries({ queryKey: ["positions"] });
    },
  });

  const cancelMut = useMutation({
    mutationFn: (id: string) => api(`/v1/orders/${id}/cancel`, { method: "POST" }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["orders"] }),
  });

  const asks = [...(book.data?.asks || [])].slice(0, 12).reverse();
  const bids = (book.data?.bids || []).slice(0, 12);
  const maxDepth = Math.max(1, ...[...asks, ...bids].map((level) => Number(level.quantity) || 0));
  const bestBid = Number(book.data?.bids?.[0]?.price || selected?.bid || 0);
  const bestAsk = Number(book.data?.asks?.[0]?.price || selected?.ask || 0);
  const spread = bestBid > 0 && bestAsk > 0 ? bestAsk - bestBid : 0;
  const activeStatuses = new Set(["NEW", "VALIDATING", "VALIDATED", "PENDING", "ROUTING", "SENT", "ACKNOWLEDGED", "PARTIALLY_FILLED", "CANCEL_PENDING"]);
  const selectedOrders = (orders.data?.items || []).filter((order) => !selected || order.symbol === selected.ticker);
  const activeOrders = selectedOrders.filter((order) => activeStatuses.has(order.status));

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div><p className="eyebrow">Торговый терминал</p><h1 className="page-title">Рынок КФБ</h1><p className="page-subtitle">{connected ? "Живая подписка на стакан и заявки подключена." : "Переподключение WebSocket…"}</p></div>
        <div className="flex items-center gap-3 rounded-xl border border-slate-200 bg-white px-4 py-2 text-xs text-slate-500"><span className="h-2 w-2 animate-pulse rounded-full bg-emerald-500"/><span>Рынок подключён</span><span className="h-4 w-px bg-slate-200"/><span>Обновлено {market.dataUpdatedAt ? new Date(market.dataUpdatedAt).toLocaleTimeString("ru-RU") : "—"}</span><button className="font-semibold text-blue-600" onClick={() => { void market.refetch(); void book.refetch(); }}>Обновить</button></div>
      </div>

      <div className="grid gap-4 xl:grid-cols-[310px_minmax(420px,1fr)_350px]">
      <div className="surface min-h-[690px] overflow-hidden">
        <div className="flex items-center justify-between gap-3 mb-3">
          <div className="px-4 pt-4"><h2 className="font-semibold text-slate-900">Инструменты</h2><div className="mt-0.5 text-xs text-slate-400">{filtered.length} доступно</div></div>
        </div>
        <input
          className="mx-4 mb-3 w-[calc(100%-2rem)] rounded-xl border border-slate-200 bg-slate-50 px-3 py-2.5 text-sm outline-none focus:border-blue-400 focus:bg-white focus:ring-4 focus:ring-blue-100"
          placeholder={t("trade.search")}
          value={q}
          onChange={(e) => setQ(e.target.value)}
        />
        <div className="max-h-[600px] overflow-auto border-t border-slate-100">
              {filtered.length === 0 && <div className="px-4 py-10 text-center text-sm text-slate-400">{t("trade.noMarket")}</div>}
              {filtered.map((i) => {
                const on = selected?.id === i.id;
                return (
                  <button
                    key={i.id}
                    className={`flex w-full items-center justify-between gap-3 border-b border-slate-100 px-4 py-3 text-left transition ${on ? "bg-blue-50 shadow-[inset_3px_0_0_#2563eb]" : "hover:bg-slate-50"}`}
                    onClick={() => setSelectedId(i.id)}
                  >
                    <div className="min-w-0"><div className="truncate font-semibold text-slate-900">{i.ticker}</div><div className="mt-0.5 truncate text-[11px] text-slate-400">{i.board || "Без режима"} · {i.currency}</div></div>
                    <div className="text-right"><div className="font-semibold tabular-nums text-slate-800">{i.last || "—"}</div><div className="mt-0.5 text-[11px] text-slate-400">{i.bid || "—"} / {i.ask || "—"}</div></div>
                  </button>
                );
              })}
        </div>
      </div>

      <div className="space-y-4">
        <div className="surface overflow-hidden">
          {!selected ? (
            <div className="p-10 text-center text-sm text-slate-400">{t("trade.pickInstrument")}</div>
          ) : (
            <>
              <div className="border-b border-slate-200 p-5">
                <div className="flex items-start justify-between"><div><div className="flex items-center gap-2"><h2 className="text-xl font-bold text-slate-950">{selected.ticker}</h2><span className="rounded-md bg-emerald-50 px-2 py-1 text-[10px] font-bold text-emerald-700">ТОРГИ ИДУТ</span></div><p className="mt-1 text-sm text-slate-500">{selected.name}</p></div><div className="text-right"><div className="text-3xl font-semibold tabular-nums text-slate-950">{selected.last || "—"}</div><div className="text-xs font-medium text-slate-400">{selected.currency}</div></div></div>
                <div className="mt-5 grid grid-cols-4 gap-3"><Stat label="ISIN" value={selected.isin || "—"}/><Stat label="Режим" value={selected.board || "—"}/><Stat label="Тип" value={(selected as MarketItem & { security_type?: string }).security_type || "Ценная бумага"}/><Stat label="Объём" value={selected.volume || "—"}/></div>
              </div>
              <div className="flex items-center justify-between border-b border-slate-100 px-5 py-3"><div><div className="flex items-center gap-2"><h3 className="font-semibold text-slate-900">Стакан торгов</h3><span className={`rounded-full px-2 py-0.5 text-[10px] font-bold ${book.data?.live ? "bg-emerald-50 text-emerald-700" : "bg-slate-100 text-slate-500"}`}>{book.data?.live ? "LIVE · КФБ" : "Локальные данные"}</span></div><p className="text-xs text-slate-400">Нажмите на цену, чтобы подставить её в заявку{book.data?.updated_at ? ` · ${new Date(book.data.updated_at).toLocaleTimeString("ru-RU")}` : ""}</p></div><div className="text-right text-xs"><span className="text-slate-400">Спред</span><div className="font-semibold tabular-nums text-slate-700">{spread > 0 ? spread.toFixed(2) : "—"}</div></div></div>
              <div className="grid grid-cols-[1fr_1fr_1fr] px-5 py-2 text-[10px] font-semibold uppercase tracking-wider text-slate-400"><span>Цена</span><span className="text-right">Количество</span><span className="text-right">Мои заявки</span></div>
              <div className="min-h-[345px] px-2 pb-3">
                {asks.length === 0 && bids.length === 0 && <div className="py-24 text-center"><div className="text-2xl text-slate-300">≋</div><div className="mt-2 text-sm font-medium text-slate-500">Стакан пока пуст</div><div className="mt-1 text-xs text-slate-400">Биржа не передала активные уровни</div></div>}
                {asks.map((level, i) => <DepthRow key={`a${i}`} level={level} side="ASK" max={maxDepth} own={activeOrders.filter((o) => o.side === "SELL" && o.price === level.price).reduce((sum,o)=>sum+Number(o.quantity),0)} onPick={() => { form.setValue("side", "BUY"); form.setValue("price", level.price); }}/>) }
                {(asks.length > 0 || bids.length > 0) && <div className="my-1 flex items-center justify-between rounded-lg bg-slate-50 px-3 py-2 text-xs"><span className="font-medium text-slate-500">Рыночный спред</span><span className="font-semibold tabular-nums text-slate-800">{bestBid || "—"} ↔ {bestAsk || "—"}</span></div>}
                {bids.map((level, i) => <DepthRow key={`b${i}`} level={level} side="BID" max={maxDepth} own={activeOrders.filter((o) => o.side === "BUY" && o.price === level.price).reduce((sum,o)=>sum+Number(o.quantity),0)} onPick={() => { form.setValue("side", "SELL"); form.setValue("price", level.price); }}/>) }
              </div>
            </>
          )}
        </div>
      </div>

        <form className="surface self-start overflow-hidden" onSubmit={form.handleSubmit((v) => mut.mutate(v))}>
          <div className="border-b border-slate-200 px-5 py-4"><h2 className="font-semibold text-slate-900">Новая заявка</h2><p className="mt-0.5 text-xs text-slate-400">{selected?.ticker || "Выберите инструмент"}</p></div>
          <div className="p-5"><div className="mb-4 rounded-xl border border-blue-100 bg-blue-50/70 p-3">
            <div className="flex items-center justify-between gap-3 text-xs">
              <span className="text-slate-500">{t("trade.account")}</span>
              <span className="font-medium tabular-nums text-slate-700">{venueAccount || t("trade.accountAuto")}</span>
            </div>
            <div className="mt-2 flex items-end justify-between gap-3 border-t border-blue-100 pt-2">
              <div><div className="text-[11px] font-medium uppercase tracking-wide text-blue-600">Доступно для торговли</div><div className="mt-0.5 text-xl font-bold tabular-nums text-slate-950">{formatAmount(availableCash)} <span className="text-xs font-semibold text-slate-500">{exchangeCash?.currency || selected?.currency || "KGS"}</span></div></div>
              <span className="rounded-full bg-white px-2 py-1 text-[10px] font-semibold text-emerald-700 shadow-sm">КФБ</span>
            </div>
            {side === "SELL" && <div className="mt-2 text-xs text-slate-500">Доступно бумаг: <b className="tabular-nums text-slate-800">{formatAmount(availableSecurity)}</b></div>}
          </div>
          <div className="grid grid-cols-2 rounded-xl bg-slate-100 p-1 mb-4">
            <button
              type="button"
              className={`rounded-lg py-2.5 text-sm font-semibold transition ${side === "BUY" ? "bg-emerald-600 text-white shadow-sm" : "text-slate-500 hover:text-slate-800"}`}
              onClick={() => form.setValue("side", "BUY")}
            >
              {t("trade.buy")}
            </button>
            <button
              type="button"
              className={`rounded-lg py-2.5 text-sm font-semibold transition ${side === "SELL" ? "bg-rose-600 text-white shadow-sm" : "text-slate-500 hover:text-slate-800"}`}
              onClick={() => form.setValue("side", "SELL")}
            >
              {t("trade.sell")}
            </button>
          </div>
          <label className="block text-sm mb-3">{t("trade.type")}
            <select className="mt-1 w-full rounded-xl border border-slate-200 bg-white px-3 py-2.5 outline-none focus:border-blue-400 focus:ring-4 focus:ring-blue-100" {...form.register("order_type")}>
              <option value="LIMIT">Лимитная</option>
              <option value="MARKET">Рыночная</option>
            </select>
          </label>
          <label className="block text-sm mb-3">{t("trade.tif")}
            <select className="mt-1 w-full rounded-xl border border-slate-200 bg-white px-3 py-2.5 outline-none focus:border-blue-400 focus:ring-4 focus:ring-blue-100" {...form.register("time_in_force")}>
              <option value="DAY">До конца дня</option><option value="IOC">Исполнить сразу доступное</option><option value="FOK">Всё или ничего</option><option value="GTC">До отмены</option>
            </select>
          </label>
          <label className="block text-sm mb-3">{t("trade.qty")}
            <input className="mt-1 w-full rounded-xl border border-slate-200 px-3 py-2.5 outline-none focus:border-blue-400 focus:ring-4 focus:ring-blue-100" {...form.register("quantity")} />
          </label>
          {ot !== "MARKET" && (
            <label className="block text-sm mb-3">{t("trade.price")}
              <div className="relative"><input className="mt-1 w-full rounded-xl border border-slate-200 px-3 py-2.5 pr-14 outline-none focus:border-blue-400 focus:ring-4 focus:ring-blue-100" {...form.register("price")} /><span className="absolute right-3 top-3.5 text-xs font-medium text-slate-400">{selected?.currency}</span></div>
            </label>
          )}
          <div className="mb-4 rounded-xl bg-slate-50 p-3 text-sm"><div className="flex justify-between text-slate-500"><span>{t("trade.preview")}</span><b className="tabular-nums text-slate-900">{Number.isFinite(est) ? est.toFixed(2) : "—"} {selected?.currency}</b></div><div className="mt-2 flex justify-between text-xs text-slate-400"><span>Комиссия (оценочно)</span><span>{Number.isFinite(est) ? (est * .001).toFixed(2) : "—"} {selected?.currency}</span></div></div>
          <FormError error={mut.error} />
          <button className={`w-full rounded-xl py-3 text-sm font-semibold text-white shadow-sm transition disabled:cursor-not-allowed disabled:bg-slate-300 ${side === "BUY" ? "bg-emerald-600 hover:bg-emerald-700" : "bg-rose-600 hover:bg-rose-700"}`} disabled={!writable || mut.isPending || !selected}>
            {mut.isPending ? "Отправка…" : side === "BUY" ? "Выставить заявку на покупку" : "Выставить заявку на продажу"}
          </button>
          <p className="mt-3 text-center text-[11px] text-slate-400">Заявка будет передана напрямую в шлюз КФБ</p></div>
        </form>
      </div>

      <div className="surface p-5">
        <div className="mb-3 flex items-center justify-between"><div><h2 className="font-semibold text-slate-900">Мои заявки</h2><p className="mt-0.5 text-xs text-slate-400">Активные и последние заявки по выбранному инструменту</p></div><span className="rounded-full bg-blue-50 px-3 py-1 text-xs font-semibold text-blue-700">Активных: {activeOrders.length}</span></div>
        <table className="w-full text-sm">
          <thead>
            <tr className="text-left text-muted border-b border-line">
              <th className="py-2">{t("trade.instrument")}</th>
              <th>Операция</th>
              <th>{t("trade.qty")}</th>
              <th>{t("trade.price")}</th>
              <th>{t("common.status")}</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {selectedOrders.length === 0 && (
              <tr><td className="py-6 text-muted" colSpan={6}>{t("common.empty")}</td></tr>
            )}
            {selectedOrders.map((o) => (
              <tr key={o.id} className="border-b border-line">
                <td className="py-2">{o.symbol || o.client_order_id}</td>
                <td><SideBadge side={o.side} /></td>
                <td>{o.quantity}{o.filled_quantity ? ` / ${o.filled_quantity}` : ""}</td>
                <td>{o.price || "—"}</td>
                <td>
                  <StatusBadge status={o.status} />
                  {o.reject_reason ? (
                    <span className="text-xs text-red-700 ml-1">
                      {t(`errors.${o.reject_reason}`, { defaultValue: formatRejectReason(o.reject_reason) })}
                    </span>
                  ) : null}
                </td>
                <td className="text-right">
                  {canCancel && o.can_cancel ? (
                    <button
                      className="text-sm text-brand"
                      type="button"
                      disabled={cancelMut.isPending}
                      onClick={() => cancelMut.mutate(o.id)}
                    >
                      {t("crud.cancel")}
                    </button>
                  ) : null}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        <FormError error={cancelMut.error} />
      </div>
    </div>
  );
}

function Stat({ label, value }: { label: string; value: string }) {
  return <div><div className="text-[10px] font-semibold uppercase tracking-wider text-slate-400">{label}</div><div className="mt-1 truncate text-sm font-medium text-slate-700" title={value}>{value}</div></div>;
}

function formatAmount(value?: string) {
  if (value === undefined || value === "") return "—";
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed.toLocaleString("ru-RU", { maximumFractionDigits: 8 }) : value;
}

function DepthRow({ level, side, max, own, onPick }: { level: BookLevel; side: "ASK" | "BID"; max: number; own: number; onPick: () => void }) {
  const ratio = Math.min(100, ((Number(level.quantity) || 0) / max) * 100);
  const ask = side === "ASK";
  return <button type="button" onClick={onPick} className="group relative grid w-full grid-cols-[1fr_1fr_1fr] overflow-hidden rounded-md px-3 py-1.5 text-xs hover:ring-1 hover:ring-inset hover:ring-blue-200"><span className={`absolute inset-y-0 right-0 ${ask ? "bg-rose-50" : "bg-emerald-50"}`} style={{ width: `${ratio}%` }}/><span className={`relative text-left font-semibold tabular-nums ${ask ? "text-rose-600" : "text-emerald-600"}`}>{level.price}</span><span className="relative text-right tabular-nums text-slate-600">{level.quantity}</span><span className="relative text-right font-medium tabular-nums text-blue-600">{own || "—"}</span></button>;
}
