import { useQuery } from "@tanstack/react-query";
import { api } from "../api";

export function ResourceTable({
  title,
  path,
  columns,
}: {
  title: string;
  path: string;
  columns: { key: string; label: string }[];
}) {
  const q = useQuery({ queryKey: [path], queryFn: () => api<{ items: Record<string, unknown>[] }>(path) });
  const items = q.data?.items || [];
  return (
    <div>
      <h1 className="page-title mb-4">{title}</h1>
      <div className="surface overflow-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="text-left text-muted border-b border-line bg-paper/70">
              {columns.map((c) => (
                <th key={c.key} className="px-3 py-2 font-medium">{c.label}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {items.length === 0 && (
              <tr><td className="px-3 py-6 text-muted" colSpan={columns.length}>—</td></tr>
            )}
            {items.map((row, i) => (
              <tr key={String(row.id ?? i)} className="border-b border-line hover:bg-paper/60">
                {columns.map((c) => (
                  <td key={c.key} className="px-3 py-2">{String(row[c.key] ?? "")}</td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
