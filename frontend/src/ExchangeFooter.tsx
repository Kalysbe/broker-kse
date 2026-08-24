import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { api } from "./api";

export type KFBHealth = {
  connected: boolean;
  health: string;
  error?: string;
  last_check_at?: string;
};

export type KFBStatus = KFBHealth & {
  base_url: string;
  token_prefix: string;
  ready: boolean;
  firm: { quik_firm?: string; firm_name?: string };
  instruments: Record<string, unknown>[];
  accounts: Record<string, unknown>[];
  positions: Record<string, unknown>[];
};

export function useKFBHealth() {
  return useQuery({
    queryKey: ["kfb-health"],
    queryFn: () => api<KFBHealth>("/v1/integrations/kfb/health"),
    refetchInterval: 15000,
    retry: 1,
  });
}

export function useKFBStatus() {
  return useQuery({
    queryKey: ["kfb"],
    queryFn: () => api<KFBStatus>("/v1/integrations/kfb?refresh=1"),
    refetchInterval: 15000,
    retry: 1,
  });
}

function formatStamp(lang: string, d: Date) {
  const locale = lang.startsWith("en") ? "en-GB" : lang.startsWith("ky") ? "ky-KG" : "ru-RU";
  return new Intl.DateTimeFormat(locale, {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  }).format(d);
}

export function ExchangeFooter() {
  const { t, i18n } = useTranslation();
  const q = useKFBHealth();
  const [now, setNow] = useState(() => new Date());

  useEffect(() => {
    const id = window.setInterval(() => setNow(new Date()), 1000);
    return () => window.clearInterval(id);
  }, []);

  const d = q.data;
  const online = Boolean(d?.health === "ONLINE" && d?.connected);
  const checking = q.isFetching && !d;
  const stamp = formatStamp(i18n.language, now);

  let label = t("kfb.footerOffline");
  let tone: "online" | "offline" | "checking" = "offline";
  if (checking) {
    label = t("kfb.footerChecking");
    tone = "checking";
  } else if (online) {
    label = t("kfb.footerOnline");
    tone = "online";
  }

  return (
    <footer className="fixed bottom-0 inset-x-0 z-50 h-8 bg-navy text-white flex items-center justify-between px-4 text-xs tracking-wide select-none">
      <div className="flex items-center gap-2 min-w-0">
        <span className={`status-dot status-dot-${tone}`} aria-hidden />
        <span className={`truncate ${tone === "online" ? "text-[#22c55e]" : ""}`}>{label}</span>
        {d?.error && !online ? <span className="text-red-300 truncate hidden sm:inline">· {d.error}</span> : null}
      </div>
      <time
        className={`tabular-nums shrink-0 ml-4 ${tone === "online" ? "text-[#22c55e] status-time-online" : "text-white/90"}`}
        dateTime={now.toISOString()}
      >
        {stamp}
      </time>
    </footer>
  );
}
