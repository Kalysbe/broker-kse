import { useTranslation } from "react-i18next";
import { KFBConnectionSettings } from "../components/KFBConnectionSettings";
import { useKFBStatus } from "../ExchangeFooter";

export function KFBPage() {
  const { t } = useTranslation();
  const q = useKFBStatus();
  const d = q.data;
  const online = d?.health === "ONLINE" && d?.connected;
  const configured = Boolean(d?.base_url && d?.token_prefix);
  const issue = connectionIssue(d?.error, Boolean(d?.base_url), Boolean(d?.token_prefix), q.isError);
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
      <KFBConnectionSettings />
      {!online && !q.isLoading && (
        <div className="surface border-red-200 bg-red-50 p-5 mb-4" role="alert">
          <div className="flex gap-3">
            <div className="mt-1 h-3 w-3 shrink-0 rounded-full bg-red-500" aria-hidden />
            <div className="min-w-0">
              <h2 className="font-semibold text-red-900">{t(`kfb.issue.${issue}.title`)}</h2>
              <p className="mt-1 text-sm text-red-800">{t(`kfb.issue.${issue}.description`)}</p>
              {issue === "notConfigured" && (
                <div className="mt-4 text-sm text-red-950">
                  <div>Администратор может изменить адреса и установить сертификаты в настройках подключения выше. Токен доступа выдаёт администратор биржи.</div>
                </div>
              )}
              {d?.error && (
                <details className="mt-3 text-xs text-red-700">
                  <summary className="cursor-pointer font-medium">{t("kfb.technicalDetails")}</summary>
                  <div className="mt-1 font-mono break-all">{d.error}</div>
                </details>
              )}
            </div>
          </div>
        </div>
      )}
      <div className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-4 gap-3 mb-4">
        <Card label={t("kfb.status")} value={q.isFetching ? t("kfb.checking") : online ? t("kfb.online") : configured ? t("kfb.unavailable") : t("kfb.notConfigured")} accent={online} />
        <Card label={t("kfb.firm")} value={d?.firm?.firm_name || t("kfb.noData")} />
        <Card label={t("kfb.quikFirm")} value={d?.firm?.quik_firm || t("kfb.noData")} />
        <Card label={t("kfb.token")} value={d?.token_prefix || t("kfb.notSpecified")} />
      </div>
      <div className="surface p-4 mb-4 text-sm">
        <div className="text-muted mb-1">{t("kfb.endpoint")}</div>
        <div>{d?.base_url || t("kfb.notSpecified")}</div>
        {d?.last_check_at ? <div className="text-muted mt-2">{t("kfb.lastCheck")}: {new Date(d.last_check_at).toLocaleString()}</div> : null}
      </div>
      <Section title={t("kfb.accounts")} rows={d?.accounts} cols={["account_id", "trading_account", "client_code", "currency", "status"]} connected={online} />
      <div className="h-4" />
      <Section title={t("kfb.positions")} rows={d?.positions} cols={["kind", "account_id", "client_code", "symbol", "currency", "quantity", "avg_price", "available_qty", "blocked_qty"]} connected={online} />
      <div className="h-4" />
      <Section title={t("kfb.instruments")} rows={d?.instruments} cols={["quik_symbol", "internal_symbol", "board", "currency", "security_type", "status"]} connected={online} />
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

function Section({ title, rows, cols, connected }: { title: string; rows?: Record<string, unknown>[]; cols: string[]; connected: boolean }) {
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
                <th key={c} className="px-3 py-2 font-medium">{t(`kfb.col.${c}`)}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {items.length === 0 && (
              <tr><td className="px-3 py-6 text-muted" colSpan={cols.length}>{connected ? t("kfb.emptyConnected") : t("kfb.emptyDisconnected")}</td></tr>
            )}
            {items.map((row, i) => (
              <tr key={i} className="border-b border-line">
                {cols.map((c) => (
                  <td key={c} className="px-3 py-2">{formatCell(c, row[c], t)}</td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function connectionIssue(error: string | undefined, hasUrl: boolean, hasToken: boolean, queryFailed: boolean) {
  if (queryFailed) return "apiUnavailable";
  if (!hasUrl || !hasToken || /not configured/i.test(error || "")) return "notConfigured";
  if (/401|403|unauthor|forbidden|token/i.test(error || "")) return "authorization";
  if (/timeout|deadline/i.test(error || "")) return "timeout";
  return "gatewayUnavailable";
}

function formatCell(key: string, value: unknown, t: (key: string, options?: Record<string, unknown>) => string) {
  if (value == null || value === "") return "—";
  const raw = String(value);
  if (key === "status" || key === "kind" || key === "security_type") {
    const translated = t(`kfb.value.${raw}`, { defaultValue: raw });
    return translated;
  }
  return raw;
}
