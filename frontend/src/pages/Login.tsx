import { useMutation } from "@tanstack/react-query";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { zodResolver } from "@hookform/resolvers/zod";
import { useTranslation } from "react-i18next";
import { Link } from "react-router-dom";
import { api, setAccessToken } from "../api";

const schema = z.object({
  login: z.string().min(1),
  password: z.string().min(1),
  totp: z.string().optional(),
});

type LoginForm = z.infer<typeof schema>;

export function LoginPage() {
  const { t } = useTranslation();
  const form = useForm<LoginForm>({ resolver: zodResolver(schema) });
  const mut = useMutation({
    mutationFn: (v: z.infer<typeof schema>) =>
      api<{ access_token: string }>("/v1/auth/login", { method: "POST", body: JSON.stringify(v) }),
    onSuccess: (d) => {
      setAccessToken(d.access_token);
      window.location.href = "/";
    },
  });
  return (
    <div className="min-h-screen grid lg:grid-cols-[1.05fr_0.95fr] bg-[#f6f8fc] text-slate-900">
      <div className="relative hidden lg:flex flex-col justify-between p-12 overflow-hidden">
        <div className="login-grid absolute inset-0 opacity-40" />
        <div className="login-orb absolute -left-16 top-24 h-72 w-72 rounded-full bg-blue-200/60 blur-3xl" />
        <div className="login-orb absolute right-10 bottom-10 h-56 w-56 rounded-full bg-violet-200/40 blur-3xl" style={{ animationDelay: "-4s" }} />
        <div className="relative">
          <div className="inline-flex items-center gap-3">
            <div className="h-11 w-11 rounded-xl bg-blue-600 grid place-items-center font-bold text-white shadow-[0_10px_25px_rgba(37,99,235,.25)]">S</div>
            <div>
              <div className="tracking-[0.22em] font-semibold">{t("app.short")}</div>
              <div className="text-sm text-slate-500">{t("app.name")}</div>
            </div>
          </div>
        </div>
        <div className="relative max-w-lg">
          <div className="text-blue-600 text-sm font-semibold tracking-[0.2em] uppercase mb-3">{t("auth.secureDesk")}</div>
          <h1 className="text-5xl font-semibold tracking-tight leading-tight mb-4 text-slate-950">{t("auth.hero")}</h1>
          <p className="text-slate-500 text-base leading-relaxed">{t("auth.heroHint")}</p>
        </div>
        <div className="relative text-xs font-medium text-slate-400">KSE · Depository · SBIS</div>
      </div>
      <div className="relative grid place-items-center border-l border-slate-200 bg-white p-6 pt-20 min-h-screen">
        <Link className="login-home-link" to="/welcome">← О КФБ Инвест</Link>
        <form
          className="w-full max-w-[420px] rounded-3xl border border-slate-200 bg-white p-9 shadow-[0_24px_70px_rgba(15,23,42,.10)]"
          onSubmit={form.handleSubmit((v) => mut.mutate(v))}
        >
          <div className="lg:hidden text-blue-600 font-semibold tracking-[0.18em] mb-1">{t("app.short")}</div>
          <h1 className="text-3xl font-semibold tracking-tight mb-1">{t("auth.title")}</h1>
          <p className="text-sm text-slate-500 mb-7">{t("auth.subtitle")}</p>
          <label className="block text-sm mb-3">
            {t("auth.username")}
            <input className="mt-1 w-full border border-slate-200 bg-slate-50 px-3 py-3 focus:border-blue-400 focus:bg-white focus:ring-4 focus:ring-blue-100 outline-none transition" {...form.register("login")} />
          </label>
          <label className="block text-sm mb-3">
            {t("auth.password")}
            <input type="password" className="mt-1 w-full border border-slate-200 bg-slate-50 px-3 py-3 focus:border-blue-400 focus:bg-white focus:ring-4 focus:ring-blue-100 outline-none transition" {...form.register("password")} />
          </label>
          <label className="block text-sm mb-5">
            {t("auth.totp")}
            <input className="mt-1 w-full border border-slate-200 bg-slate-50 px-3 py-3 focus:border-blue-400 focus:bg-white focus:ring-4 focus:ring-blue-100 outline-none transition" {...form.register("totp")} />
          </label>
          {mut.isError && <div className="text-sm text-red-700 mb-3 bg-[#FDECEC] border border-red-200 rounded-xl px-3 py-2">{(mut.error as Error).message}</div>}
          <button className="w-full rounded-xl bg-blue-600 py-3 font-semibold text-white shadow-[0_10px_20px_rgba(37,99,235,.2)] transition hover:bg-blue-700 disabled:opacity-60" disabled={mut.isPending}>{t("auth.login")}</button>
        </form>
      </div>
    </div>
  );
}
