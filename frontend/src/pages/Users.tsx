import { useEffect, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useSearchParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { api } from "../api";
import { useAuth } from "../auth";
import { Field, FormActions, FormError, Modal, inputClass } from "../ui";

type UserRow = {
  id: string;
  login: string;
  email: string;
  phone?: string | null;
  first_name: string;
  last_name: string;
  status: string;
  roles: string[];
  client_id?: string | null;
  client_no?: string | null;
};

type Role = { code: string; name: string };
type Client = { id: string; client_no: string; email?: string };

const emptyForm = {
  login: "",
  email: "",
  phone: "",
  first_name: "",
  last_name: "",
  password: "",
  status: "ACTIVE",
  client_id: "",
  roles: [] as string[],
};

export function UsersPage() {
  const { t } = useTranslation();
  const { can } = useAuth();
  const qc = useQueryClient();
  const [params, setParams] = useSearchParams();
  const preClient = params.get("client_id") || "";
  const users = useQuery({ queryKey: ["users"], queryFn: () => api<{ items: UserRow[] }>("/v1/users") });
  const roles = useQuery({ queryKey: ["roles"], queryFn: () => api<{ items: Role[] }>("/v1/roles") });
  const clients = useQuery({ queryKey: ["clients"], queryFn: () => api<{ items: Client[] }>("/v1/clients") });
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<UserRow | null>(null);
  const [form, setForm] = useState(emptyForm);
  const writable = can("users.write");

  useEffect(() => {
    if (preClient) {
      setEditing(null);
      setForm({ ...emptyForm, client_id: preClient, roles: ["CLIENT"] });
      setOpen(true);
    }
  }, [preClient]);

  const mut = useMutation({
    mutationFn: async () => {
      const body: Record<string, unknown> = {
        email: form.email,
        phone: form.phone,
        first_name: form.first_name,
        last_name: form.last_name,
        password: form.password,
        client_id: form.client_id,
        roles: form.roles,
      };
      if (editing) {
        return api(`/v1/users/${editing.id}`, { method: "PATCH", body: JSON.stringify({ ...body, status: form.status }) });
      }
      return api("/v1/users", { method: "POST", body: JSON.stringify({ ...body, login: form.login }) });
    },
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["users"] });
      close();
    },
  });
  const del = useMutation({
    mutationFn: (id: string) => api(`/v1/users/${id}`, { method: "DELETE" }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["users"] }),
  });

  function close() {
    setOpen(false);
    setEditing(null);
    setForm(emptyForm);
    if (preClient) setParams({});
  }

  function startCreate() {
    setEditing(null);
    setForm(emptyForm);
    setOpen(true);
  }

  function startEdit(u: UserRow) {
    setEditing(u);
    setForm({
      login: u.login,
      email: u.email,
      phone: u.phone || "",
      first_name: u.first_name,
      last_name: u.last_name,
      password: "",
      status: u.status,
      client_id: u.client_id || "",
      roles: u.roles || [],
    });
    setOpen(true);
  }

  function toggleRole(code: string) {
    setForm((f) => ({
      ...f,
      roles: f.roles.includes(code) ? f.roles.filter((x) => x !== code) : [...f.roles, code],
    }));
  }

  const items = users.data?.items || [];
  const title = useMemo(() => (editing ? t("crud.editUser") : t("crud.createUser")), [editing, t]);

  return (
    <div>
      <div className="flex items-center justify-between mb-4">
        <h1 className="text-xl">{t("nav.users")}</h1>
        {writable && (
          <button className="btn-primary" onClick={startCreate}>
            {t("common.create")}
          </button>
        )}
      </div>
      {users.isError && <FormError error={users.error} />}
      <div className="surface overflow-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="text-left text-muted border-b border-line">
              <th className="px-3 py-2">{t("auth.username")}</th>
              <th className="px-3 py-2">{t("crud.email")}</th>
              <th className="px-3 py-2">{t("crud.name")}</th>
              <th className="px-3 py-2">{t("crud.roles")}</th>
              <th className="px-3 py-2">{t("crud.client")}</th>
              <th className="px-3 py-2">{t("common.status")}</th>
              {writable && <th className="px-3 py-2">{t("common.actions")}</th>}
            </tr>
          </thead>
          <tbody>
            {items.length === 0 && (
              <tr><td className="px-3 py-6 text-muted" colSpan={7}>{t("common.empty")}</td></tr>
            )}
            {items.map((u) => (
              <tr key={u.id} className="border-b border-line">
                <td className="px-3 py-2">{u.login}</td>
                <td className="px-3 py-2">{u.email}</td>
                <td className="px-3 py-2">{u.first_name} {u.last_name}</td>
                <td className="px-3 py-2">{(u.roles || []).join(", ")}</td>
                <td className="px-3 py-2">{u.client_no || "—"}</td>
                <td className="px-3 py-2">{u.status}</td>
                {writable && (
                  <td className="px-3 py-2 space-x-2 whitespace-nowrap">
                    <button className="text-brand" onClick={() => startEdit(u)}>{t("crud.edit")}</button>
                    <button
                      className="text-red-700"
                      onClick={() => {
                        if (window.confirm(t("crud.confirmDelete"))) del.mutate(u.id);
                      }}
                    >
                      {t("crud.delete")}
                    </button>
                  </td>
                )}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {open && (
        <Modal title={title} onClose={close}>
          <form onSubmit={(e) => { e.preventDefault(); mut.mutate(); }}>
            <Field label={t("auth.username")}>
              <input className={inputClass} value={form.login} disabled={!!editing} onChange={(e) => setForm({ ...form, login: e.target.value })} />
            </Field>
            <div className="grid grid-cols-2 gap-3">
              <Field label={t("crud.firstName")}>
                <input className={inputClass} value={form.first_name} onChange={(e) => setForm({ ...form, first_name: e.target.value })} />
              </Field>
              <Field label={t("crud.lastName")}>
                <input className={inputClass} value={form.last_name} onChange={(e) => setForm({ ...form, last_name: e.target.value })} />
              </Field>
            </div>
            <Field label={t("crud.email")}>
              <input className={inputClass} value={form.email} onChange={(e) => setForm({ ...form, email: e.target.value })} />
            </Field>
            <Field label={t("crud.phone")}>
              <input className={inputClass} value={form.phone} onChange={(e) => setForm({ ...form, phone: e.target.value })} />
            </Field>
            <Field label={t("auth.password")}>
              <input type="password" className={inputClass} value={form.password} onChange={(e) => setForm({ ...form, password: e.target.value })} />
              {editing ? <div className="text-xs text-muted mt-1">{t("crud.passwordLeave")}</div> : null}
            </Field>
            <Field label={t("common.status")}>
              <select className={inputClass} value={form.status} onChange={(e) => setForm({ ...form, status: e.target.value })}>
                <option>ACTIVE</option>
                <option>BLOCKED</option>
                <option>SUSPENDED</option>
              </select>
            </Field>
            <Field label={t("crud.client")}>
              <select className={inputClass} value={form.client_id} onChange={(e) => setForm({ ...form, client_id: e.target.value })}>
                <option value="">—</option>
                {(clients.data?.items || []).map((c) => (
                  <option key={c.id} value={c.id}>{c.client_no} {c.email ? `· ${c.email}` : ""}</option>
                ))}
              </select>
            </Field>
            <div className="text-sm mb-2">{t("crud.roles")}</div>
            <div className="grid grid-cols-2 gap-1 mb-3 max-h-40 overflow-auto border border-line p-2">
              {(roles.data?.items || []).map((r) => (
                <label key={r.code} className="text-sm flex items-center gap-2">
                  <input type="checkbox" checked={form.roles.includes(r.code)} onChange={() => toggleRole(r.code)} />
                  {r.code}
                </label>
              ))}
            </div>
            <FormError error={mut.error} />
            <FormActions onCancel={close} submitLabel={editing ? t("common.save") : t("common.create")} />
          </form>
        </Modal>
      )}
    </div>
  );
}
