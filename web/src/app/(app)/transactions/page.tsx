"use client";

import { useEffect, useState } from "react";
import Modal from "@/components/Modal";
import NumberInput from "@/components/NumberInput";
import { api, errMsg, idr, nowLocal, rb, signedClass, signedIdr, txSign, type Account, type Tx } from "@/lib/api";

const empty = { type: "topup", account_id: "", counterpart: "", robux_amount: 0, rate_idr: 0, fee_idr: 0, status: "selesai", note: "", created_at: "" };
const fresh = () => ({ ...empty, created_at: nowLocal() });
const TYPE_SHORT: Record<string, string> = { topup: "Topup", penjualan: "Jual", lain: "Lain", fee: "Fee", subscribe: "Subscribe", transfer: "Transfer" };
const emptyTransfer = { from_account_id: "", to_account_id: "", robux_amount: 0, note: "" };

export default function TransactionsPage() {
  const [txs, setTxs] = useState<Tx[]>([]);
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [form, setForm] = useState(fresh);
  const [editId, setEditId] = useState<number | null>(null);
  const [editTransfer, setEditTransfer] = useState<Tx | null>(null);
  const [transfer, setTransfer] = useState(emptyTransfer);
  const [filter, setFilter] = useState<Record<string, string>>({});
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [transferBusy, setTransferBusy] = useState(false);
  const [modal, setModal] = useState<null | "tx" | "transfer" | "transferDetail">(null);
  const [showFilter, setShowFilter] = useState(false);

  const load = async (f = filter) => {
    const q = new URLSearchParams();
    if (f.type) q.set("type", f.type);
    if (f.account_id) q.set("account_id", f.account_id);
    if (f.from) q.set("from", f.from);
    if (f.to) q.set("to", f.to);
    if (f.status) q.set("status", f.status);
    const s = q.toString();
    const [t, a] = await Promise.all([api<Tx[]>(`/api/transactions${s ? `?${s}` : ""}`), api<Account[]>("/api/accounts")]);
    setTxs(t);
    setAccounts(a);
  };
  useEffect(() => {
    Promise.all([api<Tx[]>("/api/transactions"), api<Account[]>("/api/accounts")])
      .then(([t, a]) => {
        setTxs(t);
        setAccounts(a);
      })
      .catch((e) => setError(errMsg(e)));
  }, []);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      await api(editId ? `/api/transactions/${editId}` : "/api/transactions", {
        method: editId ? "PUT" : "POST",
        body: JSON.stringify({ ...form, account_id: form.account_id ? Number(form.account_id) : null }),
      });
      setForm(fresh());
      setEditId(null);
      setModal(null);
      await load();
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setBusy(false);
    }
  };

  const submitTransfer = async (e: React.FormEvent) => {
    e.preventDefault();
    setTransferBusy(true);
    setError("");
    try {
      await api("/api/transactions/transfer", {
        method: "POST",
        body: JSON.stringify({ ...transfer, from_account_id: Number(transfer.from_account_id), to_account_id: Number(transfer.to_account_id) }),
      });
      setTransfer(emptyTransfer);
      setModal(null);
      await load();
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setTransferBusy(false);
    }
  };

  const cancelEdit = () => {
    setModal(null);
    setEditId(null);
    setEditTransfer(null);
    setForm(fresh());
  };

  const edit = (t: Tx) => {
    setError("");
    if (t.type === "transfer") {
      // transfer tidak bisa diubah, hanya dihapus
      setEditId(null);
      setEditTransfer(t);
      setModal("transferDetail");
      return;
    }
    setEditTransfer(null);
    setModal("tx");
    setEditId(t.id);
    setForm({
      type: t.type, account_id: t.account_id ? String(t.account_id) : "", counterpart: t.counterpart,
      robux_amount: t.robux_amount, rate_idr: t.rate_idr, fee_idr: t.fee_idr, status: t.status, note: t.note,
      created_at: t.created_at.replace(" ", "T"),
    });
  };

  const del = async (id: number) => {
    if (!confirm("Hapus transaksi ini? Tindakan ini tidak bisa dibatalkan.")) return;
    try {
      await api(`/api/transactions/${id}`, { method: "DELETE" });
      cancelEdit();
      await load();
    } catch (e) {
      setError(errMsg(e));
    }
  };

  const openNew = () => {
    setError("");
    setEditId(null);
    setEditTransfer(null);
    setForm(fresh());
    setModal("tx");
  };
  const openTransfer = () => {
    setError("");
    setModal("transfer");
  };

  const previewTotal = form.robux_amount * form.rate_idr + form.fee_idr;
  // topup/lain/subscribe mengurangi dana, penjualan menambah
  const previewSign = txSign(form.type);
  // subscribe = biaya langganan akun, tidak memakai robux & rate
  const isSubscribe = form.type === "subscribe";

  return (
    <div className="space-y-6">
      <div className="space-y-4">
          <div className="flex flex-wrap items-center gap-2">
            <button onClick={openNew} className="btn-primary h-10">+ Transaksi baru</button>
            <button onClick={openTransfer} className="btn-ghost h-10">Transfer antar akun</button>
            <button onClick={() => setShowFilter(!showFilter)} className="btn-ghost h-10 sm:hidden">
              Filter {showFilter ? "▴" : "▾"}
            </button>
          </div>
          {error && !modal && <p className="text-sm text-neg">{error}</p>}
          <div className={`${showFilter ? "grid" : "hidden"} grid-cols-2 gap-2 text-sm sm:flex sm:flex-wrap sm:items-center`}>
            <select value={filter.type || ""} onChange={(e) => { const f = { ...filter, type: e.target.value }; setFilter(f); load(f); }} className="input sm:w-auto">
              <option value="">Semua tipe</option>
              <option value="topup">Topup</option>
              <option value="penjualan">Penjualan</option>
              <option value="lain">Lain</option>
              <option value="subscribe">Subscribe akun</option>
              <option value="transfer">Transfer</option>
            </select>
            <select value={filter.account_id || ""} onChange={(e) => { const f = { ...filter, account_id: e.target.value }; setFilter(f); load(f); }} className="input sm:w-auto">
              <option value="">Semua akun</option>
              {accounts.map((a) => <option key={a.id} value={a.id}>{a.name}</option>)}
            </select>
            <select value={filter.status || ""} onChange={(e) => { const f = { ...filter, status: e.target.value }; setFilter(f); load(f); }} className="input sm:w-auto">
              <option value="">Semua status</option>
              <option value="selesai">Selesai</option>
              <option value="pending">Pending</option>
              <option value="dibatalkan">Dibatalkan</option>
            </select>
            <input type="date" value={filter.from || ""} onChange={(e) => { const f = { ...filter, from: e.target.value }; setFilter(f); load(f); }} className="input sm:w-auto" />
            <span className="hidden self-center text-muted sm:inline">s/d</span>
            <input type="date" value={filter.to || ""} onChange={(e) => { const f = { ...filter, to: e.target.value }; setFilter(f); load(f); }} className="input sm:w-auto" />
          </div>

          <div className="card overflow-x-auto">
            <table className="tbl whitespace-nowrap">
              <thead>
                <tr>
                  <th className="px-5 py-3">Tanggal</th>
                  <th className="px-5 py-3">Tipe</th>
                  <th className="px-5 py-3">Akun</th>
                  <th className="px-5 py-3 text-right">Robux</th>
                  <th className="px-5 py-3 text-right">Dana (IDR)</th>
                  <th className="px-5 py-3">Status</th>
                  <th className="px-5 py-3"></th>
                </tr>
              </thead>
              <tbody>
                {txs.length === 0 && (
                  <tr><td colSpan={7} className="px-5 py-10 text-center text-muted">Tidak ada transaksi.</td></tr>
                )}
                {txs.map((t) => {
                  const [date, time] = t.created_at.split(" ");
                  return (
                    <tr key={t.id} className={editId === t.id || editTransfer?.id === t.id ? "bg-hover" : ""}>
                      <td className="px-5 py-3.5">
                        <p className="font-medium">{date}</p>
                        <p className="text-xs text-muted">{time}</p>
                      </td>
                      <td className="px-5 py-3.5">
                        <span className="rounded-md border border-line bg-bg px-2 py-0.5 text-xs font-medium">{TYPE_SHORT[t.type] || t.type}</span>
                      </td>
                      <td className="max-w-48 px-5 py-3.5">
                        <p className="truncate font-medium">{t.account_name || "—"}</p>
                        {t.counterpart && <p className="truncate text-xs text-muted">{t.counterpart}</p>}
                      </td>
                      <td className="px-5 py-3.5 text-right font-medium tabular-nums">{t.robux_amount ? rb(t.robux_amount) : "—"}</td>
                      <td className={`px-5 py-3.5 text-right font-medium tabular-nums ${signedClass(t.type, t.idr_total)}`}>{signedIdr(t.type, t.idr_total)}</td>
                      <td className="px-5 py-3.5">
                        <span className={`badge capitalize ${
                          t.status === "selesai" ? "text-ready" : t.status === "pending" ? "text-pending" : "text-muted"
                        }`}>{t.status}</span>
                      </td>
                      <td className="px-5 py-3.5 text-right">
                        <button onClick={() => edit(t)} className="link">Edit</button>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
      </div>

      <Modal open={modal === "tx"} title={editId ? `Edit Transaksi #${editId}` : "Transaksi Baru"} onClose={cancelEdit}>
          <form onSubmit={submit} className="space-y-3">
            <select value={form.type} onChange={(e) => setForm({ ...form, type: e.target.value })} className="input">
              <option value="topup">Topup — beli R (modal keluar)</option>
              <option value="penjualan">Penjualan — jual R (pendapatan)</option>
              <option value="lain">Lain / pengeluaran</option>
              <option value="subscribe">Subscribe akun — langganan (mis. Premium)</option>
            </select>
            <select value={form.account_id} onChange={(e) => setForm({ ...form, account_id: e.target.value })} className="input">
              <option value="">Akun Roblox *</option>
              {accounts.map((a) => <option key={a.id} value={a.id}>{a.name} ({rb(a.current_robux)})</option>)}
            </select>
            <input placeholder={isSubscribe ? "Penyedia langganan (opsional)" : "Counterpart (pembeli/penjual)"} value={form.counterpart} onChange={(e) => setForm({ ...form, counterpart: e.target.value })} className="input" />
            {isSubscribe ? (
              <label className="block text-xs text-muted">
                Biaya langganan (IDR)
                <NumberInput placeholder="0" value={form.fee_idr} onValue={(n) => setForm({ ...form, fee_idr: n })} className="input mt-1" />
              </label>
            ) : (
            <div className="grid grid-cols-2 gap-2">
              <label className="block text-xs text-muted">
                Jumlah Robux
                <NumberInput placeholder="0" value={form.robux_amount} onValue={(n) => setForm({ ...form, robux_amount: n })} className="input mt-1" />
              </label>
              <label className="block text-xs text-muted">
                Rate per R (IDR)
                <NumberInput placeholder="0" value={form.rate_idr} onValue={(n) => setForm({ ...form, rate_idr: n })} className="input mt-1" />
              </label>
            </div>
            )}
            <p className="text-xs text-muted">
              {previewSign > 0 ? "Dana masuk" : "Dana keluar"}:{" "}
              <span className={`font-semibold ${previewSign > 0 ? "text-pos" : "text-neg"}`}>
                {previewSign > 0 ? "+" : "−"}{idr(previewTotal)}
              </span>{" "}
              {isSubscribe ? "(biaya langganan)" : "(robux × rate)"}
            </p>
            <select value={form.status} onChange={(e) => setForm({ ...form, status: e.target.value })} className="input">
              <option value="selesai">Selesai</option>
              <option value="pending">Pending</option>
              <option value="dibatalkan">Dibatalkan</option>
            </select>
            <input type="datetime-local" value={form.created_at} onChange={(e) => setForm({ ...form, created_at: e.target.value })} className="input" />
            <input placeholder="Catatan" value={form.note} onChange={(e) => setForm({ ...form, note: e.target.value })} className="input" />
            {error && <p className="text-sm text-neg">{error}</p>}
            <button disabled={busy} className="btn-primary w-full">
              {busy ? "Menyimpan…" : editId ? "Simpan Perubahan" : "Simpan Transaksi"}
            </button>
            {editId && (
              <>
                <button type="button" onClick={cancelEdit} className="btn-ghost w-full">Batal edit</button>
                <div className="border-t border-line pt-3">
                  <button type="button" onClick={() => del(editId)} className="btn-ghost w-full text-muted hover:border-fg hover:text-fg">
                    Hapus transaksi ini
                  </button>
                </div>
              </>
            )}
          </form>
      </Modal>

      <Modal open={modal === "transferDetail" && !!editTransfer} title={`Transfer #${editTransfer?.id ?? ""}`} onClose={cancelEdit}>
          {editTransfer && (
            <div className="space-y-3">
              <div className="text-sm">
                <p className="font-medium">{editTransfer.from_name} → {editTransfer.to_name}</p>
                <p className="text-muted">{rb(editTransfer.robux_amount)} · {editTransfer.created_at}</p>
              </div>
              <p className="text-xs text-muted">Transfer tidak bisa diubah. Hapus lalu buat ulang bila ada yang salah.</p>
              {error && <p className="text-sm text-neg">{error}</p>}
              <button type="button" onClick={() => del(editTransfer.id)} className="btn-primary w-full">Hapus transfer ini</button>
              <button type="button" onClick={cancelEdit} className="btn-ghost w-full">Batal</button>
            </div>
          )}
      </Modal>

      <Modal open={modal === "transfer"} title="Transfer Antar Akun" onClose={() => setModal(null)}>
          <form onSubmit={submitTransfer} className="space-y-3">
            <select value={transfer.from_account_id} onChange={(e) => setTransfer({ ...transfer, from_account_id: e.target.value })} className="input">
              <option value="">Dari akun *</option>
              {accounts.map((a) => <option key={a.id} value={a.id}>{a.name} ({rb(a.current_robux)})</option>)}
            </select>
            <select value={transfer.to_account_id} onChange={(e) => setTransfer({ ...transfer, to_account_id: e.target.value })} className="input">
              <option value="">Ke akun *</option>
              {accounts.map((a) => <option key={a.id} value={a.id}>{a.name}</option>)}
            </select>
            <label className="block text-xs text-muted">
              Jumlah Robux
              <NumberInput placeholder="0" value={transfer.robux_amount} onValue={(n) => setTransfer({ ...transfer, robux_amount: n })} className="input mt-1" />
            </label>
            <input placeholder="Catatan" value={transfer.note} onChange={(e) => setTransfer({ ...transfer, note: e.target.value })} className="input" />
            {error && <p className="text-sm text-neg">{error}</p>}
            <button disabled={transferBusy} className="btn-primary w-full">
              {transferBusy ? "Memindahkan…" : "Transfer"}
            </button>
          </form>
      </Modal>
    </div>
  );
}