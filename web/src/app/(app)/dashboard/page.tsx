"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { api, errMsg, idr, LOW_BALANCE, rb, txTypeLabel, today, type Summary, type Tx } from "@/lib/api";

function Card({ label, value, sub, tone = "text-fg" }: { label: string; value: string; sub?: string; tone?: string }) {
  return (
    <div className="card p-5">
      <p className="text-xs text-muted">{label}</p>
      <p className={`mt-2 text-2xl font-semibold tracking-tight ${tone}`}>{value}</p>
      {sub && <p className="mt-0.5 text-xs text-muted">{sub}</p>}
    </div>
  );
}

const rate = (n: number) => `${idr(Math.round(n))}/R`;
const daysAgo = (n: number) => {
  const d = new Date();
  d.setDate(d.getDate() - n);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
};

function DailyChart({ data }: { data: Summary["daily"] }) {
  // isi hari kosong supaya sumbu waktu tidak meloncat
  const byDate = new Map(data.map((d) => [d.date, d]));
  const days = Array.from({ length: 30 }, (_, i) => daysAgo(29 - i)).map(
    (date) => byDate.get(date) || { date, income: 0, robux_sold: 0, profit: 0 },
  );
  const max = Math.max(1, ...days.map((d) => d.income));
  return (
    <div className="card p-5">
      <div className="flex h-40 items-end gap-1">
        {days.map((d) => (
          <div key={d.date} className="group relative flex h-full flex-1 items-end">
            <div
              className={`w-full rounded-t ${d.profit < 0 ? "bg-neg/70" : "bg-pos/80"} group-hover:opacity-80`}
              style={{ height: `${(d.income / max) * 100}%`, minHeight: d.income ? 2 : 0 }}
            />
            <div className="pointer-events-none absolute bottom-full left-1/2 z-10 mb-1 hidden -translate-x-1/2 whitespace-nowrap rounded-md border border-line bg-surface px-2.5 py-1.5 text-xs shadow-sm group-hover:block">
              <p className="font-semibold">{d.date}</p>
              <p>Penjualan: {idr(d.income)}</p>
              <p>R terjual: {rb(d.robux_sold)}</p>
              <p>Profit: {idr(d.profit)}</p>
            </div>
          </div>
        ))}
      </div>
      <div className="mt-1 flex justify-between text-xs text-muted">
        <span>{days[0].date}</span>
        <span>{days[days.length - 1].date}</span>
      </div>
    </div>
  );
}

// Saldo per Akun: kartu per akun, 10 per halaman.
const PER_PAGE = 10;

function AccountBalances({ accounts }: { accounts: Summary["accounts"] }) {
  const [page, setPage] = useState(0);
  const pages = Math.max(1, Math.ceil(accounts.length / PER_PAGE));
  const current = Math.min(page, pages - 1);
  const shown = accounts.slice(current * PER_PAGE, current * PER_PAGE + PER_PAGE);
  return (
    <section>
      <div className="flex items-baseline justify-between gap-3">
        <h2 className="section-title">Saldo per Akun</h2>
        <span className="text-xs text-muted">{accounts.length} akun</span>
      </div>
      {accounts.length === 0 ? (
        <div className="card p-5 text-center text-sm text-muted">Belum ada akun.</div>
      ) : (
        <>
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {shown.map((a) => (
              <div key={a.id} className="card p-4">
                <p className="truncate font-medium" title={a.username_roblox || "—"}>
                  {a.username_roblox || "—"}
                </p>
                <div className="mt-3 flex items-end justify-between gap-3">
                  <div>
                    <p className="text-xs text-muted">Saldo awal</p>
                    <p className="mt-0.5 text-sm">{rb(a.initial_balance_robux)}</p>
                  </div>
                  <div className="text-right">
                    <p className="text-xs text-muted">Saldo sekarang</p>
                    <p className={`mt-0.5 text-lg font-semibold tracking-tight ${
                      a.current_robux < LOW_BALANCE ? "text-warn" : "text-fg"
                    }`}>{rb(a.current_robux)}</p>
                  </div>
                </div>
              </div>
            ))}
          </div>
          {pages > 1 && (
            <div className="mt-3 flex items-center justify-between gap-3">
              <button
                className="btn-ghost disabled:opacity-40"
                onClick={() => setPage(current - 1)}
                disabled={current === 0}
              >
                Sebelumnya
              </button>
              <span className="text-xs text-muted">
                Halaman {current + 1} dari {pages}
              </span>
              <button
                className="btn-ghost disabled:opacity-40"
                onClick={() => setPage(current + 1)}
                disabled={current >= pages - 1}
              >
                Selanjutnya
              </button>
            </div>
          )}
        </>
      )}
    </section>
  );
}

