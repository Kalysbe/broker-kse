import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";
import { useAuth } from "../auth";

type Profile = { environment: string; rest_url: string; grpc_target: string; token_configured: boolean };
type Settings = Profile & { profiles: Record<string, Profile>; grpc_connected: boolean; grpc_error?: string };
const names: Record<string,string> = { test: "Тестовый", production: "Боевой", custom: "Другой сервер" };

export function KFBConnectionSettings() {
  const { user } = useAuth();
  const admin = user?.roles.includes("SUPER_ADMIN") === true;
  const qc = useQueryClient();
  const query = useQuery({ queryKey:["kfb-settings"], queryFn:()=>api<Settings>("/v1/integrations/kfb/settings"), enabled:admin, refetchInterval:5000 });
  const [environment,setEnvironment] = useState("");
  const [token,setToken] = useState("");
  useEffect(()=>{ if(query.data && !environment) setEnvironment(query.data.environment); },[query.data,environment]);
  const selected = query.data?.profiles[environment];
  const save = useMutation({
    mutationFn:()=>api<Settings>("/v1/integrations/kfb/settings",{method:"PUT",body:JSON.stringify({environment,rest_url:selected?.rest_url,grpc_target:selected?.grpc_target,api_token:token})}),
    onSuccess:data=>{qc.setQueryData(["kfb-settings"],data);setToken("");void qc.invalidateQueries({queryKey:["kfb"]});void qc.invalidateQueries({queryKey:["kfb-health"]});},
  });
  if(!admin)return null;
  return <section className="surface p-5 mb-5" aria-label="Подключение к КФБ">
    <h2 className="text-lg font-semibold">Подключение к КФБ</h2>
    <p className="mt-1 text-sm text-muted">Выберите сервер и введите токен, выданный КФБ.</p>
    {query.error && <p role="alert" className="mt-3 text-red-700">{query.error instanceof Error ? query.error.message : "Не удалось загрузить настройки"}</p>}
    {query.data && selected && <form className="mt-4 space-y-4" onSubmit={event=>{event.preventDefault();save.mutate();}}>
      <p className="text-sm">Сейчас используется: <strong>{names[query.data.environment] || query.data.environment}</strong></p>
      <label className="block text-sm">Сервер КФБ<select className="input mt-1 w-full" value={environment} disabled={save.isPending} onChange={event=>{setEnvironment(event.target.value);setToken("");save.reset();}}>
        {query.data.profiles.custom && <option value="custom">Другой сервер (текущее подключение)</option>}
        <option value="test">Тестовый · локальный FIX_GateWay</option><option value="production">Боевой · 172.23.10.84</option>
      </select></label>
      <label className="block text-sm">Токен КФБ<input className="input mt-1 w-full" type="password" autoComplete="new-password" value={token} onChange={event=>{setToken(event.target.value);save.reset();}} disabled={save.isPending} required={!selected.token_configured} placeholder={selected.token_configured ? "Токен сохранён. Оставьте пустым, чтобы использовать его" : "Вставьте токен, выданный КФБ"}/></label>
      <p className="text-xs text-muted">Один токен используется для REST и gRPC. Токены серверов сохраняются отдельно.</p>
      <p className="text-xs text-muted">REST: {selected.rest_url}/api/v1 · gRPC: {selected.grpc_target}</p>
      {environment==="production" && query.data.environment!=="production" && <p className="rounded-lg bg-amber-50 p-3 text-sm text-amber-900">Выбран боевой сервер. После сохранения заявки будут направляться на него.</p>}
      {save.error && <p role="alert" className="text-sm text-red-700">{save.error instanceof Error ? save.error.message : "Не удалось сохранить подключение"}</p>}
      {save.isSuccess && <p role="status" className="text-sm text-emerald-700">Подключение сохранено. Переподключаемся к КФБ.</p>}
      <div className="flex flex-wrap items-center gap-4"><button className="btn-primary" disabled={save.isPending}>{save.isPending ? "Подключаемся…" : "Подключиться"}</button><span className="text-sm">Обновления от биржи: {query.data.grpc_connected ? "подключены" : "нет подключения"}</span></div>
      {!query.data.grpc_connected && query.data.grpc_error && <details className="text-xs text-muted"><summary>Причина отсутствия подключения</summary><p className="mt-2 break-all">{query.data.grpc_error}</p></details>}
    </form>}
  </section>;
}