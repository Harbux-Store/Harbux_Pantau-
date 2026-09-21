"use client";

import { useEffect, useState } from "react";
import Modal from "@/components/Modal";
import NumberInput from "@/components/NumberInput";
import { api, daysLeft, errMsg, rb, type Account } from "@/lib/api";

type Stock = Account["stock_status"];
type Form = { username_roblox: string; robux: number; stock_status: Stock };

// Dua jenis pending: menunggu Robux masuk (5 hari) dan masa tunggu setelah
// Robux akun terpakai habis (30 hari). Masa tunggu dihitung dari status_until.
const STATUS: Record<Stock, { label: string; tone: string; hint: string; days?: number }> = {
  ready: { label: "Ready", tone: "text-ready", hint: "Siap dijual" },
  pending: { label: "Pending 5 hari", tone: "text-pending", hint: "Menunggu Robux masuk", days: 5 },
  cooldown: { label: "Pending 30 hari", tone: "text-cooldown", hint: "Robux sudah terpakai habis", days: 30 },
  borrow: { label: "Borrow", tone: "text-borrow", hint: "Sedang dipinjam" },
};

// Teks + warna sisa masa tunggu satu akun; hijau bila masa tunggunya sudah lewat.
const waitInfo = (a: Account) => {
  const d = daysLeft(a.status_until);
  if (d === null) return null;
  return d > 0
    ? { text: `sisa ${d} hari`, tone: "text-muted" }
    : { text: "masa tunggu selesai", tone: "text-ready" };
};
const STATUS_KEYS = Object.keys(STATUS) as Stock[];

const empty: Form = { username_roblox: "", robux: 0, stock_status: "ready" };

// Isian akun dipakai dua kali: panel "Tambah Akun" di samping daftar, dan popup "Edit Akun".
function AccountFields({ form, setForm }: { form: Form; setForm: (f: Form) => void }) {
  return (
    <>
      <label className="block space-y-2">
        <span className="text-sm font-medium">Username Roblox</span>
        <input
          required
          placeholder="contoh: irfan_rbx"
          value={form.username_roblox}
          onChange={(e) => setForm({ ...form, username_roblox: e.target.value })}
          className="input h-11 text-base"
        />
      </label>

      <label className="block space-y-2">
        <span className="text-sm font-medium">Jumlah Robux</span>
        <div className="relative">
          <NumberInput
            placeholder="contoh: 10.000"
            value={form.robux}
            onValue={(n) => setForm({ ...form, robux: n })}
            className="input h-11 pr-12 text-base"
          />
          <span className="pointer-events-none absolute inset-y-0 right-3 flex items-center text-sm text-muted">R</span>
        </div>
      </label>

      <div className="space-y-2">
        <span className="text-sm font-medium">Status</span>
        <div className="grid grid-cols-2 gap-2">
          {STATUS_KEYS.map((s) => (
            <button
              key={s}
              type="button"
              onClick={() => setForm({ ...form, stock_status: s })}
              className={`h-11 rounded-md border text-sm font-medium transition-colors ${
                form.stock_status === s ? `border-fg bg-hover ${STATUS[s].tone}` : "border-line text-muted hover:text-fg"
              }`}
            >
              {STATUS[s].label}
            </button>
          ))}
        </div>
        <p className="text-sm text-muted">
          {STATUS[form.stock_status].hint}
          {STATUS[form.stock_status].days ? ` · masa tunggu ${STATUS[form.stock_status].days} hari` : ""}
        </p>
      </div>
    </>
  );
}