function MiniTable({ title, head, rows }: { title: string; head: string[]; rows: (string | number)[][] }) {
  return (
    <section>
      <h2 className="section-title">{title}</h2>
      <div className="card overflow-x-auto">
        <table className="tbl">
          <thead>
            <tr>{head.map((h, i) => <th key={h} className={`px-4 py-2.5 ${i ? "text-right" : ""}`}>{h}</th>)}</tr>
          </thead>
          <tbody>
            {rows.length === 0 && (
              <tr><td colSpan={head.length} className="px-4 py-4 text-center text-muted">Belum ada data.</td></tr>
            )}
            {rows.map((r, i) => (
              <tr key={i}>
                {r.map((c, j) => <td key={j} className={`px-4 py-2.5 ${j ? "text-right" : "font-medium"}`}>{c}</td>)}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}

export default function DashboardPage() {
  const [summary, setSummary] = useState<Summary | null>(null);
  const [todaySum, setTodaySum] = useState<Summary | null>(null);
  const [month, setMonth] = useState<Summary | null>(null);
  const [recent, setRecent] = useState<Tx[]>([]);
  const [error, setError] = useState("");

  useEffect(() => {
    (async () => {
      try {
        const [s, td, m, tr] = await Promise.all([
          api<Summary>("/api/summary"),
          api<Summary>(`/api/summary?from=${today()}&to=${today()}`),
          api<Summary>(`/api/summary?from=${daysAgo(29)}&to=${today()}`),
          api<Tx[]>("/api/transactions?limit=10"),
        ]);
        setSummary(s);
        setTodaySum(td);
        setMonth(m);
        setRecent(tr);
      } catch (e) {
        setError(errMsg(e));
      }
    })();
  }, []);

  if (error) return <p className="text-neg">{error}</p>;
  if (!summary || !todaySum || !month) return <p>Memuat…</p>;

  const low = summary.accounts.filter((a) => a.current_robux < LOW_BALANCE);
  const margin = summary.avg_sell_rate - summary.avg_buy_rate;

  return (
    <div className="space-y-6">
      {(low.length > 0 || summary.pending_count > 0) && (
        <div className="space-y-2">
          {low.length > 0 && (
            <div className="card border-l-2 border-l-warn px-4 py-3 text-sm">
              Saldo menipis (&lt; {rb(LOW_BALANCE)}): {low.map((a) => `${a.name} (${rb(a.current_robux)})`).join(", ")}
            </div>
          )}
          {summary.pending_count > 0 && (
            <Link href="/transactions" className="card block border-l-2 border-l-fg px-4 py-3 text-sm hover:bg-hover">
              {summary.pending_count} transaksi masih pending — cek di halaman Transaksi
            </Link>
          )}
        </div>
      )}

      <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
        <Card label="Profit hari ini" value={idr(todaySum.profit)} sub={`${todaySum.sales_count} penjualan · ${rb(todaySum.robux_sold)}`} tone={todaySum.profit >= 0 ? "text-pos" : "text-neg"} />
        <Card label="Profit 30 hari" value={idr(month.profit)} sub={`Penjualan ${idr(month.income)}`} tone={month.profit >= 0 ? "text-pos" : "text-neg"} />
        <Card label="Profit (semua)" value={idr(summary.profit)} sub={`Pendapatan ${idr(summary.income)}`} tone={summary.profit >= 0 ? "text-pos" : "text-neg"} />
        <Card label="Stok R semua akun" value={rb(summary.accounts.reduce((a, c) => a + c.current_robux, 0))} sub={`${summary.accounts.length} akun aktif`} />
        <Card label="Harga beli rata-rata" value={rate(summary.avg_buy_rate)} sub={`Total topup ${idr(summary.topup_spent)}`} />
        <Card label="Harga jual rata-rata" value={rate(summary.avg_sell_rate)} sub={`${rb(summary.robux_sold)} terjual`} />
        <Card label="Margin per R" value={summary.robux_sold ? rate(margin) : "—"} sub={summary.robux_sold ? "Harga jual − harga beli rata-rata" : "Belum ada penjualan"} />
        <Card label="HPP + biaya" value={idr(summary.cost)} sub={`HPP ${idr(summary.cogs)} · biaya ${idr(summary.expenses)}`} />
      </div>

      <section>
        <h2 className="section-title">Penjualan 30 Hari Terakhir</h2>
        <DailyChart data={month.daily ?? []} />
      </section>

      <div className="grid gap-6 lg:grid-cols-2">
        <MiniTable
          title="Penjualan per Akun (30 hari)"
          head={["Akun", "Trx", "R", "Pendapatan"]}
          rows={(month.per_account ?? []).map((p) => [p.name, p.count, rb(p.robux_sold), idr(p.income)])}
        />
        <MiniTable
          title="Pembeli Teratas (30 hari)"
          head={["Pembeli", "Trx", "R", "Total"]}
          rows={(month.top_buyers ?? []).map((b) => [b.name, b.count, rb(b.robux), idr(b.income)])}
        />
      </div>

      <AccountBalances accounts={summary.accounts} />

      <section>
        <h2 className="section-title">Transaksi Terakhir</h2>
        <div className="card overflow-x-auto">
          <table className="tbl">
            <thead>
              <tr>
                <th className="px-4 py-2.5.5">Tanggal</th>
                <th className="px-4 py-2.5.5">Tipe</th>
                <th className="px-4 py-2.5.5">Akun</th>
                <th className="px-4 py-2.5.5 text-right">Robux</th>
                <th className="px-4 py-2.5.5 text-right">IDR</th>
                <th className="px-4 py-2.5.5">Status</th>
              </tr>
            </thead>
            <tbody>
              {recent.length === 0 && (
                <tr>
                  <td colSpan={6} className="px-4 py-4 text-center text-muted">Belum ada transaksi.</td>
                </tr>
              )}
              {recent.map((t) => (
                <tr key={t.id}>
                  <td className="px-4 py-2.5 text-muted">{t.created_at}</td>
                  <td className="px-4 py-2.5">{txTypeLabel(t.type)}</td>
                  <td className="px-4 py-2.5">{t.account_name || "—"}</td>
                  <td className="px-4 py-2.5 text-right">{t.robux_amount ? rb(t.robux_amount) : "—"}</td>
                  <td className="px-4 py-2.5 text-right">{t.idr_total ? idr(t.idr_total) : "—"}</td>
                  <td className="px-4 py-2.5">
                    <span className={`badge ${
                      t.status === "selesai" ? "text-pos" : t.status === "pending" ? "text-warn" : "text-muted"
                    }`}>{t.status}</span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  );
}
