"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { api, type User } from "@/lib/api";
import { IconAccounts, IconDashboard, IconLogout, IconMenu, IconMoon, IconReports, IconSun, IconTransactions, IconUsers, Logo } from "@/components/icons";

const links = [
  { href: "/dashboard", label: "Ringkasan", icon: IconDashboard },
  { href: "/transactions", label: "Transaksi", icon: IconTransactions },
  { href: "/accounts", label: "Akun", icon: IconAccounts },
  { href: "/reports", label: "Laporan", icon: IconReports },
];

export default function AppLayout({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [open, setOpen] = useState(false);
  const [dark, setDark] = useState(false);
  const router = useRouter();
  const pathname = usePathname();

  useEffect(() => {
    const t = document.documentElement.dataset.theme;
    // eslint-disable-next-line react-hooks/set-state-in-effect -- baca tema dari DOM sekali saat mount
    setDark(t ? t === "dark" : matchMedia("(prefers-color-scheme: dark)").matches);
  }, []);
  const toggleTheme = () => {
    const next = dark ? "light" : "dark";
    document.documentElement.dataset.theme = next;
    try {
      localStorage.setItem("theme", next);
    } catch {}
    setDark(!dark);
  };

  useEffect(() => {
    api<User>("/auth/me").then(setUser).catch(() => router.replace("/login"));
  }, [router]);

  // halaman khusus admin: staff yang membuka lewat URL langsung dialihkan
  const forbidden = !!user && user.role !== "admin" && pathname.startsWith("/users");
  useEffect(() => {
    if (forbidden) router.replace("/dashboard");
  }, [forbidden, router]);

  if (forbidden) return null;
  if (!user) return <div className="p-8 text-sm text-muted">Memuat…</div>;

  const allLinks = user.role === "admin" ? [...links, { href: "/users", label: "Pengguna", icon: IconUsers }] : links;
  const current = allLinks.find((l) => l.href === pathname);

  const logout = async () => {
    await api("/auth/logout", { method: "POST" });
    router.replace("/login");
  };

  const brand = (
    <span className="flex items-center gap-2.5">
      <Logo />
      <span className="leading-tight">
        <span className="block text-sm font-semibold tracking-tight">Harbux</span>
        <span className="block text-[11px] text-muted">Robux monitor</span>
      </span>
    </span>
  );

  return (
    <div className="min-h-screen">
      <header className="sticky top-0 z-20 flex h-14 items-center justify-between border-b border-line bg-bg/85 px-4 backdrop-blur md:hidden">
        {brand}
        <button onClick={() => setOpen(!open)} className="rounded-md p-2 text-muted hover:bg-hover hover:text-fg" aria-label="Menu">
          <IconMenu className="size-5" />
        </button>
      </header>
      {open && <div className="fixed inset-0 z-20 bg-black/30 md:hidden" onClick={() => setOpen(false)} />}

      <aside
        className={`fixed inset-y-0 left-0 z-30 flex w-60 flex-col border-r border-line bg-bg px-3 py-5 transition-transform md:translate-x-0 ${
          open ? "translate-x-0" : "-translate-x-full"
        }`}
      >
        <Link href="/dashboard" className="mb-8 px-2">{brand}</Link>

        <nav className="flex flex-1 flex-col gap-0.5">
          {allLinks.map(({ href, label, icon: Icon }) => {
            const active = pathname === href;
            return (
              <Link
                key={href}
                href={href}
                onClick={() => setOpen(false)}
                className={`relative flex items-center gap-3.5 rounded-lg px-3 py-2.5 text-[15px] font-semibold transition-colors ${
                  active ? "bg-hover text-fg" : "text-fg/85 hover:bg-hover/70 hover:text-fg"
                }`}
              >
                <Icon className="size-5 shrink-0" />
                {label}
              </Link>
            );
          })}
        </nav>

        <button
          onClick={toggleTheme}
          className="mb-3 flex items-center gap-3.5 rounded-lg px-3 py-2.5 text-[15px] font-semibold text-fg/85 transition-colors hover:bg-hover/70 hover:text-fg"
        >
          {dark ? <IconSun className="size-5" /> : <IconMoon className="size-5" />}
          {dark ? "Mode terang" : "Mode gelap"}
        </button>
        <div className="flex items-center gap-3 rounded-lg border border-line bg-surface p-2.5">
          <span className="flex size-8 shrink-0 items-center justify-center rounded-full bg-fg text-xs font-semibold uppercase text-bg">
            {user.username.slice(0, 2)}
          </span>
          <span className="min-w-0 flex-1 leading-tight">
            <span className="block truncate text-sm font-medium">{user.username}</span>
            <span className="block text-xs capitalize text-muted">{user.role}</span>
          </span>
          <button onClick={logout} title="Keluar" aria-label="Keluar" className="rounded-md p-1.5 text-muted hover:bg-hover hover:text-neg">
            <IconLogout />
          </button>
        </div>
      </aside>

      <main className="ornament relative px-4 py-8 sm:px-6 md:ml-60 md:px-10">
        <div className="relative mx-auto max-w-6xl">
          {current && (
            <div className="mb-8 flex items-center gap-3">
              <span className="flex size-9 items-center justify-center rounded-lg border border-line bg-surface text-muted">
                <current.icon className="size-[18px]" />
              </span>
              <h1 className="text-3xl font-bold tracking-tight">{current.label}</h1>
            </div>
          )}
          {children}
        </div>
      </main>
    </div>
  );
}
