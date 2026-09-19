"use client";

import { useEffect } from "react";

// Popup: di HP muncul dari bawah (bottom sheet), di layar lebar di tengah.
// Tutup dengan tombol ×, klik latar, atau Esc.
export default function Modal({ open, title, onClose, children }: {
  open: boolean;
  title: string;
  onClose: () => void;
  children: React.ReactNode;
}) {
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    const prev = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    window.addEventListener("keydown", onKey);
    return () => {
      document.body.style.overflow = prev;
      window.removeEventListener("keydown", onKey);
    };
  }, [open, onClose]);

  if (!open) return null;
  return (
    <div className="fixed inset-0 z-50 flex items-end justify-center sm:items-center sm:p-4" role="dialog" aria-modal="true" aria-label={title}>
      <div className="absolute inset-0 bg-black/50 backdrop-blur-[2px]" onClick={onClose} />
      <div className="relative max-h-[92vh] w-full overflow-y-auto rounded-t-2xl border border-line bg-bg p-5 shadow-2xl sm:max-w-md sm:rounded-2xl sm:p-6">
        <div className="mx-auto mb-4 h-1 w-10 rounded-full bg-line sm:hidden" />
        <div className="mb-5 flex items-center justify-between gap-4">
          <h2 className="text-lg font-semibold">{title}</h2>
          <button onClick={onClose} aria-label="Tutup" className="-mr-2 rounded-md p-2 text-xl leading-none text-muted hover:bg-hover hover:text-fg">×</button>
        </div>
        {children}
      </div>
    </div>
  );
}
