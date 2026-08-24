import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { api } from "../api";
import { useAuth } from "../auth";
import { Field, FormActions, FormError, Modal, inputClass } from "../ui";

type ClientRow = {
  id: string;
  client_no: string;
  type: string;
  status: string;
  kyc_status: string;
  risk_level: string;
  email?: string | null;
  client_code?: string | null;
};

const emptyInd = {
  first_name: "", last_name: "", middle_name: "", birth_date: "", citizenship: "KG",
  document_type: "PASSPORT", document_number: "", identifier: "", address_line: "", city: "",
};
const emptyLegal = {
  legal_name: "", short_name: "", registration_number: "", inn: "", legal_address: "", director_name: "", contact_name: "",
};

export function ClientsPage() {
  const { t } = useTranslation();
  const { can } = useAuth();
  const nav = useNavigate();
  const qc = useQueryClient();
  const q = useQuery({ queryKey: ["clients"], queryFn: () => api<{ items: ClientRow[] }>("/v1/clients") });
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<ClientRow | null>(null);
  const [form, setForm] = useState({
    type: "INDIVIDUAL",
    email: "",
    phone: "",
    country: "KG",
    tax_number: "",
    client_code: "",
    status: "DRAFT",
    individual: emptyInd,
    legal: emptyLegal,
  });
  const writable = can("clients.write");
  const kyc = can("kyc.review");

  const mut = useMutation({
    mutationFn: async () => {
      const payload: Record<string, unknown> = {
        email: form.email,
        phone: form.phone,
        country: form.country,
        tax_number: form.tax_number,
        client_code: form.client_code,
      };
      if (form.type === "INDIVIDUAL") payload.individual = form.individual;
      else payload.legal = form.legal;
      if (editing) {
        return api(`/v1/clients/${editing.id}`, {
          method: "PATCH",
          body: JSON.stringify({ ...payload, status: form.status }),
        });
      }
      return api("/v1/clients", { method: "POST", body: JSON.stringify({ ...payload, type: form.type }) });
    },
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["clients"] });
      close();
    },
  });
  const kycMut = useMutation({
    mutationFn: (v: { id: string; decision: string }) =>
      api(`/v1/clients/${v.id}/kyc/review`, {
        method: "POST",
        body: JSON.stringify({ decision: v.decision, comment: "", risk_level: "MEDIUM" }),
      }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["clients"] }),
  });
  const del = useMutation({
    mutationFn: (id: string) => api(`/v1/clients/${id}`, { method: "DELETE" }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["clients"] }),
  });

  function close() {
    setOpen(false);
    setEditing(null);
  }

  async function startEdit(row: ClientRow) {
    const full = await api<Record<string, any>>(`/v1/clients/${row.id}`);
    setEditing(row);
    setForm({
      type: full.type,
      email: full.email || "",
      phone: full.phone || "",
      country: full.country || "KG",
      tax_number: full.tax_number || "",
      client_code: full.client_code || "",
      status: full.status,
      individual: { ...emptyInd, ...(full.individual || {}) },
      legal: { ...emptyLegal, ...(full.legal || {}) },
    });
    setOpen(true);
  }

  const items = q.data?.items || [];
  return (
    <div>
      <div className="flex items-center justify-between mb-4">
        <h1 className="text-xl">{t("nav.allClients")}</h1>
        {writable && (
          <button className="btn-primary" onClick={() => { setEditing(null); setForm({ type: "INDIVIDUAL", email: "", phone: "", country: "KG", tax_number: "", client_code: "", status: "DRAFT", individual: emptyInd, legal: emptyLegal }); setOpen(true); }}>
            {t("common.create")}
          </button>
        )}
      </div>
      <div className="surface overflow-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="text-left text-muted border-b border-line">
              <th className="px-3 py-2">No</th>
              <th className="px-3 py-2">{t("crud.clientCode")}</th>
              <th className="px-3 py-2">{t("crud.type")}</th>
              <th className="px-3 py-2">{t("crud.email")}</th>
              <th className="px-3 py-2">{t("common.status")}</th>
              <th className="px-3 py-2">KYC</th>
              <th className="px-3 py-2">{t("common.actions")}</th>
            </tr>
          </thead>
          <tbody>
            {items.length === 0 && <tr><td className="px-3 py-6 text-muted" colSpan={7}>{t("common.empty")}</td></tr>}
            {items.map((c) => (
              <tr key={c.id} className="border-b border-line">
                <td className="px-3 py-2">{c.client_no}</td>
                <td className="px-3 py-2">{c.client_code || ""}</td>
                <td className="px-3 py-2">{c.type}</td>
                <td className="px-3 py-2">{c.email || ""}</td>
                <td className="px-3 py-2">{c.status}</td>
                <td className="px-3 py-2">{c.kyc_status}</td>
                <td className="px-3 py-2 space-x-2 whitespace-nowrap">
                  {writable && <button className="text-brand" onClick={() => void startEdit(c)}>{t("crud.edit")}</button>}
                  {writable && can("users.write") && (
                    <button className="text-brand" onClick={() => nav(`/users?client_id=${c.id}`)}>{t("crud.createLogin")}</button>
                  )}
                  {kyc && c.kyc_status !== "APPROVED" && (
                    <button className="text-brand" onClick={() => kycMut.mutate({ id: c.id, decision: "APPROVE" })}>{t("crud.approveKyc")}</button>
                  )}
                  {kyc && c.kyc_status !== "REJECTED" && (
                    <button className="text-red-700" onClick={() => kycMut.mutate({ id: c.id, decision: "REJECT" })}>{t("crud.rejectKyc")}</button>
                  )}
                  {writable && c.status !== "CLOSED" && (
                    <button className="text-red-700" onClick={() => { if (window.confirm(t("crud.confirmDelete"))) del.mutate(c.id); }}>{t("crud.close")}</button>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {open && (
        <Modal title={editing ? t("crud.editClient") : t("crud.createClient")} onClose={close}>
          <form onSubmit={(e) => { e.preventDefault(); mut.mutate(); }}>
            <Field label={t("crud.type")}>
              <select className={inputClass} disabled={!!editing} value={form.type} onChange={(e) => setForm({ ...form, type: e.target.value })}>
                <option value="INDIVIDUAL">{t("crud.individual")}</option>
                <option value="LEGAL">{t("crud.legal")}</option>
                <option value="INSTITUTIONAL">{t("crud.institutional")}</option>
                <option value="GOVERNMENT">{t("crud.government")}</option>
              </select>
            </Field>
            <Field label={t("crud.clientCode")}>
              <input className={inputClass} required value={form.client_code} onChange={(e) => setForm({ ...form, client_code: e.target.value })} />
              <div className="text-xs text-muted mt-1">{t("crud.clientCodeHint")}</div>
            </Field>
            {editing && (
              <Field label={t("common.status")}>
                <select className={inputClass} value={form.status} onChange={(e) => setForm({ ...form, status: e.target.value })}>
                  <option>DRAFT</option><option>ACTIVE</option><option>SUSPENDED</option><option>CLOSED</option>
                </select>
              </Field>
            )}
            <div className="grid grid-cols-2 gap-3">
              <Field label={t("crud.email")}><input className={inputClass} value={form.email} onChange={(e) => setForm({ ...form, email: e.target.value })} /></Field>
              <Field label={t("crud.phone")}><input className={inputClass} value={form.phone} onChange={(e) => setForm({ ...form, phone: e.target.value })} /></Field>
            </div>
            <div className="grid grid-cols-2 gap-3">
              <Field label={t("crud.country")}><input className={inputClass} value={form.country} onChange={(e) => setForm({ ...form, country: e.target.value })} /></Field>
              <Field label={t("crud.taxNumber")}><input className={inputClass} value={form.tax_number} onChange={(e) => setForm({ ...form, tax_number: e.target.value })} /></Field>
            </div>
            {form.type === "INDIVIDUAL" ? (
              <>
                <div className="grid grid-cols-2 gap-3">
                  <Field label={t("crud.firstName")}><input className={inputClass} value={form.individual.first_name} onChange={(e) => setForm({ ...form, individual: { ...form.individual, first_name: e.target.value } })} /></Field>
                  <Field label={t("crud.lastName")}><input className={inputClass} value={form.individual.last_name} onChange={(e) => setForm({ ...form, individual: { ...form.individual, last_name: e.target.value } })} /></Field>
                </div>
                <Field label={t("crud.birthDate")}><input type="date" className={inputClass} value={form.individual.birth_date} onChange={(e) => setForm({ ...form, individual: { ...form.individual, birth_date: e.target.value } })} /></Field>
                <div className="grid grid-cols-2 gap-3">
                  <Field label={t("crud.documentType")}><input className={inputClass} value={form.individual.document_type} onChange={(e) => setForm({ ...form, individual: { ...form.individual, document_type: e.target.value } })} /></Field>
                  <Field label={t("crud.documentNumber")}><input className={inputClass} value={form.individual.document_number} onChange={(e) => setForm({ ...form, individual: { ...form.individual, document_number: e.target.value } })} /></Field>
                </div>
                <Field label={t("crud.address")}><input className={inputClass} value={form.individual.address_line} onChange={(e) => setForm({ ...form, individual: { ...form.individual, address_line: e.target.value } })} /></Field>
                <Field label={t("crud.city")}><input className={inputClass} value={form.individual.city} onChange={(e) => setForm({ ...form, individual: { ...form.individual, city: e.target.value } })} /></Field>
              </>
            ) : (
              <>
                <Field label={t("crud.legalName")}><input className={inputClass} value={form.legal.legal_name} onChange={(e) => setForm({ ...form, legal: { ...form.legal, legal_name: e.target.value } })} /></Field>
                <Field label={t("crud.regNumber")}><input className={inputClass} value={form.legal.registration_number} onChange={(e) => setForm({ ...form, legal: { ...form.legal, registration_number: e.target.value } })} /></Field>
                <Field label={t("crud.legalAddress")}><input className={inputClass} value={form.legal.legal_address} onChange={(e) => setForm({ ...form, legal: { ...form.legal, legal_address: e.target.value } })} /></Field>
                <Field label={t("crud.director")}><input className={inputClass} value={form.legal.director_name} onChange={(e) => setForm({ ...form, legal: { ...form.legal, director_name: e.target.value } })} /></Field>
              </>
            )}
            <FormError error={mut.error} />
            <FormActions onCancel={close} submitLabel={editing ? t("common.save") : t("common.create")} />
          </form>
        </Modal>
      )}
    </div>
  );
}
