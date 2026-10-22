import { useEffect, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";
import { useAuth } from "../auth";
import { DataGrid, DataGridColumn } from "../components/DataGrid";
import { Field, FormError, Modal, inputClass } from "../ui";

type Account = Record<string, unknown> & { id: string; account_number: string; currency: string; status: string };
type Ledger = Record<string, unknown> & { id: string; entry_type: string; debit: string; credit: string; bucket: string; created_at: string };
type Position = Record<string, unknown> & { instrument_id: string; ticker: string; quantity: string; average_price: string; market_value: string };
type Cash = { account_id: string; buckets: Record<string, string> };
type Portfolio = { available_funds: string; securities_value: string; positions: Position[] };
type CardResult = { amount: string; currency: string; card_brand: string; card_last4: string; operation_id: string };

const ledgerColumns: DataGridColumn<Ledger>[] = [
  { key: "created_at", label: "Дата", render: (r) => new Date(r.created_at).toLocaleString("ru-RU") },
  { key: "entry_type", label: "Операция" }, { key: "bucket", label: "Контур" },
  { key: "credit", label: "Зачислено", align: "right" }, { key: "debit", label: "Списано", align: "right" },
];

export function WalletPage() {
  const { can } = useAuth();
  const qc = useQueryClient();
  const accounts = useQuery({ queryKey: ["wallet-accounts"], queryFn: () => api<{ items: Account[] }>("/v1/accounts") });
  const [accountID, setAccountID] = useState("");
  const [depositOpen, setDepositOpen] = useState(false);
  const [amount, setAmount] = useState("");
  const [card, setCard] = useState({ number: "4242 4242 4242 4242", expiry: "12/30", cvc: "123", holder: "TEST CLIENT" });
  const [receipt, setReceipt] = useState<CardResult>();
  useEffect(() => { if (!accountID && accounts.data?.items[0]) setAccountID(accounts.data.items[0].id); }, [accountID, accounts.data]);
  const account = accounts.data?.items.find((a) => a.id === accountID);
  const cash = useQuery({ queryKey: ["wallet-cash", accountID], queryFn: () => api<Cash>(`/v1/accounts/${accountID}/cash`), enabled: !!accountID, refetchInterval: 5000 });
  const portfolio = useQuery({ queryKey: ["wallet-portfolio", accountID], queryFn: () => api<Portfolio>(`/v1/accounts/${accountID}/portfolio`), enabled: !!accountID, refetchInterval: 5000 });
  const ledger = useQuery({ queryKey: ["wallet-ledger", accountID], queryFn: () => api<{ items: Ledger[] }>(`/v1/accounts/${accountID}/ledger`), enabled: !!accountID });
  const deposit = useMutation({
    mutationFn: () => api<CardResult>("/v1/deposits/card/mock", { method: "POST", body: JSON.stringify({ account_id: accountID, amount, card_number: card.number, expiry: card.expiry, cvc: card.cvc, cardholder: card.holder, idempotency_key: crypto.randomUUID() }) }),
    onSuccess: (result) => { setReceipt(result); void qc.invalidateQueries({ queryKey: ["deposits"] }); void cash.refetch(); void ledger.refetch(); void portfolio.refetch(); },
  });
  const buckets = cash.data?.buckets || {};
  const positionColumns = useMemo<DataGridColumn<Position>[]>(() => [
    { key: "ticker", label: "Инструмент" }, { key: "quantity", label: "Количество", align: "right" },
    { key: "average_price", label: "Средняя цена", align: "right" }, { key: "market_value", label: "Стоимость", align: "right" },
  ], []);
  return <div>
    <div className="page-heading"><div><p className="eyebrow">Активы клиента</p><h1 className="page-title">Кошелёк и портфель</h1><p className="page-subtitle">Деньги, ценные бумаги и история всех движений в одном месте.</p></div>
      <div className="flex flex-wrap items-center gap-3">
        {(accounts.data?.items || []).length > 0 ? <select className="h-11 min-w-[260px] rounded-xl border border-slate-200 bg-white px-3 text-sm font-medium text-slate-700 shadow-sm outline-none transition focus:border-blue-400 focus:ring-4 focus:ring-blue-100" value={accountID} onChange={(e) => setAccountID(e.target.value)}>{(accounts.data?.items || []).map((a) => <option key={a.id} value={a.id}>{a.account_number} · {a.currency}</option>)}</select> : <span className="inline-flex h-11 items-center rounded-xl border border-amber-200 bg-amber-50 px-4 text-sm font-medium text-amber-800">Торговый счёт не открыт</span>}
        {can("payments.create") && <button className="inline-flex h-11 items-center gap-2 whitespace-nowrap rounded-xl bg-blue-600 px-5 text-sm font-semibold text-white shadow-[0_8px_18px_rgba(37,99,235,.22)] transition hover:-translate-y-0.5 hover:bg-blue-700 hover:shadow-[0_12px_24px_rgba(37,99,235,.28)] active:translate-y-0 disabled:cursor-not-allowed disabled:bg-slate-300 disabled:shadow-none" disabled={!accountID} title={!accountID ? "Сначала необходимо открыть торговый счёт" : "Пополнить баланс банковской картой"} onClick={() => { setReceipt(undefined); setDepositOpen(true); }}>
          <svg className="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round"><rect x="2" y="5" width="20" height="14" rx="3"/><path d="M2 10h20M16 15h2"/></svg>
          <span>Пополнить</span>
        </button>}
      </div>
    </div>
    <div className="mb-6 grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
      <BalanceCard label="Доступно для торговли" value={buckets.AVAILABLE} currency={account?.currency} tone="blue" />
      <BalanceCard label="Зарезервировано" value={buckets.BLOCKED} currency={account?.currency} tone="amber" />
      <BalanceCard label="Ожидает обработки" value={buckets.PENDING} currency={account?.currency} tone="violet" />
      <BalanceCard label="Ценные бумаги" value={portfolio.data?.securities_value} currency={account?.currency} tone="emerald" />
    </div>
    <div className="space-y-6">
      <DataGrid title="Ценные бумаги" subtitle="Позиции рассчитаны из исполненных сделок" rows={portfolio.data?.positions || []} columns={positionColumns} loading={portfolio.isLoading} onRefresh={() => void portfolio.refetch()} />
      <DataGrid title="История денег" subtitle="Неизменяемый журнал зачислений, списаний и резервов" rows={ledger.data?.items || []} columns={ledgerColumns} loading={ledger.isLoading} onRefresh={() => void ledger.refetch()} />
    </div>
    {depositOpen && <Modal title="Пополнение банковской картой" onClose={() => setDepositOpen(false)}>{receipt ? <div className="text-center"><div className="mx-auto grid h-16 w-16 place-items-center rounded-full bg-emerald-50 text-3xl text-emerald-600">✓</div><h3 className="mt-4 text-xl font-semibold">Кошелёк пополнен</h3><div className="mt-2 text-3xl font-bold text-slate-950">+{receipt.amount} {receipt.currency}</div><p className="mt-2 text-sm text-slate-500">{receipt.card_brand} •••• {receipt.card_last4}</p><div className="mt-5 rounded-xl bg-slate-50 p-3 text-xs text-slate-500">Тестовая операция: {receipt.operation_id}</div><button className="btn-primary mt-5 w-full" onClick={() => { setDepositOpen(false); setAmount(""); }}>Готово</button></div> : <form onSubmit={(e) => { e.preventDefault(); deposit.mutate(); }}>
      <div className="mb-5 flex items-center justify-between rounded-xl border border-violet-200 bg-violet-50 p-3"><div><div className="text-sm font-semibold text-violet-900">Тестовый режим</div><div className="text-xs text-violet-700">Настоящие деньги не списываются</div></div><span className="rounded-full bg-violet-600 px-2.5 py-1 text-[10px] font-bold uppercase tracking-wider text-white">Mock</span></div>
      <Field label={`Сумма пополнения, ${account?.currency || "KGS"}`}><input autoFocus required min="1" step="0.01" type="number" className={inputClass} value={amount} onChange={(e) => setAmount(e.target.value)} placeholder="10 000" /></Field>
      <div className="mb-4 flex gap-2">{["1000","5000","10000","50000"].map((v) => <button type="button" key={v} className={`flex-1 rounded-lg border py-2 text-xs font-semibold ${amount === v ? "border-blue-500 bg-blue-50 text-blue-700" : "border-slate-200 text-slate-500 hover:bg-slate-50"}`} onClick={() => setAmount(v)}>{Number(v).toLocaleString("ru-RU")}</button>)}</div>
      <div className="mb-4 rounded-2xl bg-gradient-to-br from-blue-600 to-indigo-700 p-5 text-white shadow-[0_15px_35px_rgba(37,99,235,.22)]"><div className="flex justify-between text-xs font-semibold tracking-widest"><span>TEST CARD</span><span>VISA</span></div><div className="mt-8 text-lg tracking-[.18em]">{card.number || "•••• •••• •••• ••••"}</div><div className="mt-5 flex justify-between text-xs"><span>{card.holder || "CARDHOLDER"}</span><span>{card.expiry || "MM/YY"}</span></div></div>
      <Field label="Номер карты"><input required inputMode="numeric" className={inputClass} value={card.number} onChange={(e) => setCard({ ...card, number: e.target.value.replace(/\D/g, "").slice(0,16).replace(/(.{4})/g, "$1 ").trim() })} /></Field>
      <div className="grid grid-cols-2 gap-3"><Field label="Срок действия"><input required className={inputClass} value={card.expiry} onChange={(e) => { const v=e.target.value.replace(/\D/g,"").slice(0,4); setCard({...card, expiry: v.length > 2 ? `${v.slice(0,2)}/${v.slice(2)}` : v}); }} /></Field><Field label="CVC"><input required inputMode="numeric" type="password" className={inputClass} value={card.cvc} onChange={(e) => setCard({...card,cvc:e.target.value.replace(/\D/g,"").slice(0,3)})} /></Field></div>
      <Field label="Имя владельца"><input required className={inputClass} value={card.holder} onChange={(e) => setCard({...card,holder:e.target.value.toUpperCase()})} /></Field>
      <div className="mb-3 text-xs text-slate-400">Успешная карта: 4242 4242 4242 4242 · Для отказа: 4000 0000 0000 0002</div><FormError error={deposit.error} />
      <div className="mt-5 flex gap-2"><button type="button" className="flex-1 rounded-xl border border-slate-200 py-2.5 text-sm" onClick={() => setDepositOpen(false)}>Отмена</button><button type="submit" disabled={deposit.isPending || !amount} className="flex-[2] rounded-xl bg-blue-600 py-2.5 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-50">{deposit.isPending ? "Обработка…" : `Пополнить ${amount ? Number(amount).toLocaleString("ru-RU") : ""} ${String(account?.currency || "")}`}</button></div>
    </form>}</Modal>}
  </div>;
}

function BalanceCard({ label, value, currency, tone }: { label: string; value?: string; currency?: unknown; tone: string }) {
  const color: Record<string,string> = { blue: "bg-blue-50 text-blue-700", amber: "bg-amber-50 text-amber-700", violet: "bg-violet-50 text-violet-700", emerald: "bg-emerald-50 text-emerald-700" };
  return <div className="surface p-5"><div className={`mb-4 grid h-10 w-10 place-items-center rounded-xl ${color[tone]}`}>◈</div><div className="text-sm text-slate-500">{label}</div><div className="mt-1 text-2xl font-semibold tabular-nums text-slate-950">{value ?? "—"} <span className="text-sm font-medium text-slate-400">{String(currency || "")}</span></div></div>;
}
