export type User = { id: number; username: string; role: "admin" | "staff" };
export type Account = {
  id: number;
  name: string;
  username_roblox: string;
  owner: string;
  initial_balance_robux: number;
  current_robux: number;
  active: boolean;
  stock_status: "ready" | "pending" | "borrow";
};
export type Tx = {
  id: number;
  type: "topup" | "penjualan" | "fee" | "lain" | "transfer";
  account_id: number | null;
  from_account_id: number | null;
  to_account_id: number | null;
  counterpart: string;
  robux_amount: number;
  rate_idr: number;
  fee_idr: number;
  idr_total: number;
  status: string;
  note: string;
  created_at: string;
  created_by: number;
  account_name: string;
  from_name: string;
  to_name: string;
};
export type Summary = {
  income: number;
  cost: number;
  profit: number;
  robux_sold: number;
  robux_bought: number;
  cogs: number;
  expenses: number;
  topup_spent: number;
  sales_count: number;
  avg_buy_rate: number;
  avg_sell_rate: number;
  pending_count: number;
  daily: { date: string; income: number; robux_sold: number; profit: number }[];
  per_account: { name: string; count: number; robux_sold: number; income: number }[];
  top_buyers: { name: string; count: number; robux: number; income: number }[];
  from: string | null;
  to: string | null;
  accounts: Account[];
};

export const idr = (n: number) =>
  new Intl.NumberFormat("id-ID", { style: "currency", currency: "IDR", maximumFractionDigits: 0 }).format(n);
export const rb = (n: number) => `${n.toLocaleString("id-ID")} R`;

const TYPE_LABEL: Record<string, string> = {
  topup: "Topup (beli)",
  penjualan: "Penjualan",
  fee: "Fee",
  lain: "Lain",
  transfer: "Transfer",
};
export const txTypeLabel = (t: string) => TYPE_LABEL[t] || t;

// Arah dana per tipe transaksi: penjualan menambah kas (+),
// topup/fee/lain mengurangi kas (−), transfer hanya memindah Robux (netral).
export const txSign = (type: string) => (type === "penjualan" ? 1 : type === "transfer" ? 0 : -1);
export const signedIdr = (type: string, total: number) => {
  const s = txSign(type);
  if (!total || s === 0) return "—";
  return `${s > 0 ? "+" : "−"}${idr(Math.abs(total))}`;
};
export const signedClass = (type: string, total: number) => {
  const s = txSign(type);
  if (!total || s === 0) return "text-muted";
  return s > 0 ? "text-pos" : "text-neg";
};
// Pakai waktu lokal (WIB), bukan UTC — toISOString() membuat tanggal mundur sebelum jam 07:00.
const pad = (n: number) => String(n).padStart(2, "0");
export const today = () => {
  const d = new Date();
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
};
export const nowLocal = () => {
  const d = new Date();
  return `${today()}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
};
export const LOW_BALANCE = 500;

export async function api<T = unknown>(path: string, opts?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    credentials: "same-origin",
    headers: opts && opts.body ? { "Content-Type": "application/json" } : undefined,
    ...opts,
  });
  const data: unknown = await res.json().catch(() => null);
  if (!res.ok) throw new Error(((data as { error?: string } | null)?.error) || res.statusText);
  return data as T;
}

export const errMsg = (e: unknown) => (e instanceof Error ? e.message : String(e));