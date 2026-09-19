"use client";

import { useState } from "react";
import { api, errMsg, idr, rb, txTypeLabel, today, type Summary, type Tx } from "@/lib/api";

export default function ReportsPage() {
  const [from, setFrom] = useState(today());
  const [to, setTo] = useState(today());
  const [summary, setSummary] = useState<Summary | null>(null);
  const [txs, setTxs] = useState<Tx[]>([]);
  const [error, setError] = useState("");

  const load = async () => {
    setError("");
    try {
      const [s, t] = await Promise.all([
        api<Summary>(`/api/summary?from=${from}&to=${to}`),
        api<Tx[]>(`/api/transactions?from=${from}&to=${to}`),
      ]);
      setSummary(s);
      setTxs(t);
    } catch (e) {
      setError(errMsg(e));
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-end gap-3 card p-5">
        <div className="space-y-1">
          <label className="text-xs text-muted">Dari</label>
          <input type="date" value={from} onChange={(e) => setFrom(e.target.value)} className="input w-auto" />
        </div>
        <div className="space-y-1">
          <label className="text-xs text-muted">Sampai</label>
          <input type="date" value={to} onChange={(e) => setTo(e.target.value)} className="input w-auto" />
        </div>
        <button onClick={load} className="btn-primary">Lihat</button>
        <a
          href={`/api/report/export?from=${from}&to=${to}`}
          className="btn-ghost"
        >
          Ekspor CSV
        </a>
        {error && <p className="text-sm text-neg">{error}</p>}
      </div>

      {summary && (
        <>
          <div className="grid grid-cols-2 gap-3 md:grid-cols-5">
            <div className="card p-5">
              <p className="text-xs text-muted">Pendapatan</p>
              <p className="mt-2 text-xl font-semibold tracking-tight">{idr(summary.income)}</p>
            </div>
            <div className="card p-5">
              <p className="text-xs text-muted">HPP + biaya</p>
              <p className="mt-2 text-xl font-semibold tracking-tight">{idr(summary.cost)}</p>
              <p className="text-xs text-muted">HPP {idr(summary.cogs)} · biaya {idr(summary.expenses)}</p>
            </div>
            <div className="card p-5">
              <p className="text-xs text-muted">Profit</p>
              <p className={`mt-2 text-xl font-semibold tracking-tight ${summary.profit >= 0 ? "text-pos" : "text-neg"}`}>{idr(summary.profit)}</p>
            </div>
            <div className="card p-5">
              <p className="text-xs text-muted">R terjual</p>
              <p className="mt-2 text-xl font-semibold tracking-tight">{rb(summary.robux_sold)}</p>
            </div>
            <div className="card p-5">
              <p className="text-xs text-muted">R dibeli</p>
              <p className="mt-2 text-xl font-semibold tracking-tight">{rb(summary.robux_bought)}</p>
            </div>
          </div>

          <div className="card overflow-x-auto">
            <table className="tbl">
              <thead>
                <tr>
                  <th className="px-4 py-2.5.5">Tanggal</th>
                  <th className="px-4 py-2.5.5">Tipe</th>
                  <th className="px-4 py-2.5.5">Akun</th>
                  <th className="px-4 py-2.5.5">Counterpart</th>
                  <th className="px-4 py-2.5.5 text-right">Robux</th>
                  <th className="px-4 py-2.5.5 text-right">IDR</th>
                  <th className="px-4 py-2.5.5">Status</th>
                </tr>
              </thead>
              <tbody>
                {txs.length === 0 && (
                  <tr><td colSpan={7} className="px-4 py-4 text-center text-muted">Tidak ada transaksi pada periode ini.</td></tr>
                )}
                {txs.map((t) => (
                  <tr key={t.id}>
                    <td className="px-4 py-2.5 text-muted">{t.created_at}</td>
                    <td className="px-4 py-2.5">{txTypeLabel(t.type)}</td>
                    <td className="px-4 py-2.5">{t.account_name || "—"}</td>
                    <td className="px-4 py-2.5 text-muted">{t.counterpart || "—"}</td>
                    <td className="px-4 py-2.5 text-right">{t.robux_amount ? rb(t.robux_amount) : "—"}</td>
                    <td className="px-4 py-2.5 text-right">{t.idr_total ? idr(t.idr_total) : "—"}</td>
                    <td className="px-4 py-2.5">{t.status}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </>
      )}
    </div>
  );
}