"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { api, errMsg } from "@/lib/api";
import { Logo } from "@/components/icons";

export default function LoginPage() {
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const router = useRouter();

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      await api("/auth/login", { method: "POST", body: JSON.stringify({ username, password }) });
      router.replace("/dashboard");
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="ornament relative flex min-h-screen items-center justify-center px-4">
      <form onSubmit={submit} className="card relative w-full space-y-3 p-8 shadow-sm" style={{ maxWidth: 380 }}>
        <div className="mb-8">
          <Logo className="mb-5 size-9" />
          <h1 className="text-xl font-semibold tracking-tight">Harbux</h1>
          <p className="text-sm text-muted">Masuk untuk melanjutkan</p>
        </div>
        <input value={username} onChange={(e) => setUsername(e.target.value)} placeholder="Username" autoComplete="username" className="input" />
        <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} placeholder="Password" autoComplete="current-password" className="input" />
        {error && <p className="text-sm text-neg">{error}</p>}
        <button disabled={busy} className="btn-primary w-full">{busy ? "Masuk…" : "Masuk"}</button>
      </form>
    </div>
  );
}
