# Harbux Pantau

Aplikasi web untuk **memantau penjualan Robux dan akun-akun Roblox**. Setiap pengguna mencatat akun Roblox miliknya, stok Robux di tiap akun, serta transaksi beli/jual/transfer — lalu aplikasi menghitung saldo, profit, dan laporan secara otomatis.

---

## Fitur

### Ringkasan (Dashboard)
- Profit **hari ini**, **30 hari terakhir**, dan **keseluruhan**.
- Total stok Robux semua akun.
- Harga beli rata-rata, harga jual rata-rata, dan **margin per R**.
- Grafik penjualan 30 hari terakhir (hijau = untung, merah = rugi).
- Penjualan per akun dan daftar pembeli teratas.
- Peringatan saldo akun menipis dan transaksi yang masih *pending*.

### Akun Roblox
- Tambah akun cukup dengan **username Roblox**, **jumlah Robux**, dan **status**:
  - 🟢 **Ready** — siap dijual
  - 🟡 **Pending** — Robux belum masuk / tertahan
  - 🔵 **Borrow** — sedang dipinjam
- Status bisa diganti langsung dari tabel.
- Kartu ringkasan jumlah Robux per status.
- Edit dan hapus akun (hapus ada di dalam form Edit).

### Transaksi
- Tipe transaksi: **Topup** (beli Robux), **Penjualan**, dan **Lain** (pengeluaran).
- **Transfer antar akun** milik sendiri.
- Form tampil sebagai **popup** (di HP muncul dari bawah).
- Filter berdasarkan tipe, akun, status, dan rentang tanggal.
- Input angka format Indonesia — `10.000` terbaca sepuluh ribu.
- Penjualan dan transfer **ditolak bila saldo akun tidak cukup**.

### Laporan
- Ringkasan pendapatan, modal, dan profit untuk rentang tanggal tertentu.
- **Ekspor CSV** untuk dibuka di Excel / Google Sheets.

### Pengguna
- Dua peran: **Admin** dan **Staff**.
- **Data tiap pengguna terpisah** — pengguna hanya melihat akun & transaksi miliknya.
- Admin dapat melihat data semua pengguna, menambah pengguna, dan menghapus pengguna.

### Tampilan
- **Mode terang** bergaya situs Roblox dan **mode gelap** hitam pekat — bisa diganti dari sidebar.
- Responsif untuk HP (sidebar berubah menjadi menu).

---

## Cara Menghitung

**Saldo akun**
```
Saldo = Saldo awal + Topup + Transfer masuk − Penjualan − Transfer keluar
```
Hanya transaksi berstatus *selesai* yang dihitung.

**Profit**
```
Profit = Pendapatan − (HPP + Biaya)
HPP    = Robux terjual × Harga beli rata-rata
```
- **Pendapatan**: total uang dari penjualan.
- **HPP (Harga Pokok Penjualan)**: modal dari Robux yang *sudah terjual*. Uang topup tidak langsung dianggap rugi karena masih berupa stok Robux.
- **Biaya**: pengeluaran lain (transaksi tipe *Lain*).

**Margin per R**
```
Margin per R = Harga jual rata-rata − Harga beli rata-rata
```
Keuntungan rata-rata untuk setiap 1 Robux yang terjual.

---

## Teknologi

| Bagian | Teknologi |
|---|---|
| Backend (API) | Go, SQLite, JWT (login via cookie), bcrypt |
| Frontend (Web) | Next.js, React, TypeScript, Tailwind CSS |

### Struktur Folder
```
Harbux/
├── api/                 # Backend Go
│   ├── main.go          # Seluruh API: login, akun, transaksi, ringkasan, laporan
│   ├── main_test.go     # Pengujian otomatis
│   └── data/            # Database SQLite (dibuat otomatis)
└── web/                 # Frontend Next.js
    └── src/
        ├── app/
        │   ├── login/           # Halaman login
        │   └── (app)/
        │       ├── dashboard/   # Ringkasan
        │       ├── transactions/# Transaksi
        │       ├── accounts/    # Akun Roblox
        │       ├── reports/     # Laporan
        │       └── users/       # Pengguna (khusus admin)
        ├── components/          # Ikon, popup, input angka
        └── lib/api.ts           # Tipe data & fungsi pemanggil API
```

---

## Hak Akses

| Aksi | Staff | Admin |
|---|---|---|
| Kelola akun Roblox & transaksi | ✅ milik sendiri | ✅ semua |
| Lihat ringkasan & laporan | ✅ milik sendiri | ✅ semua |
| Kelola pengguna | ❌ | ✅ |

Semua pembatasan dicek di server, bukan hanya di tampilan.
