import { NavLink, Outlet, useLocation } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useAuth } from "./auth";
import { ExchangeFooter } from "./ExchangeFooter";
import { useSocketCache } from "./useSocketCache";
import { DataSources } from "./components/DataSources";

const groups: { label: string; items: { to: string; key: string; perm?: string; icon: string }[] }[] = [
  { label: "dashboard", items: [{ to: "/", key: "dashboard", icon: "grid" }] },
  {
    label: "trading",
    items: [
      { to: "/trading", key: "market", perm: "orders.read", icon: "pulse" },
      { to: "/orders", key: "orders", perm: "orders.read", icon: "list" },
      { to: "/trades", key: "trades", perm: "trades.read", icon: "swap" },
      { to: "/order-book", key: "orderBook", perm: "orders.read", icon: "layers" },
    ],
  },
  {
    label: "clients",
    items: [
      { to: "/clients", key: "allClients", perm: "clients.read", icon: "users" },
      { to: "/kyc", key: "kyc", perm: "kyc.read", icon: "shield" },
      { to: "/documents", key: "documents", perm: "documents.read", icon: "file" },
      { to: "/contracts", key: "contracts", perm: "contracts.read", icon: "doc" },
    ],
  },
  {
    label: "accounts",
    items: [
      { to: "/accounts", key: "tradingAccounts", perm: "accounts.read", icon: "wallet" },
      { to: "/cash", key: "cashAccounts", perm: "cash.read", icon: "coins" },
      { to: "/positions", key: "positions", perm: "portfolio.read", icon: "chart" },
    ],
  },
  { label: "portfolio", items: [{ to: "/portfolio", key: "portfolio", perm: "portfolio.read", icon: "pie" }] },
  { label: "settlement", items: [{ to: "/settlement", key: "settlement", perm: "settlement.read", icon: "check" }] },
  {
    label: "payments",
    items: [
      { to: "/deposits", key: "deposits", perm: "cash.read", icon: "down" },
      { to: "/withdrawals", key: "withdrawals", perm: "cash.read", icon: "up" },
    ],
  },
  {
    label: "securities",
    items: [
      { to: "/instruments", key: "instruments", perm: "instruments.read", icon: "tag" },
      { to: "/issuers", key: "issuers", perm: "instruments.read", icon: "building" },
      { to: "/corporate-actions", key: "corporateActions", perm: "instruments.read", icon: "spark" },
    ],
  },
  { label: "reports", items: [{ to: "/reports", key: "reports", perm: "reports.run", icon: "report" }] },
  {
    label: "integrations",
    items: [
      { to: "/integrations/kfb", key: "kfb", perm: "adapters.read", icon: "plug" },
      { to: "/integrations/depository", key: "depository", perm: "adapters.read", icon: "safe" },
    ],
  },
  { label: "brokers", items: [{ to: "/brokers", key: "brokers", perm: "brokers.read", icon: "broker" }] },
  {
    label: "administration",
    items: [
      { to: "/users", key: "users", perm: "users.read", icon: "users" },
      { to: "/roles", key: "roles", perm: "roles.read", icon: "key" },
      { to: "/tokens", key: "apiTokens", perm: "tokens.write", icon: "token" },
      { to: "/settings", key: "settings", perm: "admin.settings", icon: "cog" },
    ],
  },
  {
    label: "monitoring",
    items: [
      { to: "/logs", key: "logs", perm: "logs.read", icon: "log" },
      { to: "/audit", key: "audit", perm: "audit.read", icon: "eye" },
      { to: "/notifications", key: "notifications", perm: "notifications.manage", icon: "bell" },
      { to: "/health", key: "health", perm: "adapters.read", icon: "heart" },
      { to: "/load-testing", key: "loadTesting", perm: "adapters.operate", icon: "pulse" },
    ],
  },
];

function initials(first?: string, last?: string, login?: string) {
  const a = (first || "").trim().charAt(0);
  const b = (last || "").trim().charAt(0);
  const s = `${a}${b}`.toUpperCase();
  return s || (login || "?").slice(0, 2).toUpperCase();
}

