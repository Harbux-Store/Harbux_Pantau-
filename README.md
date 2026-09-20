# Harbux Pantau

Aplikasi untuk **memantau penjualan Robux dan akun-akun Roblox** kamu. Catat akun dan transaksinya, lalu Harbux menghitung sisa Robux, profit, dan laporan secara otomatis.

---

## Mulai Menggunakan

1. **Login** dengan username dan password yang diberikan admin.
2. Buka menu **Akun**, lalu tambahkan akun Roblox kamu.
3. Buka menu **Transaksi** untuk mencatat pembelian, penjualan, atau transfer Robux.
4. Lihat hasilnya di menu **Ringkasan** dan **Laporan**.

Data setiap pengguna terpisah — kamu hanya melihat akun dan transaksi milikmu sendiri.

> Mau memasang Harbux di server sendiri? Lihat [panduan deploy](deploy/README.md).

---

## Menu

### Ringkasan
Gambaran cepat usaha kamu: profit hari ini, profit 30 hari, total stok Robux, harga beli dan jual rata-rata, serta grafik penjualan.

### Akun
Tempat menyimpan akun-akun Roblox.

- Klik **Tambah Akun**, isi **username Roblox**, **jumlah Robux**, dan **status**.
- Status akun:
  - 🟢 **Ready** — Robux siap dijual
  - 🟡 **Pending** — Robux belum masuk / masih tertahan
  - 🔵 **Borrow** — akun sedang dipinjam
- Status bisa diganti langsung dari tabel.
- Untuk mengubah atau menghapus akun, klik **Edit**.

### Transaksi
Catatan semua keluar-masuk Robux.

- **+ Transaksi baru** untuk mencatat:
  - **Topup** — saat kamu membeli Robux
  - **Penjualan** — saat kamu menjual Robux ke pembeli
  - **Lain** — pengeluaran lain
- **Transfer antar akun** untuk memindahkan Robux dari satu akun ke akun lain.
- Untuk mengubah atau menghapus transaksi, klik **Edit**.

Sisa Robux di setiap akun akan **berubah otomatis** sesuai transaksi yang dicatat.

### Laporan
Pilih rentang tanggal untuk melihat pendapatan dan profit pada periode itu. Klik **Ekspor CSV** untuk membuka datanya di Excel atau Google Sheets.

### Pengguna *(khusus admin)*
Admin dapat menambah dan menghapus pengguna.

---

## Tips

- **Mengetik angka** — gunakan titik sebagai pemisah ribuan. `10.000` dibaca sepuluh ribu.
- **Penjualan ditolak?** Artinya sisa Robux di akun tersebut tidak cukup.
- **Status transaksi *Pending*** tidak dihitung ke saldo sampai diubah menjadi *Selesai*.
- **Mode terang / gelap** bisa diganti dari tombol di sidebar.

---

## Arti Angka di Ringkasan

| Istilah | Artinya |
|---|---|
| **Profit** | Keuntungan bersih dari penjualan (hijau = untung, merah = rugi) |
| **Harga beli rata-rata** | Rata-rata modal kamu untuk 1 Robux |
| **Harga jual rata-rata** | Rata-rata harga kamu menjual 1 Robux |
| **Margin per R** | Selisih harga jual dan harga beli untuk setiap 1 Robux |
| **HPP + biaya** | Modal dari Robux yang sudah terjual, ditambah pengeluaran lain |