export default function AccountsPage() {
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [form, setForm] = useState<Form>(empty);
  const [editing, setEditing] = useState<Account | null>(null);
  const [editForm, setEditForm] = useState<Form>(empty);
  const [query, setQuery] = useState("");
  const [error, setError] = useState("");
  const [editError, setEditError] = useState("");
  const [busy, setBusy] = useState(false);
  const [editBusy, setEditBusy] = useState(false);

  const load = async () => setAccounts(await api<Account[]>("/api/accounts?all=1"));
  useEffect(() => {
    api<Account[]>("/api/accounts?all=1").then(setAccounts).catch((e) => setError(errMsg(e)));
  }, []);

  // Kirim data akun lengkap ke server. "Jumlah Robux" yang diisi user adalah saldo SEKARANG,
  // jadi saldo awal disesuaikan supaya riwayat transaksi tetap dihitung.
  const save = (a: Account | null, patch: Form & { active?: boolean }) => {
    const moved = a ? a.current_robux - a.initial_balance_robux : 0;
    const body = {
      name: "",
      username_roblox: patch.username_roblox,
      owner: a?.owner ?? "",
      initial_balance_robux: patch.robux - moved,
      stock_status: patch.stock_status,
      active: patch.active,
    };
    return a
      ? api(`/api/accounts/${a.id}`, { method: "PUT", body: JSON.stringify(body) })
      : api("/api/accounts", { method: "POST", body: JSON.stringify(body) });
  };

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      await save(null, form);
      setForm(empty);
      await load();
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setBusy(false);
    }
  };

  const submitEdit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!editing) return;
    setEditBusy(true);
    setEditError("");
    try {
      await save(editing, editForm);
      setEditing(null);
      await load();
    } catch (e) {
      setEditError(errMsg(e));
    } finally {
      setEditBusy(false);
    }
  };

  const run = async (fn: () => Promise<unknown>) => {
    setError("");
    try {
      await fn();
      await load();
    } catch (e) {
      setError(errMsg(e));
    }
  };

  const changeStatus = (a: Account, s: Stock) =>
    run(() => save(a, { username_roblox: a.username_roblox || a.name, robux: a.current_robux, stock_status: s }));

  const remove = (a: Account) => {
    const name = a.username_roblox || a.name;
    if (!confirm(`Hapus permanen akun "${name}"?\n\nSemua transaksi akun ini ikut terhapus dan tidak bisa dikembalikan.`)) return;
    setEditing(null);
    run(() => api(`/api/accounts/${a.id}?permanent=1`, { method: "DELETE" }));
  };

  const startEdit = (a: Account) => {
    setEditError("");
    setEditing(a);
    setEditForm({ username_roblox: a.username_roblox || a.name, robux: a.current_robux, stock_status: a.stock_status });
  };

  const active = accounts.filter((a) => a.active);
  const shown = active.filter((a) => (a.username_roblox || a.name).toLowerCase().includes(query.trim().toLowerCase()));
  const total = active.reduce((s, a) => s + a.current_robux, 0);
  const count = (s: Stock) => active.filter((a) => a.stock_status === s).length;
  const robuxOf = (s: Stock) => active.filter((a) => a.stock_status === s).reduce((t, a) => t + a.current_robux, 0);

  return (
    <div className="space-y-8">
      {/* ringkasan stok */}
      <div className="grid grid-cols-2 gap-4 lg:grid-cols-5">
        <div className="card p-6">
          <p className="text-sm text-muted">Total Robux</p>
          <p className="mt-2 text-3xl font-semibold tracking-tight">{rb(total)}</p>
          <p className="mt-1 text-sm text-muted">{active.length} akun aktif</p>
        </div>
        {STATUS_KEYS.map((s) => (
          <div key={s} className="card p-6">
            <p className={`badge text-sm ${STATUS[s].tone}`}>{STATUS[s].label}</p>
            <p className="mt-2 text-3xl font-semibold tracking-tight">{rb(robuxOf(s))}</p>
            <p className="mt-1 text-sm text-muted">{count(s)} akun · {STATUS[s].hint}</p>
            {STATUS[s].days && <p className="text-sm text-muted">Masa tunggu {STATUS[s].days} hari</p>}
          </div>
        ))}
      </div>

      <div className="grid gap-6 xl:grid-cols-[1fr_380px]">
        {/* daftar akun */}
        <div className="card overflow-hidden">
          <div className="flex flex-wrap items-center justify-between gap-3 border-b border-line px-6 py-4">
            <h2 className="text-base font-medium">Daftar Akun</h2>
            <input
              placeholder="Cari username…"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              className="input h-10 w-full text-base sm:w-64"
            />
          </div>
          <div className="overflow-x-auto">
            <table className="tbl text-base">
              <thead>
                <tr>
                  <th className="px-6 py-3">Username Roblox</th>
                  <th className="px-6 py-3 text-right">Jumlah Robux</th>
                  <th className="px-6 py-3">Status</th>
                  <th className="px-6 py-3"></th>
                </tr>
              </thead>
              <tbody>
                {shown.length === 0 && (
                  <tr>
                    <td colSpan={4} className="px-6 py-12 text-center text-muted">
                      {active.length === 0 && !query ? "Belum ada akun. Tambahkan lewat form di samping." : "Tidak ada yang cocok."}
                    </td>
                  </tr>
                )}
                {shown.map((a) => (
                  <tr key={a.id}>
                    <td className="px-6 py-4 font-medium">{a.username_roblox || a.name}</td>
                    <td className="px-6 py-4 text-right text-lg font-semibold tabular-nums">{rb(a.current_robux)}</td>
                    <td className="px-6 py-4">
                      <select
                        value={a.stock_status}
                        onChange={(e) => changeStatus(a, e.target.value as Stock)}
                        className={`input h-10 w-40 text-base font-medium ${STATUS[a.stock_status].tone}`}
                      >
                        {STATUS_KEYS.map((s) => (
                          <option key={s} value={s} className="text-fg">{STATUS[s].label}</option>
                        ))}
                      </select>
                      {(() => {
                        const w = waitInfo(a);
                        return w && <p className={`mt-1 text-xs ${w.tone}`}>{w.text}</p>;
                      })()}
                    </td>
                    <td className="whitespace-nowrap px-6 py-4 text-right">
                      <button onClick={() => startEdit(a)} className="link text-base">Edit</button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>

        {/* form tambah */}
        <form onSubmit={submit} className="card h-fit space-y-5 p-6">
          <div>
            <h2 className="text-lg font-semibold">Tambah Akun</h2>
            <p className="text-sm text-muted">Masukkan akun Roblox baru</p>
          </div>

          <AccountFields form={form} setForm={setForm} />

          {error && <p className="text-sm text-neg">{error}</p>}
          <button disabled={busy} className="btn-primary h-11 w-full text-base">
            {busy ? "Menyimpan…" : "Tambah Akun"}
          </button>
        </form>
      </div>

      {/* popup edit */}
      <Modal open={!!editing} title="Edit Akun" onClose={() => setEditing(null)}>
        {editing && (
          <form onSubmit={submitEdit} className="space-y-5">
            <p className="-mt-2 text-sm text-muted">{editing.username_roblox || editing.name}</p>

            <AccountFields form={editForm} setForm={setEditForm} />

            {editError && <p className="text-sm text-neg">{editError}</p>}
            <div className="flex gap-2 pt-1">
              <button disabled={editBusy} className="btn-primary h-11 flex-1 text-base">
                {editBusy ? "Menyimpan…" : "Simpan"}
              </button>
              <button type="button" onClick={() => setEditing(null)} className="btn-ghost h-11 text-base">
                Batal
              </button>
            </div>
            <div className="border-t border-line pt-5">
              <button
                type="button"
                onClick={() => remove(editing)}
                className="h-11 w-full rounded-md border border-line text-base font-medium text-muted transition-colors hover:border-fg hover:text-fg"
              >
                Hapus akun ini
              </button>
              <p className="mt-2 text-center text-xs text-muted">Transaksi akun ini ikut terhapus permanen</p>
            </div>
          </form>
        )}
      </Modal>
    </div>
  );
}
