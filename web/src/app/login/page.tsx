"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { api, errMsg } from "@/lib/api";
import { Logo } from "@/components/icons";

export default function LoginPage() {
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  // null = belum diketahui, true = database masih kosong (buat admin pertama)
  const [setup, setSetup] = useState<boolean | null>(null);
  const router = useRouter();

  useEffect(() => {
    api<{ setup_needed: boolean }>("/auth/setup")
      .then((d) => setSetup(d.setup_needed))
      .catch(() => setSetup(false));
  }, []);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      await api(setup ? "/auth/register" : "/auth/login", {
        method: "POST",
        body: JSON.stringify({ username, password }),
      });
      router.replace("/dashboard");
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setBusy(false);
    }
  };

  if (setup === null) return <div className="p-8 text-sm text-muted">Memuat…</div>;

  return (
    <div className="ornament relative flex min-h-screen items-center justify-center px-4">
      <form onSubmit={submit} className="card relative w-full space-y-3 p-8 shadow-sm" style={{ maxWidth: 380 }}>
        <div className="mb-8">
          <Logo className="mb-5 size-9" />
          <h1 className="text-xl font-semibold tracking-tight">{setup ? "Buat Admin Pertama" : "Harbux"}</h1>
          <p className="text-sm text-muted">
            {setup ? "Belum ada pengguna. Buat akun admin untuk mulai." : "Masuk untuk melanjutkan"}
          </p>
        </div>
        <input
          required
          minLength={3}
          value={username}
          onChange={(e) => setUsername(e.target.value)}
          placeholder="Username"
          autoComplete="username"
          className="input"
        />
        <input
          required
          minLength={setup ? 8 : undefined}
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          placeholder={setup ? "Password (min. 8 karakter)" : "Password"}
          autoComplete={setup ? "new-password" : "current-password"}
          className="input"
        />
        {error && <p className="text-sm text-neg">{error}</p>}
        <button disabled={busy} className="btn-primary w-full">
          {busy ? "Memproses…" : setup ? "Buat Admin & Masuk" : "Masuk"}
        </button>
      </form>
    </div>
  );
}
