import { useTranslation } from "react-i18next";
import { useKFBStatus } from "../ExchangeFooter";

export function KFBPage() {
  const { t } = useTranslation();
  const q = useKFBStatus();
  const d = q.data;
  const online = d?.health === "ONLINE" && d?.connected;
  return (
    <div>
      <div className="flex items-center justify-between mb-4">
        <h1 className="text-xl">{t("nav.kfb")}</h1>
        <button
          className="btn-primary"
          onClick={() => { void q.refetch(); }}
          disabled={q.isFetching}
        >
          {t("kfb.check")}
        </button>
      </div>
      <div className="grid grid-cols-4 gap-3 mb-4">
        <Card label={t("kfb.status")} value={online ? t("kfb.online") : t("kfb.offline")} accent={online} />
        <Card label={t("kfb.firm")} value={d?.firm?.firm_name || "—"} />
        <Card label={t("kfb.quikFirm")} value={d?.firm?.quik_firm || "—"} />
        <Card label={t("kfb.token")} value={d?.token_prefix || "—"} />
      </div>
      <div className="surface p-4 mb-4 text-sm">
        <div className="text-muted mb-1">{t("kfb.endpoint")}</div>
        <div>{d?.base_url || "—"}</div>
        {d?.error ? <div className="text-red-700 mt-2">{d.error}</div> : null}
      </div>
      <Section title={t("kfb.accounts")} rows={d?.accounts} cols={["account_id", "trading_account", "client_code", "currency", "status"]} />
      <div className="h-4" />
      <Section title={t("kfb.positions")} rows={d?.positions} cols={["kind", "account_id", "client_code", "symbol", "currency", "quantity", "avg_price", "available_qty", "blocked_qty"]} />
      <div className="h-4" />
      <Section title={t("kfb.instruments")} rows={d?.instruments} cols={["quik_symbol", "internal_symbol", "board", "currency", "security_type", "status"]} />
    </div>
  );
}

function Card({ label, value, accent }: { label: string; value: string; accent?: boolean }) {
  return (
    <div className="surface p-4">
      <div className="text-xs text-muted">{label}</div>
      <div className={`text-lg mt-1 ${accent ? "text-brand" : ""}`}>{value}</div>
    </div>
  );
}

function Section({ title, rows, cols }: { title: string; rows?: Record<string, unknown>[]; cols: string[] }) {
  const { t } = useTranslation();
  const items = rows || [];
  return (
    <div>
      <h2 className="text-sm mb-2">{title}</h2>
      <div className="surface overflow-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="text-left text-muted border-b border-line">
              {cols.map((c) => (
                <th key={c} className="px-3 py-2 font-medium">{c}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {items.length === 0 && (
              <tr><td className="px-3 py-6 text-muted" colSpan={cols.length}>{t("common.empty")}</td></tr>
            )}
            {items.map((row, i) => (
              <tr key={i} className="border-b border-line">
                {cols.map((c) => (
                  <td key={c} className="px-3 py-2">{String(row[c] ?? "")}</td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
