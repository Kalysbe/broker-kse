import { useQuery } from "@tanstack/react-query";
import { api } from "../api";
import { useOrderStream } from "../useOrderStream";
import { DataGrid } from "../components/DataGrid";

export function ResourceTable({
  title,
  path,
  columns,
}: {
  title: string;
  path: string;
  columns: { key: string; label: string }[];
}) {
  const isOrders = path.split("?")[0] === "/v1/orders";
  const connected = useOrderStream(undefined, isOrders);
  const q = useQuery({ queryKey: [path], queryFn: () => api<{ items: Record<string, unknown>[] }>(path), refetchInterval: isOrders && !connected ? 30000 : false });
  const items = q.data?.items || [];
  return <div><div className="page-heading"><div><p className="eyebrow">Рабочая область</p><h1 className="page-title">{title}</h1><p className="page-subtitle">Управляйте данными, используйте поиск, сортировку и экспорт.</p></div></div><DataGrid title={title} subtitle={isOrders ? (connected ? "Живая подписка на заявки подключена" : "Переподключение к событиям · резервная сверка") : "Актуальные данные системы"} rows={items} columns={columns} loading={q.isLoading} error={q.error instanceof Error ? q.error.message : undefined} onRefresh={() => void q.refetch()} /></div>;
}
