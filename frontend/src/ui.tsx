import type { FormEvent, ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { ApiRequestError } from "./api";

export function Modal({
  title,
  children,
  onClose,
}: {
  title: string;
  children: ReactNode;
  onClose: () => void;
}) {
  return (
    <div className="fixed inset-0 bg-black/40 z-50 grid place-items-center p-4">
      <div className="surface w-full max-w-xl max-h-[90vh] overflow-auto p-6">
        <div className="flex items-start justify-between mb-4 gap-4">
          <h2 className="text-lg">{title}</h2>
          <button type="button" className="text-muted text-xl leading-none" onClick={onClose}>
            ×
          </button>
        </div>
        {children}
      </div>
    </div>
  );
}

export function Field({
  label,
  children,
}: {
  label: string;
  children: ReactNode;
}) {
  return (
    <label className="block text-sm mb-3">
      {label}
      <div className="mt-1">{children}</div>
    </label>
  );
}

export const inputClass = "w-full border border-line px-3 py-2 bg-white outline-none focus:border-brand focus:ring-2 focus:ring-brand/20";

export function FormError({ error }: { error?: unknown }) {
  const { t } = useTranslation();
  if (!error) return null;
  const req = error instanceof ApiRequestError ? error : null;
  const mapped = req?.code ? t(`errors.${req.code}`, { defaultValue: "" }) : "";
  const message = mapped || (error as Error).message;
  const fieldEntries = Object.entries(req?.fields || {});
  return (
    <div className="text-sm text-red-800 bg-[#FDECEC] border border-red-200 rounded-xl px-3 py-2 mb-3">
      <div>{message}</div>
      {fieldEntries.length > 0 && (
        <ul className="mt-1 list-disc pl-4 text-xs">
          {fieldEntries.map(([name, reason]) => (
            <li key={name}>{name}: {t(`errors.reason.${reason}`, { defaultValue: reason })}</li>
          ))}
        </ul>
      )}
    </div>
  );
}

export function FormActions({
  onCancel,
  submitLabel,
  cancelLabel,
}: {
  onCancel: () => void;
  submitLabel: string;
  cancelLabel?: string;
}) {
  const { t } = useTranslation();
  return (
    <div className="flex justify-end gap-2 mt-4">
      <button type="button" className="px-3 py-2 border border-line text-sm rounded-xl hover:bg-paper" onClick={onCancel}>
        {cancelLabel || t("crud.cancel")}
      </button>
      <button type="submit" className="btn-primary">
        {submitLabel}
      </button>
    </div>
  );
}

export function onSubmit(fn: () => void) {
  return (e: FormEvent) => {
    e.preventDefault();
    fn();
  };
}
