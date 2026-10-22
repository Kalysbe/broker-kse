import { useQuery } from "@tanstack/react-query";
import { api } from "../api";
import { useAuth } from "../auth";
import { DataGrid, DataGridColumn } from "../components/DataGrid";
import { SideBadge } from "../components/StatusBadge";

type Trade = Record<string, unknown> & { id: string; exec_id: string; client_code: string; symbol: string; board: string; side: string; quantity: string; price: string; gross_amount: string; quik_order_id: string; created_at: string };
type Response = { items: Trade[]; source?: string; live?: boolean };

export function TradesPage() {
  const { can } = useAuth();
  const q = useQuery({ queryKey: ["trades"], queryFn: () => api<Response>("/v1/trades"), refetchInterval: 5000 });
  const admin = can("clients.read");
  const columns: DataGridColumn<Trade>[] = [
    { key: "created_at", label: "Время сделки", render: (r) => r.created_at ? new Date(r.created_at).toLocaleString("ru-RU") : "—" },
    { key: "symbol", label: "Инструмент", render: (r) => <div><div className="font-semibold text-slate-900">{r.symbol}</div><div className="text-xs text-slate-400">{r.board}</div></div> },
  ];
  if (admin) columns.push({ key: "client_code", label: "Клиент" });
  columns.push(
    { key: "side", label: "Операция", render: (r) => <SideBadge side={r.side}/> },
    { key: "quantity", label: "Количество", align: "right" }, { key: "price", label: "Цена", align: "right" },
    { key: "gross_amount", label: "Сумма", align: "right", render: (r) => <span className="font-semibold tabular-nums text-slate-900">{r.gross_amount || "—"}</span> },
    { key: "quik_order_id", label: "Номер на бирже", render: (r) => <span className="font-mono text-xs text-slate-500">{r.quik_order_id || "—"}</span> },
  );
  return <div><div className="page-heading"><div><p className="eyebrow">Исполненные операции</p><h1 className="page-title">Сделки</h1><p className="page-subtitle">{admin ? "Все сделки клиентов, полученные напрямую из торгового шлюза." : "Ваши исполненные сделки на фондовой бирже."}</p></div><div className={`rounded-full px-3 py-1.5 text-xs font-semibold ${q.data?.live ? "bg-emerald-50 text-emerald-700" : "bg-amber-50 text-amber-700"}`}>{q.data?.live ? "● LIVE · КФБ" : "Локальные данные"}</div></div><DataGrid title={admin ? "Все биржевые сделки" : "Мои биржевые сделки"} subtitle="Обновление каждые 5 секунд" rows={q.data?.items || []} columns={columns} loading={q.isLoading} error={q.error instanceof Error ? q.error.message : undefined} onRefresh={() => void q.refetch()} /></div>;
}
