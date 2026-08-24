import { useState } from "react";
import { useMutation } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { api } from "../api";
import { useAuth } from "../auth";
import { Field, FormError, inputClass } from "../ui";

function initials(first?: string, last?: string, login?: string) {
  const a = (first || "").trim().charAt(0);
  const b = (last || "").trim().charAt(0);
  const s = `${a}${b}`.toUpperCase();
  return s || (login || "?").slice(0, 2).toUpperCase();
}

function Row({ label, value }: { label: string; value?: string | null }) {
  return (
    <div className="flex items-start justify-between gap-4 py-3 border-b border-line last:border-0">
      <div className="text-sm text-muted shrink-0">{label}</div>
      <div className="text-sm text-ink text-right break-all">{value || "—"}</div>
    </div>
  );
}

export function ProfilePage() {
  const { t, i18n } = useTranslation();
  const { user } = useAuth();
  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const [confirm, setConfirm] = useState("");
  const [localError, setLocalError] = useState("");
  const [ok, setOk] = useState(false);

  const mut = useMutation({
    mutationFn: () =>
      api("/v1/auth/change-password", {
        method: "POST",
        body: JSON.stringify({ current_password: current, new_password: next }),
      }),
    onSuccess: () => {
      setCurrent("");
      setNext("");
      setConfirm("");
      setLocalError("");
      setOk(true);
    },
    onError: () => setOk(false),
  });

  const submit = () => {
    setOk(false);
    if (!current || !next) {
      setLocalError(t("profile.fillAll"));
      return;
    }
    if (next !== confirm) {
      setLocalError(t("profile.mismatch"));
      return;
    }
    setLocalError("");
    mut.mutate();
  };

  const locale = i18n.language.startsWith("en") ? "en-GB" : i18n.language.startsWith("ky") ? "ky-KG" : "ru-RU";
  const lastLogin = user?.last_login_at
    ? new Intl.DateTimeFormat(locale, { dateStyle: "medium", timeStyle: "short" }).format(new Date(user.last_login_at))
    : "";

  return (
    <div className="max-w-5xl">
      <h1 className="page-title mb-5">{t("nav.profile")}</h1>
      <div className="surface p-5 mb-5 flex items-center gap-4 bg-mesh text-white relative overflow-hidden">
        <div className="login-grid absolute inset-0 opacity-30" />
        <div className="relative h-16 w-16 rounded-2xl bg-white text-navy grid place-items-center text-xl font-semibold shrink-0">
          {initials(user?.first_name, user?.last_name, user?.login)}
        </div>
        <div className="relative min-w-0">
          <div className="text-xl font-semibold truncate">
            {user?.first_name} {user?.last_name}
          </div>
          <div className="text-sm text-white/70 truncate">
            {user?.login}
            {user?.email ? ` · ${user.email}` : ""}
          </div>
          <div className="mt-2 flex flex-wrap gap-1.5">
            {(user?.roles || []).map((r) => (
              <span key={r} className="text-[11px] px-2 py-0.5 rounded-full bg-white/15">
                {r}
              </span>
            ))}
          </div>
        </div>
      </div>

      <div className="grid lg:grid-cols-2 gap-4">
        <section className="surface p-5">
          <h2 className="font-medium mb-1">{t("profile.personal")}</h2>
          <p className="text-xs text-muted mb-2">{t("profile.personalHint")}</p>
          <Row label={t("auth.username")} value={user?.login} />
          <Row label={t("crud.firstName")} value={user?.first_name} />
          <Row label={t("crud.lastName")} value={user?.last_name} />
          <Row label={t("crud.email")} value={user?.email} />
          <Row label={t("crud.phone")} value={user?.phone} />
          <Row label={t("common.status")} value={user?.status} />
          <Row label={t("profile.twoFactor")} value={user?.totp_enabled ? t("profile.on") : t("profile.off")} />
          <Row label={t("profile.lastLogin")} value={lastLogin} />
          <Row label={t("profile.lastIp")} value={user?.last_login_ip} />
        </section>

        <section className="surface p-5">
          <h2 className="font-medium mb-1">{t("profile.broker")}</h2>
          <p className="text-xs text-muted mb-2">{t("profile.brokerHint")}</p>
          <Row label={t("profile.brokerName")} value={user?.broker?.name} />
          <Row label={t("profile.brokerCode")} value={user?.broker?.code} />
          <Row label={t("common.status")} value={user?.broker?.status} />
          <Row label={t("profile.license")} value={user?.broker?.license_no} />
          <Row label={t("kfb.firm")} value={user?.broker?.firm_name} />
          <Row label={t("kfb.quikFirm")} value={user?.broker?.quik_firm} />
          <Row label={t("profile.roles")} value={(user?.roles || []).join(", ")} />
        </section>

        <section className="surface p-5 lg:col-span-2">
          <h2 className="font-medium mb-1">{t("profile.resetTitle")}</h2>
          <p className="text-sm text-muted mb-4">{t("profile.resetHint")}</p>
          <form
            className="grid sm:grid-cols-3 gap-3 max-w-3xl"
            onSubmit={(e) => {
              e.preventDefault();
              submit();
            }}
          >
            <Field label={t("profile.currentPassword")}>
              <input type="password" className={inputClass} value={current} onChange={(e) => setCurrent(e.target.value)} autoComplete="current-password" />
            </Field>
            <Field label={t("profile.newPassword")}>
              <input type="password" className={inputClass} value={next} onChange={(e) => setNext(e.target.value)} autoComplete="new-password" />
            </Field>
            <Field label={t("profile.confirmPassword")}>
              <input type="password" className={inputClass} value={confirm} onChange={(e) => setConfirm(e.target.value)} autoComplete="new-password" />
            </Field>
            <div className="sm:col-span-3">
              {localError ? <div className="text-sm text-red-800 bg-[#FDECEC] border border-red-200 rounded-xl px-3 py-2 mb-3">{localError}</div> : null}
              <FormError error={mut.error} />
              {ok ? <div className="text-sm text-emerald-800 bg-emerald-50 border border-emerald-200 rounded-xl px-3 py-2 mb-3">{t("profile.passwordChanged")}</div> : null}
              <button type="submit" className="btn-primary" disabled={mut.isPending}>
                {t("profile.resetAction")}
              </button>
            </div>
          </form>
        </section>
      </div>
    </div>
  );
}