export function Shell() {
  useSocketCache();
  const { t, i18n } = useTranslation();
  const { user, can, logout } = useAuth();
  const loc = useLocation();
  const onProfile = loc.pathname === "/profile";

  return (
    <>
      <div className="grid h-[calc(100vh-2rem)] grid-cols-[248px_1fr] overflow-hidden bg-[#f6f8fc]">
        <aside className="relative flex min-h-0 flex-col overflow-hidden border-r border-slate-200 bg-white text-slate-600">
          <div className="relative px-5 py-5 border-b border-slate-100">
            <div className="flex items-center gap-3">
              <div className="h-10 w-10 rounded-xl bg-blue-600 grid place-items-center font-bold text-white shadow-[0_8px_20px_rgba(37,99,235,.25)]">
                S
              </div>
              <div>
                <div className="text-slate-950 font-bold tracking-[0.14em] text-sm">{t("app.short")}</div>
                <div className="text-[11px] text-slate-400 mt-0.5 leading-tight">{t("app.name")}</div>
              </div>
            </div>
          </div>
          <nav
            className="relative min-h-0 flex-1 overflow-y-auto py-4 sidebar-scroll"
            onWheel={(event) => event.stopPropagation()}
          >
            {groups.map((g) => {
              const items = g.items.filter((it) => !user || !it.perm || can(it.perm));
              if (items.length === 0) return null;
              return (
                <div key={g.label} className="mb-4">
                  <div className="px-5 text-[10px] font-semibold uppercase tracking-[0.16em] text-slate-400 mb-1.5">
                    {t(`nav.${g.label}`)}
                  </div>
                  {items.map((it) => (
                    <NavLink
                      key={it.to}
                      to={it.to}
                      end={it.to === "/"}
                      className={({ isActive }) =>
                        `mx-2 flex items-center gap-2.5 rounded-xl px-3 py-2 text-sm font-medium transition-all ${
                          isActive
                            ? "bg-blue-50 text-blue-700 shadow-[inset_3px_0_0_0_#2563eb]"
                            : "text-slate-500 hover:bg-slate-50 hover:text-slate-900"
                        }`
                      }
                    >
                      <NavIcon name={it.icon} />
                      {t(`nav.${it.key}`)}
                    </NavLink>
                  ))}
                </div>
              );
            })}
          </nav>
          {user && (
            <NavLink
              to="/profile"
              className={`relative m-3 mt-0 shrink-0 rounded-2xl border p-3 flex items-center gap-3 transition-colors ${
                onProfile ? "border-blue-200 bg-blue-50 text-blue-800" : "border-slate-200 bg-white hover:bg-slate-50 text-slate-700"
              }`}
            >
              <div className="h-9 w-9 rounded-xl bg-blue-600 grid place-items-center text-white font-semibold text-sm shrink-0">
                {initials(user.first_name, user.last_name, user.login)}
              </div>
              <div className="min-w-0">
                <div className="text-sm font-medium truncate">
                  {user.first_name} {user.last_name}
                </div>
                <div className="text-[11px] text-slate-400 truncate">{t("nav.profile")}</div>
              </div>
            </NavLink>
          )}
        </aside>
        <div className="flex min-h-0 min-w-0 flex-col overflow-y-auto app-scroll">
          <header className="h-[72px] bg-white/90 backdrop-blur border-b border-slate-200 flex items-center justify-between gap-6 px-6 sticky top-0 z-20">
            <div className="min-w-0">
              <div className="text-[10px] font-semibold uppercase tracking-[0.14em] text-blue-600">
                {user?.broker?.name || t("app.name")}
              </div>
              <div className="text-sm font-medium text-slate-800 truncate">
                {user ? `${user.first_name} ${user.last_name}` : ""}
                {user?.roles?.[0] ? <span className="text-slate-400"> · {user.roles[0]}</span> : null}
              </div>
            </div>
            <label className="relative hidden max-w-md flex-1 lg:block">
              <span className="absolute left-3 top-2.5 text-slate-400">⌕</span>
              <input className="w-full rounded-xl border border-slate-200 bg-slate-50 py-2 pl-9 pr-3 text-sm outline-none transition focus:border-blue-400 focus:bg-white focus:ring-4 focus:ring-blue-100" placeholder="Быстрый поиск по системе…" />
            </label>
            <div className="flex gap-2 items-center">
              <div className="flex rounded-xl border border-slate-200 overflow-hidden text-xs">
                {(["ky", "ru", "en"] as const).map((lang) => (
                  <button
                    key={lang}
                    type="button"
                    className={`px-2.5 py-1.5 uppercase ${
                      i18n.language.startsWith(lang) ? "bg-slate-900 text-white" : "bg-white text-slate-400 hover:bg-slate-50"
                    }`}
                    onClick={() => {
                      void i18n.changeLanguage(lang);
                      localStorage.setItem("lang", lang);
                    }}
                  >
                    {lang}
                  </button>
                ))}
              </div>
              <NavLink
                to="/profile"
                className="h-9 w-9 rounded-xl bg-blue-600 text-white grid place-items-center text-xs font-semibold shadow-[0_5px_15px_rgba(37,99,235,.2)]"
                title={t("nav.profile")}
              >
                {user ? initials(user.first_name, user.last_name, user.login) : "?"}
              </NavLink>
              <button className="btn-ghost" onClick={() => logout()}>
                {t("auth.logout")}
              </button>
            </div>
          </header>
          <main className="p-6 lg:p-8 max-w-[1800px] w-full mx-auto">
            <DataSources /><Outlet />
          </main>
        </div>
      </div>
      <ExchangeFooter />
    </>
  );
}

