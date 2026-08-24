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

const tones = [
  "from-brand/20 to-brand/5 text-brand-dark",
  "from-navy/15 to-navy/5 text-navy",
  "from-emerald-500/15 to-emerald-500/5 text-emerald-700",
  "from-sky-500/15 to-sky-500/5 text-sky-800",
  "from-amber-400/20 to-amber-400/5 text-amber-800",
  "from-violet-500/15 to-violet-500/5 text-violet-800",
  "from-rose-500/15 to-rose-500/5 text-rose-700",
];

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
      <div className="rounded-2xl bg-mesh text-white p-6 mb-6 overflow-hidden relative">
        <div className="login-grid absolute inset-0 opacity-40" />
        <div className="relative">
          <div className="text-sm text-white/60 capitalize">{today}</div>
          <h1 className="text-2xl font-semibold mt-1">
            {t("dash.welcome", { name: user?.first_name || t("app.short") })}
          </h1>
          <p className="text-sm text-white/65 mt-2 max-w-xl">{t("dash.welcomeHint")}</p>
        </div>
      </div>
      <div className="grid grid-cols-2 xl:grid-cols-4 gap-3 mb-6">
        {cards.map(([l, v], i) => (
          <div key={l} className="surface p-4">
            <div className={`h-8 w-8 rounded-xl bg-gradient-to-br ${tones[i % tones.length]} mb-3`} />
            <div className="text-xs text-muted">{l}</div>
            <div className="text-2xl font-semibold mt-1 tabular-nums">{v}</div>
          </div>
        ))}
      </div>
      <div className="surface p-5 h-80">
        <div className="text-sm font-medium mb-3">{t("dash.ordersToday")} / {t("dash.tradesToday")}</div>
        <ResponsiveContainer width="100%" height="90%">
          <BarChart data={chart}>
            <CartesianGrid stroke="#D7E2E4" vertical={false} />
            <XAxis dataKey="name" />
            <YAxis />
            <Tooltip />
            <Bar dataKey="orders" fill="#3AA8B5" radius={[6, 6, 0, 0]} />
            <Bar dataKey="trades" fill="#0B2429" radius={[6, 6, 0, 0]} />
          </BarChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
}
