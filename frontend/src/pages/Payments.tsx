import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";
import { useAuth } from "../auth";
import { DataGrid, DataGridColumn } from "../components/DataGrid";

type Payment = Record<string, unknown> & { id: string; account_id: string; amount: string; currency: string; status: string; created_at: string };

export function PaymentsPage({ kind }: { kind: "deposits" | "withdrawals" }) {
  const { can } = useAuth(); const qc = useQueryClient();
  const q = useQuery({ queryKey: [kind], queryFn: () => api<{ items: Payment[] }>(`/v1/${kind}`) });
  const approve = useMutation({ mutationFn: (id: string) => api(`/v1/${kind}/${id}/approve`, { method: "POST" }), onSuccess: () => void qc.invalidateQueries({ queryKey: [kind] }) });
  const columns: DataGridColumn<Payment>[] = [
    { key: "created_at", label: "Дата", render: (r) => new Date(r.created_at).toLocaleString("ru-RU") },
    { key: "account_id", label: "Счёт" }, { key: "amount", label: "Сумма", align: "right", render: (r) => `${r.amount} ${r.currency}` },
    { key: "status", label: "Статус", render: (r) => <span className={`rounded-full px-2.5 py-1 text-xs font-semibold ${r.status === "COMPLETED" ? "bg-emerald-50 text-emerald-700" : "bg-amber-50 text-amber-700"}`}>{r.status === "COMPLETED" ? "Завершено" : "Ожидает подтверждения"}</span> },
  ];
  if (can("payments.approve")) columns.push({ key: "actions", label: "Действия", sortable: false, render: (r) => r.status === "REQUESTED" ? <button className="text-sm font-semibold text-blue-600 hover:text-blue-800" onClick={() => approve.mutate(r.id)}>Подтвердить</button> : "—" });
  const title = kind === "deposits" ? "Пополнения" : "Выводы";
  return <div><div className="page-heading"><div><p className="eyebrow">Денежные операции</p><h1 className="page-title">{title}</h1><p className="page-subtitle">Контроль заявок и подтверждённых движений средств.</p></div></div><DataGrid title={title} rows={q.data?.items || []} columns={columns} loading={q.isLoading} onRefresh={() => void q.refetch()} /></div>;
}
