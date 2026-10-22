import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { api } from "../api";

type PositionsResponse = {
  items?: Record<string, unknown>[];
  taken_at?: unknown;
  note?: string;
  error?: string;
};

const PREFERRED_COLS = [
  "kind",
  "account_id",
  "client_code",
  "symbol",
  "board",
  "currency",
  "quantity",
  "avg_price",
  "available_qty",
  "blocked_qty",
];

function columnsFrom(rows: Record<string, unknown>[]) {
  if (rows.length === 0) return PREFERRED_COLS;
  const extra: string[] = [];
  const seen = new Set<string>();
  for (const row of rows) {
    for (const key of Object.keys(row)) {
      if (PREFERRED_COLS.includes(key)) seen.add(key);
      else if (!extra.includes(key)) extra.push(key);
    }
  }
  return [...PREFERRED_COLS.filter((key) => seen.has(key)), ...extra];
}

function Table({ rows }: { rows: Record<string, unknown>[] }) {
  const { t } = useTranslation();
  const cols = useMemo(() => columnsFrom(rows), [rows]);
  return (
    <div className="surface overflow-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="text-left text-muted border-b border-line">
            {cols.map((c) => (
              <th key={c} className="px-3 py-2 font-medium">{t(`positions.col.${c}`, { defaultValue: c })}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.length === 0 && (
            <tr><td className="px-3 py-6 text-muted" colSpan={cols.length}>{t("common.empty")}</td></tr>
          )}
          {rows.map((row, i) => (
            <tr key={i} className="border-b border-line">
              {cols.map((c) => (
                <td key={c} className="px-3 py-2">
                  {c === "kind"
                    ? t(`positions.kind.${String(row[c] || "")}`, { defaultValue: String(row[c] ?? "") })
                    : String(row[c] ?? "")}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

export function PositionsPage() {
  const { t } = useTranslation();
  const q = useQuery({
    queryKey: ["positions"],
    queryFn: () => api<PositionsResponse>("/v1/positions"),
    refetchInterval: false,
    retry: 1,
  });
  const items = q.data?.items || [];
  const cash = items.filter((row) => String(row.kind || "").toUpperCase() === "CASH");
  const securities = items.filter((row) => String(row.kind || "").toUpperCase() !== "CASH");
  const split = cash.length > 0 && securities.length > 0;

  return (
    <div>
      <div className="flex items-center justify-between mb-4">
        <h1 className="text-xl">{t("nav.positions")}</h1>
        <button
          className="btn-primary"
          onClick={() => { void q.refetch(); }}
          disabled={q.isFetching}
        >
          {t("common.refresh")}
        </button>
      </div>
      {q.data?.taken_at || q.data?.note ? (
        <div className="text-sm text-muted mb-3">
          {q.data.taken_at ? <span>{t("positions.takenAt")}: {String(q.data.taken_at)} </span> : null}
          {q.data.note ? <span>{q.data.note}</span> : null}
        </div>
      ) : null}
      {q.data?.error ? <div className="text-sm text-red-700 mb-3">{q.data.error}</div> : null}
      {split ? (
        <>
          <h2 className="text-sm mb-2">{t("positions.securities")}</h2>
          <Table rows={securities} />
          <h2 className="text-sm mb-2 mt-4">{t("positions.cash")}</h2>
          <Table rows={cash} />
        </>
      ) : (
        <Table rows={items} />
      )}
    </div>
  );
}
