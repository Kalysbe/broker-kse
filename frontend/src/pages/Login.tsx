import { useMutation } from "@tanstack/react-query";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { zodResolver } from "@hookform/resolvers/zod";
import { useTranslation } from "react-i18next";
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
    <div className="min-h-screen grid lg:grid-cols-[1.1fr_0.9fr] bg-mesh text-white">
      <div className="relative hidden lg:flex flex-col justify-between p-12 overflow-hidden">
        <div className="login-grid absolute inset-0 opacity-60" />
        <div className="login-orb absolute -left-16 top-24 h-72 w-72 rounded-full bg-brand/30 blur-3xl" />
        <div className="login-orb absolute right-10 bottom-10 h-56 w-56 rounded-full bg-brand-light/20 blur-3xl" style={{ animationDelay: "-4s" }} />
        <div className="relative">
          <div className="inline-flex items-center gap-3">
            <div className="h-11 w-11 rounded-2xl bg-gradient-to-br from-brand-light to-brand grid place-items-center font-bold text-navy">S</div>
            <div>
              <div className="tracking-[0.22em] font-semibold">{t("app.short")}</div>
              <div className="text-sm text-white/60">{t("app.name")}</div>
            </div>
          </div>
        </div>
        <div className="relative max-w-lg">
          <div className="text-brand-light text-sm tracking-[0.2em] uppercase mb-3">{t("auth.secureDesk")}</div>
          <h1 className="text-4xl font-semibold leading-tight mb-4">{t("auth.hero")}</h1>
          <p className="text-white/60 text-sm leading-relaxed">{t("auth.heroHint")}</p>
        </div>
        <div className="relative text-xs text-white/40">KSE · Depository · SBIS</div>
      </div>
      <div className="grid place-items-center p-6 bg-paper text-ink min-h-screen">
        <form
          className="surface w-full max-w-[400px] p-8"
          onSubmit={form.handleSubmit((v) => mut.mutate(v))}
        >
          <div className="lg:hidden text-brand font-semibold tracking-[0.18em] mb-1">{t("app.short")}</div>
          <h1 className="text-2xl font-semibold mb-1">{t("auth.title")}</h1>
          <p className="text-sm text-muted mb-6">{t("auth.subtitle")}</p>
          <label className="block text-sm mb-3">
            {t("auth.username")}
            <input className="mt-1 w-full border border-line px-3 py-2.5 focus:border-brand focus:ring-2 focus:ring-brand/20 outline-none" {...form.register("login")} />
          </label>
          <label className="block text-sm mb-3">
            {t("auth.password")}
            <input type="password" className="mt-1 w-full border border-line px-3 py-2.5 focus:border-brand focus:ring-2 focus:ring-brand/20 outline-none" {...form.register("password")} />
          </label>
          <label className="block text-sm mb-5">
            {t("auth.totp")}
            <input className="mt-1 w-full border border-line px-3 py-2.5 focus:border-brand focus:ring-2 focus:ring-brand/20 outline-none" {...form.register("totp")} />
          </label>
          {mut.isError && <div className="text-sm text-red-700 mb-3 bg-[#FDECEC] border border-red-200 rounded-xl px-3 py-2">{(mut.error as Error).message}</div>}
          <button className="btn-primary w-full py-2.5" disabled={mut.isPending}>{t("auth.login")}</button>
        </form>
      </div>
    </div>
  );
}
