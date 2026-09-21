"use client";

import { useEffect, useState } from "react";
import { api, errMsg, type User } from "@/lib/api";

export default function UsersPage() {
  const [users, setUsers] = useState<User[]>([]);
  const [me, setMe] = useState<User | null>(null);
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [role, setRole] = useState<User["role"]>("staff");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const load = async () => setUsers(await api<User[]>("/api/users"));
  useEffect(() => {
    api<User>("/auth/me").then(setMe).catch(() => {});
    api<User[]>("/api/users").then(setUsers).catch((e) => setError(errMsg(e)));
  }, []);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      await api("/api/users", { method: "POST", body: JSON.stringify({ username, password, role }) });
      setUsername("");
      setPassword("");
      await load();
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setBusy(false);
    }
  };

  const remove = async (u: User) => {
    if (!confirm(`Hapus pengguna "${u.username}"?\n\nSemua akun Roblox dan transaksi miliknya ikut terhapus permanen, dan ia langsung ter-logout.`)) return;
    setError("");
    try {
      await api(`/api/users/${u.id}`, { method: "DELETE" });
      await load();
    } catch (e) {
      setError(errMsg(e));
    }
  };

  const admins = users.filter((u) => u.role === "admin").length;

  return (
    <div className="grid gap-6 xl:grid-cols-[1fr_380px]">
      <div className="card overflow-hidden">
        <div className="border-b border-line px-6 py-4">
          <h2 className="text-base font-medium">Daftar Pengguna</h2>
          <p className="text-sm text-muted">{users.length} pengguna · {admins} admin</p>
        </div>
        <div className="overflow-x-auto">
          <table className="tbl text-base">
            <thead>
              <tr>
                <th className="px-6 py-3">Username</th>
                <th className="px-6 py-3">Role</th>
                <th className="px-6 py-3"></th>
              </tr>
            </thead>
            <tbody>
              {users.map((u) => {
                const self = u.id === me?.id;
                const lastAdmin = u.role === "admin" && admins <= 1;
                return (
                  <tr key={u.id}>
                    <td className="px-6 py-4">
                      <div className="flex items-center gap-3">
                        <span className="flex size-10 shrink-0 items-center justify-center rounded-full bg-hover text-sm font-semibold uppercase">
                          {u.username.slice(0, 2)}
                        </span>
                        <span className="font-medium">
                          {u.username}
                          {self && <span className="ml-2 text-sm font-normal text-muted">(kamu)</span>}
                        </span>
                      </div>
                    </td>
                    <td className="px-6 py-4">
                      <span className={`badge text-sm ${u.role === "admin" ? "text-fg" : ""}`}>{u.role === "admin" ? "Admin" : "Staff"}</span>
                    </td>
                    <td className="px-6 py-4 text-right">
                      {self || lastAdmin ? (
                        <span className="text-sm text-muted">{self ? "—" : "admin terakhir"}</span>
                      ) : (
                        <button onClick={() => remove(u)} className="link text-base text-neg hover:text-neg">Hapus</button>
                      )}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      </div>

      <form onSubmit={submit} className="card h-fit space-y-5 p-6">
        <div>
          <h2 className="text-lg font-semibold">Tambah Pengguna</h2>
          <p className="text-sm text-muted">Setiap pengguna punya data akun & transaksi sendiri</p>
        </div>
        <label className="block space-y-2">
          <span className="text-sm font-medium">Username</span>
          <input required value={username} onChange={(e) => setUsername(e.target.value)} className="input h-11 text-base" />
        </label>
        <label className="block space-y-2">
          <span className="text-sm font-medium">Password</span>
          <input required type="password" value={password} onChange={(e) => setPassword(e.target.value)} className="input h-11 text-base" />
        </label>
        <div className="space-y-2">
          <span className="text-sm font-medium">Role</span>
          <div className="grid grid-cols-2 gap-2">
            {(["staff", "admin"] as const).map((r) => (
              <button
                key={r}
                type="button"
                onClick={() => setRole(r)}
                className={`h-11 rounded-md border text-sm font-medium transition-colors ${
                  role === r ? "border-fg bg-hover text-fg" : "border-line text-muted hover:text-fg"
                }`}
              >
                {r === "admin" ? "Admin" : "Staff"}
              </button>
            ))}
          </div>
          <p className="text-sm text-muted">
            {role === "admin" ? "Bisa mengelola pengguna; datanya tetap terpisah" : "Hanya mengelola akun & transaksi miliknya"}
          </p>
        </div>
        {error && <p className="text-sm text-neg">{error}</p>}
        <button disabled={busy} className="btn-primary h-11 w-full text-base">{busy ? "Menyimpan…" : "Tambah Pengguna"}</button>
      </form>
    </div>
  );
}