function NavIcon({ name }: { name: string }) {
  const common = "w-4 h-4 shrink-0 opacity-80";
  const p = {
    fill: "none",
    stroke: "currentColor",
    strokeWidth: 1.8,
    strokeLinecap: "round" as const,
    strokeLinejoin: "round" as const,
    className: common,
    viewBox: "0 0 24 24",
  };
  switch (name) {
    case "grid":
      return (
        <svg {...p}>
          <rect x="3" y="3" width="7" height="7" rx="1.5" />
          <rect x="14" y="3" width="7" height="7" rx="1.5" />
          <rect x="3" y="14" width="7" height="7" rx="1.5" />
          <rect x="14" y="14" width="7" height="7" rx="1.5" />
        </svg>
      );
    case "pulse":
      return (
        <svg {...p}>
          <path d="M3 12h4l2-6 4 12 2-6h6" />
        </svg>
      );
    case "list":
      return (
        <svg {...p}>
          <path d="M8 6h13M8 12h13M8 18h13M3 6h.01M3 12h.01M3 18h.01" />
        </svg>
      );
    case "swap":
      return (
        <svg {...p}>
          <path d="M7 7h13l-3-3M17 17H4l3 3" />
        </svg>
      );
    case "layers":
      return (
        <svg {...p}>
          <path d="M12 3 3 8l9 5 9-5-9-5ZM3 14l9 5 9-5" />
        </svg>
      );
    case "users":
      return (
        <svg {...p}>
          <path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2" />
          <circle cx="9" cy="7" r="4" />
          <path d="M22 21v-2a4 4 0 0 0-3-3.87M16 3.13a4 4 0 0 1 0 7.75" />
        </svg>
      );
    case "shield":
      return (
        <svg {...p}>
          <path d="M12 3 5 6v6c0 5 3.5 8 7 9 3.5-1 7-4 7-9V6l-7-3Z" />
        </svg>
      );
    case "file":
    case "doc":
    case "report":
      return (
        <svg {...p}>
          <path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8Z" />
          <path d="M14 3v5h5M9 13h6M9 17h4" />
        </svg>
      );
    case "wallet":
    case "coins":
      return (
        <svg {...p}>
          <rect x="3" y="6" width="18" height="14" rx="2" />
          <path d="M3 10h18M16 14h2" />
        </svg>
      );
    case "chart":
    case "pie":
      return (
        <svg {...p}>
          <path d="M4 19V9M10 19V5M16 19v-7M22 19H2" />
        </svg>
      );
    case "check":
      return (
        <svg {...p}>
          <path d="M20 6 9 17l-5-5" />
        </svg>
      );
    case "down":
      return (
        <svg {...p}>
          <path d="M12 5v14M5 12l7 7 7-7" />
        </svg>
      );
    case "up":
      return (
        <svg {...p}>
          <path d="M12 19V5M5 12l7-7 7 7" />
        </svg>
      );
    case "tag":
      return (
        <svg {...p}>
          <path d="M20.6 13.4 12 22l-9-9 8.6-8.6A2 2 0 0 1 13 4h6v6a2 2 0 0 1-.4 1.4Z" />
          <circle cx="16" cy="8" r="1" />
        </svg>
      );
    case "building":
    case "broker":
      return (
        <svg {...p}>
          <path d="M4 21V5a2 2 0 0 1 2-2h7v18M13 9h7v12M8 8h2M8 12h2M8 16h2M16 13h2M16 17h2" />
        </svg>
      );
    case "spark":
      return (
        <svg {...p}>
          <path d="M12 3v4M12 17v4M4.9 6.5l2.8 2.8M16.3 14.7l2.8 2.8M3 12h4M17 12h4M4.9 17.5l2.8-2.8M16.3 9.3l2.8-2.8" />
        </svg>
      );
    case "plug":
      return (
        <svg {...p}>
          <path d="M12 22v-5M8 8V3M16 8V3M7 8h10v4a5 5 0 0 1-10 0Z" />
        </svg>
      );
    case "safe":
      return (
        <svg {...p}>
          <rect x="3" y="4" width="18" height="16" rx="2" />
          <circle cx="12" cy="12" r="3" />
        </svg>
      );
    case "key":
    case "token":
      return (
        <svg {...p}>
          <circle cx="8" cy="15" r="4" />
          <path d="M11.7 12.3 21 3M16 3h5v5" />
        </svg>
      );
    case "cog":
      return (
        <svg {...p}>
          <circle cx="12" cy="12" r="3" />
          <path d="M12 3v2M12 19v2M5 5l1.5 1.5M17.5 17.5 19 19M3 12h2M19 12h2M5 19l1.5-1.5M17.5 6.5 19 5" />
        </svg>
      );
    case "log":
    case "eye":
      return (
        <svg {...p}>
          <path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12Z" />
          <circle cx="12" cy="12" r="3" />
        </svg>
      );
    case "bell":
      return (
        <svg {...p}>
          <path d="M6 8a6 6 0 1 1 12 0c0 7 3 7 3 9H3c0-2 3-2 3-9M10 21h4" />
        </svg>
      );
    case "heart":
      return (
        <svg {...p}>
          <path d="M20.8 8.6a4.6 4.6 0 0 0-7.8-3.3L12 6.3l-1-1a4.6 4.6 0 0 0-7.8 3.3c0 5.4 8.8 10.4 8.8 10.4s8.8-5 8.8-10.4Z" />
        </svg>
      );
    default:
      return (
        <svg {...p}>
          <circle cx="12" cy="12" r="8" />
        </svg>
      );
  }
}
