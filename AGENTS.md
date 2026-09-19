# Harbux — Panduan untuk agent AI

## Konvensi commit

- Satu commit = satu penambahan/perubahan logis (1 fitur, 1 perbaikan, 1 refactor).
  Contoh: "Tambah edit transaksi", "Perbaiki null pada daftar akun" — bukan "update banyak hal".
- Nama file/lint/format tidak boleh disatukan ke fitur; pisahkan bila berdiri sendiri.
- Jangan `git push` sebelum semua commit yang berhubungan dites (`go test`, `npm run lint`, `npm run build`).
- Setelah selesai mengerjakan 1 unit perubahan: `git add <file>` (file spesifik, bukan `-A` bila ada sisa tak terkait) → `git commit -m "..."` → tanya user sebelum push bila ragu.
- Pastikan `.gitignore` tetap mengecualikan `api/data/`, binary `*.exe`, `node_modules/`, `.next/`, `.env*`.

## Cara menjalankan

- Backend: `api/cashflow-server.exe` (port 8080, DB di `api/data/cashflow.db`, secret via `HARBUX_SECRET`).
- Frontend: `cd web && npm run dev` (port 3000, rewrite `/api/*` & `/auth/*` ke 8080).
- Akun admin awal: `admin` / `admin123`.