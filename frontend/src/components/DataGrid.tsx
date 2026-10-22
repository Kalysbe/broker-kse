import { ReactNode, useMemo, useState } from "react";
import { SideBadge, StatusBadge } from "./StatusBadge";

export type DataGridColumn<T> = {
  key: keyof T | string;
  label: string;
  render?: (row: T) => ReactNode;
  sortable?: boolean;
  align?: "left" | "center" | "right";
};

type Props<T extends Record<string, unknown>> = {
  rows: T[];
  columns: DataGridColumn<T>[];
  loading?: boolean;
  error?: string;
  rowKey?: (row: T, index: number) => string;
  onRefresh?: () => void;
  title?: string;
  subtitle?: string;
  actions?: ReactNode;
};

function valueAt<T extends Record<string, unknown>>(row: T, key: string) {
  return row[key];
}

function csvCell(value: unknown) {
  return `"${String(value ?? "").replaceAll('"', '""')}"`;
}

function defaultCell<T extends Record<string, unknown>>(row: T, key: string) {
  const value = valueAt(row, key);
  if (key === "status") return <StatusBadge status={value} />;
  if (key === "side") return <SideBadge side={value} />;
  return String(value ?? "—");
}

export function DataGrid<T extends Record<string, unknown>>({
  rows, columns, loading, error, rowKey, onRefresh, title, subtitle, actions,
}: Props<T>) {
  const [search, setSearch] = useState("");
  const [sort, setSort] = useState<{ key: string; direction: "asc" | "desc" } | null>(null);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [compact, setCompact] = useState(false);

  const keyedRows = useMemo(() => rows.map((row, index) => ({ row, id: rowKey?.(row, index) ?? String(row.id ?? index) })), [rows, rowKey]);
  const visible = useMemo(() => {
    const needle = search.trim().toLocaleLowerCase("ru");
    const filtered = needle ? keyedRows.filter(({ row }) => columns.some((column) => String(valueAt(row, String(column.key)) ?? "").toLocaleLowerCase("ru").includes(needle))) : keyedRows;
    if (!sort) return filtered;
    return [...filtered].sort((a, b) => {
      const av = valueAt(a.row, sort.key);
      const bv = valueAt(b.row, sort.key);
      const result = String(av ?? "").localeCompare(String(bv ?? ""), "ru", { numeric: true, sensitivity: "base" });
      return sort.direction === "asc" ? result : -result;
    });
  }, [columns, keyedRows, search, sort]);

  const pages = Math.max(1, Math.ceil(visible.length / pageSize));
  const currentPage = Math.min(page, pages);
  const pageRows = visible.slice((currentPage - 1) * pageSize, currentPage * pageSize);
  const allOnPage = pageRows.length > 0 && pageRows.every(({ id }) => selected.has(id));

  function toggleSort(key: string) {
    setSort((old) => old?.key === key ? { key, direction: old.direction === "asc" ? "desc" : "asc" } : { key, direction: "asc" });
  }

  function exportCsv() {
    const header = columns.map((c) => csvCell(c.label)).join(",");
    const body = visible.map(({ row }) => columns.map((c) => csvCell(valueAt(row, String(c.key)))).join(",")).join("\n");
    const blob = new Blob(["\uFEFF", header, "\n", body], { type: "text/csv;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = `${title || "export"}.csv`;
    link.click();
    URL.revokeObjectURL(url);
  }

  return (
    <section className="data-grid">
      {(title || actions) && (
        <div className="flex flex-col gap-3 border-b border-slate-200 px-5 py-4 lg:flex-row lg:items-center lg:justify-between">
          <div><h2 className="text-base font-semibold text-slate-900">{title}</h2>{subtitle && <p className="mt-0.5 text-sm text-slate-500">{subtitle}</p>}</div>
          {actions && <div className="flex flex-wrap gap-2">{actions}</div>}
        </div>
      )}
      <div className="flex flex-wrap items-center gap-2 border-b border-slate-200 bg-slate-50/70 px-4 py-3">
        <label className="relative min-w-[220px] flex-1 sm:max-w-sm">
          <span className="pointer-events-none absolute left-3 top-2.5 text-slate-400">⌕</span>
          <input value={search} onChange={(e) => { setSearch(e.target.value); setPage(1); }} className="grid-search" placeholder="Поиск по таблице…" />
        </label>
        <span className="rounded-lg bg-white px-2.5 py-2 text-xs font-medium text-slate-500 ring-1 ring-slate-200">{visible.length} записей</span>
        {selected.size > 0 && <span className="rounded-lg bg-blue-50 px-2.5 py-2 text-xs font-semibold text-blue-700">Выбрано: {selected.size}</span>}
        <div className="ml-auto flex items-center gap-2">
          <button className="grid-tool" onClick={() => setCompact((v) => !v)} title="Плотность строк">{compact ? "▤" : "☷"}</button>
          <button className="grid-tool" onClick={exportCsv} title="Экспорт CSV">⇩ <span className="hidden sm:inline">CSV</span></button>
          {onRefresh && <button className="grid-tool" onClick={onRefresh} title="Обновить">↻</button>}
        </div>
      </div>
      <div className="overflow-auto">
        <table className="w-full min-w-[720px] text-sm">
          <thead className="sticky top-0 z-10 bg-white/95 backdrop-blur">
            <tr className="border-b border-slate-200 text-left text-xs uppercase tracking-wide text-slate-500">
              <th className="w-12 px-4 py-3"><input type="checkbox" checked={allOnPage} onChange={() => setSelected((old) => { const next = new Set(old); pageRows.forEach(({ id }) => allOnPage ? next.delete(id) : next.add(id)); return next; })} /></th>
              {columns.map((column) => <th key={String(column.key)} className={`px-3 py-3 font-semibold text-${column.align || "left"}`}><button className={column.sortable === false ? "cursor-default" : "hover:text-blue-700"} onClick={() => column.sortable !== false && toggleSort(String(column.key))}>{column.label} {sort?.key === column.key && <span className="text-blue-600">{sort.direction === "asc" ? "↑" : "↓"}</span>}</button></th>)}
            </tr>
          </thead>
          <tbody>
            {loading && Array.from({ length: 6 }).map((_, i) => <tr key={i} className="border-b border-slate-100"><td colSpan={columns.length + 1} className="px-4 py-3"><div className="h-5 animate-pulse rounded bg-slate-100" /></td></tr>)}
            {!loading && error && <tr><td colSpan={columns.length + 1} className="px-6 py-12 text-center text-rose-600">Не удалось загрузить данные: {error}</td></tr>}
            {!loading && !error && pageRows.length === 0 && <tr><td colSpan={columns.length + 1} className="px-6 py-16 text-center"><div className="text-3xl text-slate-300">◎</div><div className="mt-2 font-medium text-slate-700">Ничего не найдено</div><div className="mt-1 text-sm text-slate-400">Измените поисковый запрос или обновите данные</div></td></tr>}
            {!loading && !error && pageRows.map(({ row, id }) => <tr key={id} className={`border-b border-slate-100 transition-colors hover:bg-blue-50/40 ${selected.has(id) ? "bg-blue-50/60" : ""}`}>
              <td className={`${compact ? "py-2" : "py-3.5"} px-4`}><input type="checkbox" checked={selected.has(id)} onChange={() => setSelected((old) => { const next = new Set(old); next.has(id) ? next.delete(id) : next.add(id); return next; })} /></td>
              {columns.map((column) => <td key={String(column.key)} className={`${compact ? "py-2" : "py-3.5"} px-3 text-${column.align || "left"} text-slate-700`}>{column.render ? column.render(row) : defaultCell(row, String(column.key))}</td>)}
            </tr>)}
          </tbody>
        </table>
      </div>
      <div className="flex flex-wrap items-center justify-between gap-3 border-t border-slate-200 px-4 py-3 text-sm text-slate-500">
        <label>Показывать <select className="ml-1 rounded-lg border-slate-200 py-1 pl-2 pr-7" value={pageSize} onChange={(e) => { setPageSize(Number(e.target.value)); setPage(1); }}>{[10,25,50].map((n) => <option key={n}>{n}</option>)}</select></label>
        <div className="flex items-center gap-2"><button className="grid-page" disabled={currentPage === 1} onClick={() => setPage((p) => p - 1)}>←</button><span>Страница <b className="text-slate-800">{currentPage}</b> из {pages}</span><button className="grid-page" disabled={currentPage === pages} onClick={() => setPage((p) => p + 1)}>→</button></div>
      </div>
    </section>
  );
}
