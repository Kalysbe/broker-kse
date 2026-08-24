import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { api } from "../api";
import { useAuth } from "../auth";
import { Field, FormActions, FormError, Modal, inputClass } from "../ui";

type Row = { id: string; contract_number: string; client_id: string; client_no?: string; status: string; start_date: string };
type Client = { id: string; client_no: string };
type Tariff = { id: string; code: string; name: string };

export function ContractsPage() {
  const { t } = useTranslation();
  const { can } = useAuth();
  const qc = useQueryClient();
  const q = useQuery({ queryKey: ["contracts"], queryFn: () => api<{ items: Row[] }>("/v1/contracts") });
  const clients = useQuery({ queryKey: ["clients"], queryFn: () => api<{ items: Client[] }>("/v1/clients") });
  const tariffs = useQuery({ queryKey: ["tariffs"], queryFn: () => api<{ items: Tariff[] }>("/v1/tariffs") });
  const [open, setOpen] = useState(false);
  const [form, setForm] = useState({ client_id: "", tariff_code: "STANDARD", start_date: new Date().toISOString().slice(0, 10) });
  const writable = can("contracts.write");

  const mut = useMutation({
    mutationFn: () => api("/v1/contracts", { method: "POST", body: JSON.stringify(form) }),
    onSuccess: () => { void qc.invalidateQueries({ queryKey: ["contracts"] }); setOpen(false); },
  });
  const patch = useMutation({
    mutationFn: (v: { id: string; status: string }) =>
      api(`/v1/contracts/${v.id}`, { method: "PATCH", body: JSON.stringify({ status: v.status }) }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["contracts"] }),
  });

  const items = q.data?.items || [];
  return (
    <div>
      <div className="flex items-center justify-between mb-4">
        <h1 className="text-xl">{t("nav.contracts")}</h1>
        {writable && <button className="btn-primary" onClick={() => setOpen(true)}>{t("common.create")}</button>}
      </div>
      <div className="surface overflow-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="text-left text-muted border-b border-line">
              <th className="px-3 py-2">{t("crud.number")}</th>
              <th className="px-3 py-2">{t("crud.client")}</th>
              <th className="px-3 py-2">{t("common.status")}</th>
              <th className="px-3 py-2">{t("crud.startDate")}</th>
              {writable && <th className="px-3 py-2">{t("common.actions")}</th>}
            </tr>
          </thead>
          <tbody>
            {items.length === 0 && <tr><td className="px-3 py-6 text-muted" colSpan={5}>{t("common.empty")}</td></tr>}
            {items.map((r) => (
              <tr key={r.id} className="border-b border-line">
                <td className="px-3 py-2">{r.contract_number}</td>
                <td className="px-3 py-2">{r.client_no || r.client_id}</td>
                <td className="px-3 py-2">{r.status}</td>
                <td className="px-3 py-2">{r.start_date}</td>
                {writable && (
                  <td className="px-3 py-2 space-x-2">
                    {r.status !== "CLOSED" && <button className="text-red-700" onClick={() => patch.mutate({ id: r.id, status: "CLOSED" })}>{t("crud.close")}</button>}
                    {r.status !== "ACTIVE" && <button className="text-brand" onClick={() => patch.mutate({ id: r.id, status: "ACTIVE" })}>{t("crud.activate")}</button>}
                  </td>
                )}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {open && (
        <Modal title={t("crud.createContract")} onClose={() => setOpen(false)}>
          <form onSubmit={(e) => { e.preventDefault(); mut.mutate(); }}>
            <Field label={t("crud.client")}>
              <select className={inputClass} value={form.client_id} onChange={(e) => setForm({ ...form, client_id: e.target.value })}>
                <option value="">—</option>
                {(clients.data?.items || []).map((c) => <option key={c.id} value={c.id}>{c.client_no}</option>)}
              </select>
            </Field>
            <Field label={t("crud.tariff")}>
              <select className={inputClass} value={form.tariff_code} onChange={(e) => setForm({ ...form, tariff_code: e.target.value })}>
                {(tariffs.data?.items?.length ? tariffs.data.items : [{ code: "STANDARD", name: "Standard" }]).map((x) => <option key={x.code} value={x.code}>{x.code} · {x.name}</option>)}
              </select>
            </Field>
            <Field label={t("crud.startDate")}>
              <input type="date" className={inputClass} value={form.start_date} onChange={(e) => setForm({ ...form, start_date: e.target.value })} />
            </Field>
            <FormError error={mut.error} />
            <FormActions onCancel={() => setOpen(false)} submitLabel={t("common.create")} />
          </form>
        </Modal>
      )}
    </div>
  );
}
