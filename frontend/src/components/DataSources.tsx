import { useSyncExternalStore } from "react";
import { useLocation } from "react-router-dom";
import { pageSources, sourceVersion, subscribeSources } from "../dataSources";
import { pageEndpoints } from "../pageEndpoints";

export function DataSources() {
  const location = useLocation();
  useSyncExternalStore(subscribeSources, sourceVersion);
  const sources = pageSources(location.pathname);
  const declared = location.pathname.startsWith("/load-testing/orders/") ? ["/v1/load-tests/orders/{id}"] : pageEndpoints[location.pathname] || [];
  const cached = declared.filter(endpoint => {
    const parts = endpoint.split(/\{[^}]+\}/);
    return !sources.some(source => source.label.startsWith(`GET ${parts[0]}`) && source.label.endsWith(parts[parts.length - 1]));
  });
  const reads = sources.filter(source => source.kind === "Подписка" || source.label.startsWith("GET "));
  const commands = sources.filter(source => source.kind !== "Подписка" && !source.label.startsWith("GET "));
  return <aside className="mb-5 rounded-xl border border-slate-200 bg-white px-4 py-3 text-xs text-slate-600" aria-label="Источники данных">
    <div className="font-semibold text-slate-900">Источники данных этой страницы</div>
    <p className="mt-1 text-slate-400">Источники страницы, включая кэш и общие панели. WebSocket передаёт запросы и обновления данных.</p>
    <div className="mt-2 flex flex-wrap gap-2">{reads.map(source => <div key={source.key} className="rounded-lg bg-slate-50 px-3 py-2"><span className="font-semibold">{source.kind}</span> · <code className="break-all">{source.label}</code><span className="ml-2 text-slate-500">{source.state}</span></div>)}{!reads.length && <span>Источник появится при загрузке данных.</span>}</div>
    {!!cached.length && <div className="mt-2 flex flex-wrap gap-2">{cached.map(endpoint => <span key={endpoint} className="rounded-lg bg-slate-50 px-3 py-2"><code>GET {endpoint}</code> · кэш / при загрузке выбранных данных</span>)}</div>}
    {!!commands.length && <details className="mt-2"><summary className="cursor-pointer">Запросы действий</summary>{commands.map(source => <div key={source.key} className="mt-1"><code>{source.label}</code> · {source.state}</div>)}</details>}
  </aside>;
}
