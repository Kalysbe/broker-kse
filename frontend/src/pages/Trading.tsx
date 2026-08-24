import { useEffect, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useForm } from "react-hook-form";
import { useTranslation } from "react-i18next";
import { api } from "../api";
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

type PositionRow = { kind?: string; account_id?: string; client_code?: string };
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
type Book = { bids: BookLevel[]; asks: BookLevel[] };

export function TradingPage() {
  const { t } = useTranslation();
  const { can } = useAuth();
  const qc = useQueryClient();
  const [q, setQ] = useState("");
  const [selectedId, setSelectedId] = useState("");
  const writable = can("orders.submit");
  const canCancel = can("orders.cancel");

  const market = useQuery({
    queryKey: ["market"],
    queryFn: () => api<{ items: MarketItem[] }>("/v1/market"),
    refetchInterval: 10000,
  });
  const positions = useQuery({
    queryKey: ["positions"],
    queryFn: () => api<{ items: PositionRow[] }>("/v1/positions"),
    refetchInterval: 15000,
  });
  const orders = useQuery({
    queryKey: ["orders"],
    queryFn: () => api<{ items: Order[] }>("/v1/orders"),
    refetchInterval: 5000,
  });
  const book = useQuery({
    queryKey: ["book", selectedId],
    queryFn: () => api<Book>(`/v1/market/${selectedId}/book`),
    enabled: !!selectedId,
    refetchInterval: 5000,
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

  const asks = [...(book.data?.asks || [])].slice(0, 8).reverse();
  const bids = (book.data?.bids || []).slice(0, 8);

  return (
    <div className="grid grid-cols-[1.4fr_1fr] gap-4">
      <div className="surface p-4">
        <div className="flex items-center justify-between gap-3 mb-3">
          <div>
            <h2>{t("nav.market")}</h2>
            <div className="text-xs text-muted mt-0.5">{t("trade.marketHint")}</div>
          </div>
          <button className="btn-ghost text-sm" onClick={() => void market.refetch()} disabled={market.isFetching}>
            {t("common.refresh")}
          </button>
        </div>
        <input
          className="w-full border border-line px-3 py-2 mb-3 text-sm"
          placeholder={t("trade.search")}
          value={q}
          onChange={(e) => setQ(e.target.value)}
        />
        <div className="max-h-[360px] overflow-auto border border-line rounded-xl">
          <table className="w-full text-sm">
            <thead className="sticky top-0 bg-paper">
              <tr className="text-left text-muted border-b border-line">
                <th className="px-3 py-2">{t("trade.instrument")}</th>
                <th className="px-3 py-2">{t("trade.board")}</th>
                <th className="px-3 py-2 text-right">{t("trade.last")}</th>
                <th className="px-3 py-2 text-right">{t("trade.bid")}</th>
                <th className="px-3 py-2 text-right">{t("trade.ask")}</th>
              </tr>
            </thead>
            <tbody>
              {filtered.length === 0 && (
                <tr>
                  <td className="px-3 py-6 text-muted" colSpan={5}>{t("trade.noMarket")}</td>
                </tr>
              )}
              {filtered.map((i) => {
                const on = selected?.id === i.id;
                return (
                  <tr
                    key={i.id}
                    className={`border-b border-line cursor-pointer ${on ? "bg-brand/10" : "hover:bg-paper"}`}
                    onClick={() => setSelectedId(i.id)}
                  >
                    <td className="px-3 py-2">
                      <div className="font-medium">{i.ticker}</div>
                      <div className="text-xs text-muted">{i.name}</div>
                    </td>
                    <td className="px-3 py-2 text-muted">{i.board || "—"}</td>
                    <td className="px-3 py-2 text-right tabular-nums">{i.last || "—"}</td>
                    <td className="px-3 py-2 text-right tabular-nums text-emerald-700">{i.bid || "—"}</td>
                    <td className="px-3 py-2 text-right tabular-nums text-rose-700">{i.ask || "—"}</td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      </div>

      <div className="space-y-4">
        <div className="surface p-4">
          <h2 className="mb-3">{t("nav.orderBook")}</h2>
          {!selected ? (
            <div className="text-sm text-muted">{t("trade.pickInstrument")}</div>
          ) : (
            <>
              <div className="text-sm mb-3">
                <span className="font-semibold">{selected.ticker}</span>
                <span className="text-muted"> · {selected.board || "—"} · {selected.currency}</span>
              </div>
              <div className="grid grid-cols-2 gap-2 text-xs mb-3">
                <div className="border border-line rounded-xl p-2">
                  <div className="text-muted">{t("trade.bid")}</div>
                  <div className="text-lg text-emerald-700 tabular-nums">{selected.bid || "—"}</div>
                </div>
                <div className="border border-line rounded-xl p-2">
                  <div className="text-muted">{t("trade.ask")}</div>
                  <div className="text-lg text-rose-700 tabular-nums">{selected.ask || "—"}</div>
                </div>
              </div>
              <div className="grid grid-cols-2 gap-3 text-xs">
                <div>
                  <div className="text-muted mb-1">{t("trade.asks")}</div>
                  {asks.length === 0 && <div className="text-muted">{t("common.empty")}</div>}
                  {asks.map((lvl, i) => (
                    <div key={`a${i}`} className="flex justify-between py-0.5 text-rose-800">
                      <span className="tabular-nums">{lvl.price}</span>
                      <span className="tabular-nums">{lvl.quantity}</span>
                    </div>
                  ))}
                </div>
                <div>
                  <div className="text-muted mb-1">{t("trade.bids")}</div>
                  {bids.length === 0 && <div className="text-muted">{t("common.empty")}</div>}
                  {bids.map((lvl, i) => (
                    <div key={`b${i}`} className="flex justify-between py-0.5 text-emerald-800">
                      <span className="tabular-nums">{lvl.price}</span>
                      <span className="tabular-nums">{lvl.quantity}</span>
                    </div>
                  ))}
                </div>
              </div>
            </>
          )}
        </div>

        <form className="surface p-4" onSubmit={form.handleSubmit((v) => mut.mutate(v))}>
          <h2 className="mb-3">{t("trade.submit")}</h2>
          <div className="text-sm mb-3">
            <span className="text-muted">{t("trade.account")}: </span>
            <span className="font-medium tabular-nums">{venueAccount || t("trade.accountAuto")}</span>
          </div>
          <div className="grid grid-cols-2 gap-2 mb-2">
            <button
              type="button"
              className={`py-2 rounded-xl text-white ${side === "BUY" ? "bg-brand" : "bg-navy/40"}`}
              onClick={() => form.setValue("side", "BUY")}
            >
              {t("trade.buy")}
            </button>
            <button
              type="button"
              className={`py-2 rounded-xl text-white ${side === "SELL" ? "bg-navy" : "bg-navy/40"}`}
              onClick={() => form.setValue("side", "SELL")}
            >
              {t("trade.sell")}
            </button>
          </div>
          <label className="block text-sm mb-2">{t("trade.type")}
            <select className="w-full border border-line px-2 py-2 mt-1" {...form.register("order_type")}>
              <option value="LIMIT">LIMIT</option>
              <option value="MARKET">MARKET</option>
            </select>
          </label>
          <label className="block text-sm mb-2">{t("trade.tif")}
            <select className="w-full border border-line px-2 py-2 mt-1" {...form.register("time_in_force")}>
              <option>DAY</option>
              <option>IOC</option>
              <option>FOK</option>
              <option>GTC</option>
            </select>
          </label>
          <label className="block text-sm mb-2">{t("trade.qty")}
            <input className="w-full border border-line px-2 py-2 mt-1" {...form.register("quantity")} />
          </label>
          {ot !== "MARKET" && (
            <label className="block text-sm mb-2">{t("trade.price")}
              <input className="w-full border border-line px-2 py-2 mt-1" {...form.register("price")} />
            </label>
          )}
          <div className="text-sm text-muted mb-3">
            {t("trade.preview")}: {Number.isFinite(est) ? est.toFixed(2) : "—"}
            {selected?.currency ? ` ${selected.currency}` : ""}
          </div>
          <FormError error={mut.error} />
          <button className="btn-primary w-full py-2.5" disabled={!writable || mut.isPending || !selected}>
            {t("trade.submit")}
          </button>
        </form>
      </div>

      <div className="col-span-2 surface p-4">
        <h2 className="mb-2">{t("nav.orders")}</h2>
        <table className="w-full text-sm">
          <thead>
            <tr className="text-left text-muted border-b border-line">
              <th className="py-2">{t("trade.instrument")}</th>
              <th>Side</th>
              <th>{t("trade.qty")}</th>
              <th>{t("trade.price")}</th>
              <th>{t("common.status")}</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {(orders.data?.items || []).length === 0 && (
              <tr><td className="py-6 text-muted" colSpan={6}>{t("common.empty")}</td></tr>
            )}
            {(orders.data?.items || []).map((o) => (
              <tr key={o.id} className="border-b border-line">
                <td className="py-2">{o.symbol || o.client_order_id}</td>
                <td className={o.side === "BUY" ? "text-emerald-700" : "text-rose-700"}>{o.side}</td>
                <td>{o.quantity}{o.filled_quantity ? ` / ${o.filled_quantity}` : ""}</td>
                <td>{o.price || "—"}</td>
                <td>
                  {o.status}
                  {o.reject_reason ? (
                    <span className="text-xs text-red-700 ml-1">
                      {t(`errors.${o.reject_reason}`, { defaultValue: o.reject_reason })}
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
