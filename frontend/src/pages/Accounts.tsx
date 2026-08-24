import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { api } from "../api";
import { useAuth } from "../auth";
import { Field, FormActions, FormError, Modal, inputClass } from "../ui";

type Row = {
  id: string;
  account_number: string;
  client_id: string;
  client_no?: string;
  contract_id: string;
  currency: string;
  status: string;
};
type Client = { id: string; client_no: string };
type Contract = { id: string; contract_number: string; client_id: string; status: string };

export function AccountsPage() {
  const { t } = useTranslation();
  const { can } = useAuth();
  const qc = useQueryClient();
  const q = useQuery({ queryKey: ["accounts"], queryFn: () => api<{ items: Row[] }>("/v1/accounts") });
  const clients = useQuery({ queryKey: ["clients"], queryFn: () => api<{ items: Client[] }>("/v1/clients") });
  const contracts = useQuery({ queryKey: ["contracts"], queryFn: () => api<{ items: Contract[] }>("/v1/contracts") });
  const [open, setOpen] = useState(false);
  const [form, setForm] = useState({ client_id: "", contract_id: "", currency: "KGS" });
  const writable = can("accounts.write");
  const filteredContracts = useMemo(
    () => (contracts.data?.items || []).filter((c) => !form.client_id || c.client_id === form.client_id),
    [contracts.data, form.client_id],
  );

  const mut = useMutation({
    mutationFn: () => api("/v1/accounts", { method: "POST", body: JSON.stringify(form) }),
    onSuccess: () => { void qc.invalidateQueries({ queryKey: ["accounts"] }); setOpen(false); },
  });
  const patch = useMutation({
    mutationFn: (v: { id: string; status: string }) =>
      api(`/v1/accounts/${v.id}`, { method: "PATCH", body: JSON.stringify({ status: v.status }) }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["accounts"] }),
  });

  const items = q.data?.items || [];
  return (
    <div>
      <div className="flex items-center justify-between mb-4">
        <h1 className="text-xl">{t("nav.tradingAccounts")}</h1>
        {writable && <button className="btn-primary" onClick={() => setOpen(true)}>{t("common.create")}</button>}
      </div>
      <div className="surface overflow-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="text-left text-muted border-b border-line">
              <th className="px-3 py-2">{t("crud.number")}</th>
              <th className="px-3 py-2">{t("crud.client")}</th>
              <th className="px-3 py-2">{t("crud.currency")}</th>
              <th className="px-3 py-2">{t("common.status")}</th>
              {writable && <th className="px-3 py-2">{t("common.actions")}</th>}
            </tr>
          </thead>
          <tbody>
            {items.length === 0 && <tr><td className="px-3 py-6 text-muted" colSpan={5}>{t("common.empty")}</td></tr>}
            {items.map((r) => (
              <tr key={r.id} className="border-b border-line">
                <td className="px-3 py-2">{r.account_number}</td>
                <td className="px-3 py-2">{r.client_no || r.client_id}</td>
                <td className="px-3 py-2">{r.currency}</td>
                <td className="px-3 py-2">{r.status}</td>
                {writable && (
                  <td className="px-3 py-2 space-x-2 whitespace-nowrap">
                    {r.status !== "ACTIVE" && <button className="text-brand" onClick={() => patch.mutate({ id: r.id, status: "ACTIVE" })}>{t("crud.activate")}</button>}
                    {r.status === "ACTIVE" && <button className="text-red-700" onClick={() => patch.mutate({ id: r.id, status: "BLOCKED" })}>{t("crud.block")}</button>}
                    {r.status !== "CLOSED" && <button className="text-red-700" onClick={() => patch.mutate({ id: r.id, status: "CLOSED" })}>{t("crud.close")}</button>}
                  </td>
                )}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {open && (
        <Modal title={t("crud.createAccount")} onClose={() => setOpen(false)}>
          <form onSubmit={(e) => { e.preventDefault(); mut.mutate(); }}>
            <Field label={t("crud.client")}>
              <select className={inputClass} value={form.client_id} onChange={(e) => setForm({ ...form, client_id: e.target.value, contract_id: "" })}>
                <option value="">—</option>
                {(clients.data?.items || []).map((c) => <option key={c.id} value={c.id}>{c.client_no}</option>)}
              </select>
            </Field>
            <Field label={t("crud.contract")}>
              <select className={inputClass} value={form.contract_id} onChange={(e) => setForm({ ...form, contract_id: e.target.value })}>
                <option value="">—</option>
                {filteredContracts.map((c) => <option key={c.id} value={c.id}>{c.contract_number}</option>)}
              </select>
            </Field>
            <Field label={t("crud.currency")}>
              <select className={inputClass} value={form.currency} onChange={(e) => setForm({ ...form, currency: e.target.value })}>
                <option>KGS</option><option>KZT</option><option>USD</option><option>EUR</option><option>RUB</option>
              </select>
            </Field>
            <FormError error={mut.error} />
            <FormActions onCancel={() => setOpen(false)} submitLabel={t("common.create")} />
          </form>
        </Modal>
      )}
    </div>
  );
}
