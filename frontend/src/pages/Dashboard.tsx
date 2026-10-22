import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Bar, BarChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { api } from "../api";
import { useAuth } from "../auth";

type Dash = {
  clients: number;
  active_accounts: number;
  orders_today: number;
  trades_today: number;
  turnover_today: string;
  errors: number;
  active_adapters: number;
};

const tones = ["bg-blue-50 text-blue-600", "bg-indigo-50 text-indigo-600", "bg-emerald-50 text-emerald-600", "bg-cyan-50 text-cyan-600", "bg-amber-50 text-amber-600", "bg-violet-50 text-violet-600", "bg-rose-50 text-rose-600"];
const glyphs = ["↗", "⇄", "₸", "◉", "▣", "⌁", "!"];

export function DashboardPage() {
  const { t, i18n } = useTranslation();
  const { user } = useAuth();
  const q = useQuery({ queryKey: ["dash"], queryFn: () => api<Dash>("/v1/dashboard") });
  const d = q.data;
  const cards = [
    [t("dash.ordersToday"), d?.orders_today ?? "—"],
    [t("dash.tradesToday"), d?.trades_today ?? "—"],
    [t("dash.turnover"), d?.turnover_today ?? "—"],
    [t("dash.clients"), d?.clients ?? "—"],
    [t("dash.accounts"), d?.active_accounts ?? "—"],
    [t("dash.adapters"), d?.active_adapters ?? "—"],
    [t("dash.errors"), d?.errors ?? "—"],
  ] as const;
  const chart = [
    { name: "D-3", orders: 12, trades: 9 },
    { name: "D-2", orders: 18, trades: 14 },
    { name: "D-1", orders: 15, trades: 11 },
    { name: "D0", orders: d?.orders_today ?? 0, trades: d?.trades_today ?? 0 },
  ];
  const today = new Intl.DateTimeFormat(i18n.language.startsWith("en") ? "en-GB" : i18n.language.startsWith("ky") ? "ky-KG" : "ru-RU", {
    weekday: "long",
    day: "numeric",
    month: "long",
  }).format(new Date());

  return (
    <div>
      <div className="mb-6 overflow-hidden rounded-3xl border border-blue-100 bg-white p-6 shadow-[0_18px_50px_rgba(37,99,235,.08)] relative">
        <div className="absolute -right-16 -top-28 h-64 w-64 rounded-full bg-blue-100/70 blur-2xl" />
        <div className="relative flex items-center justify-between gap-6">
          <div><div className="text-sm font-medium text-blue-600 capitalize">{today}</div>
          <h1 className="text-3xl font-semibold tracking-tight text-slate-950 mt-1">
            {t("dash.welcome", { name: user?.first_name || t("app.short") })}
          </h1>
          <p className="text-sm text-slate-500 mt-2 max-w-xl">{t("dash.welcomeHint")}</p></div>
          <div className="hidden h-20 w-20 place-items-center rounded-3xl bg-blue-600 text-3xl text-white shadow-[0_16px_30px_rgba(37,99,235,.25)] sm:grid">S</div>
        </div>
      </div>
      <div className="grid grid-cols-2 xl:grid-cols-4 gap-4 mb-6">
        {cards.map(([l, v], i) => (
          <div key={l} className="surface group p-5 transition-all hover:-translate-y-0.5 hover:shadow-[0_16px_40px_rgba(15,23,42,.09)]">
            <div className="flex items-start justify-between"><div className={`grid h-10 w-10 place-items-center rounded-xl text-lg font-semibold ${tones[i % tones.length]}`}>{glyphs[i]}</div><span className="text-xs font-medium text-emerald-600">Live</span></div>
            <div className="mt-4 text-xs font-medium uppercase tracking-wide text-slate-400">{l}</div>
            <div className="text-2xl font-semibold text-slate-900 mt-1 tabular-nums">{v}</div>
          </div>
        ))}
      </div>
      <div className="surface p-6 h-96">
        <div className="mb-5 flex items-center justify-between"><div><div className="text-base font-semibold text-slate-900">Активность торгов</div><div className="mt-1 text-sm text-slate-400">{t("dash.ordersToday")} / {t("dash.tradesToday")}</div></div><span className="rounded-full bg-emerald-50 px-3 py-1 text-xs font-semibold text-emerald-700">● Данные обновляются</span></div>
        <ResponsiveContainer width="100%" height="90%">
          <BarChart data={chart}>
            <CartesianGrid stroke="#E8EDF5" vertical={false} />
            <XAxis dataKey="name" />
            <YAxis />
            <Tooltip />
            <Bar dataKey="orders" fill="#2563EB" radius={[7, 7, 0, 0]} />
            <Bar dataKey="trades" fill="#8B5CF6" radius={[7, 7, 0, 0]} />
          </BarChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
}
