import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { api } from "../api";

type Depo = {
  connected: boolean;
  health: string;
  name: string;
  note: string;
};

export function DepositoryPage() {
  const { t } = useTranslation();
  const q = useQuery({ queryKey: ["depository"], queryFn: () => api<Depo>("/v1/integrations/depository") });
  return (
    <div>
      <h1 className="text-xl mb-4">{t("nav.depository")}</h1>
      <div className="surface p-5 max-w-2xl">
        <div className="text-xs text-muted">{t("kfb.status")}</div>
        <div className="text-lg mt-1">{q.data?.connected ? t("kfb.online") : t("kfb.offline")}</div>
        <p className="text-sm text-muted mt-3">{q.data?.note || t("depository.pending")}</p>
      </div>
    </div>
  );
}
