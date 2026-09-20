# Deploy Harbux

Susunan yang dipakai: **web di Vercel, API + database di server Debian sendiri.**

```
Browser → Vercel (Next.js, HTTPS)
             ↓  diteruskan dari sisi server
          Nginx (443, HTTPS) → API Go (127.0.0.1:8080) → MariaDB
```

Browser hanya pernah berbicara dengan domain Vercel. Next.js yang meneruskan
`/api/*` dan `/auth/*` ke server API. Karena itu cookie login tetap satu domain
dan **CORS tidak diperlukan sama sekali**.

Kamu butuh **dua domain**:

| Untuk | Contoh | Diarahkan ke |
|---|---|---|
| Web | `harbux.com` | Vercel |
| API | `api.harbux.com` | IP server Debian (A record) |

Jalankan perintah server sebagai root (atau pakai `sudo`).

---

# BAGIAN A — Server (API + database)

## A1. Paket yang dibutuhkan

```bash
apt update
apt install -y mariadb-server nginx git curl ca-certificates
```

Amankan MariaDB (buat sandi root, buang akun anonim):

```bash
mariadb-secure-installation
```

## A2. Database

```bash
mariadb -u root -p
```

```sql
CREATE DATABASE harbux CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER 'harbux'@'localhost' IDENTIFIED BY 'GANTI_SANDI';
GRANT ALL PRIVILEGES ON harbux.* TO 'harbux'@'localhost';
FLUSH PRIVILEGES;
EXIT;
```

## A3. Pengguna sistem & kode aplikasi

```bash
adduser --system --group --home /opt/harbux harbux
git clone https://github.com/Harbux-Store/Harbux_Pantau-.git /opt/harbux
chown -R harbux:harbux /opt/harbux
```

## A4. Build API (Go)

```bash
apt install -y golang-go
cd /opt/harbux/api
go build -o harbux-api .
```

Atau build dari laptop Windows (PowerShell) lalu salin dengan `scp`:

```powershell
$env:GOOS="linux"; $env:GOARCH="amd64"; go build -o harbux-api .
```

## A5. Konfigurasi API

```bash
cd /opt/harbux/api
cp .env.example .env
openssl rand -base64 48        # salin hasilnya ke HARBUX_SECRET
nano .env
```

Isi `.env`:

```
HARBUX_DSN=harbux:GANTI_SANDI@tcp(127.0.0.1:3306)/harbux?parseTime=true&charset=utf8mb4&loc=Asia%2FJakarta
HARBUX_SECRET=<hasil openssl tadi>
HARBUX_SECURE_COOKIE=0
HARBUX_TRUST_PROXY=1
PORT=8080
```

`HARBUX_SECURE_COOKIE` sengaja **0 dulu** — dinaikkan ke 1 di langkah A8,
setelah HTTPS benar-benar jalan.

```bash
chown harbux:harbux .env && chmod 600 .env
mkdir -p data && chown harbux:harbux data
```

## A6. Jalankan otomatis (systemd)

```bash
cp /opt/harbux/deploy/harbux-api.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable --now harbux-api
systemctl status harbux-api
curl http://127.0.0.1:8080/healthz      # {"status":"ok"}
```

API sengaja hanya mendengar di `127.0.0.1`, jadi dari internet ia tidak
terjangkau langsung — yang menghadap publik cuma Nginx.

## A7. Nginx

Tambahkan baris ini di dalam blok `http { }` pada `/etc/nginx/nginx.conf`:

```
limit_req_zone $binary_remote_addr zone=harbux_login:10m rate=10r/m;
```

Lalu:

```bash
cp /opt/harbux/deploy/nginx-harbux.conf /etc/nginx/sites-available/harbux
sed -i 's/GANTI_DOMAIN_API/api.domain-kamu.com/' /etc/nginx/sites-available/harbux
ln -s /etc/nginx/sites-available/harbux /etc/nginx/sites-enabled/
rm -f /etc/nginx/sites-enabled/default
nginx -t && systemctl reload nginx
```

## A8. HTTPS (wajib, bukan opsional)

Vercel hanya melayani HTTPS. Kalau API masih HTTP, browser akan menolak
seluruh permintaan sebagai *mixed content*.

```bash
apt install -y certbot python3-certbot-nginx
certbot --nginx -d api.domain-kamu.com
curl https://api.domain-kamu.com/healthz     # {"status":"ok"}
```

Setelah `curl` di atas berhasil, **baru** naikkan cookie ke mode aman:

```bash
sed -i 's/^HARBUX_SECURE_COOKIE=0/HARBUX_SECURE_COOKIE=1/' /opt/harbux/api/.env
systemctl restart harbux-api
```

## A9. Firewall

```bash
apt install -y ufw
ufw allow OpenSSH
ufw allow 'Nginx Full'
ufw enable
```

---

# BAGIAN B — Web (Vercel)

## B1. Hubungkan repo

Di [vercel.com](https://vercel.com) → **Add New Project** → pilih repo
`Harbux_Pantau-`.

Isi **Root Directory** dengan `web` (penting — kode Next.js ada di subfolder,
bukan di akar repo). Framework akan terdeteksi sendiri sebagai Next.js.

## B2. Environment Variable

Di **Settings → Environment Variables**, tambahkan:

| Name | Value | Environment |
|---|---|---|
| `API_ORIGIN` | `https://api.domain-kamu.com` | Production, Preview, Development |

Wajib `https://`, dan **tanpa garis miring di akhir**.

Selesaikan Bagian A sampai A8 lebih dulu. Kalau `API_ORIGIN` menunjuk ke
alamat yang belum hidup, build tetap sukses tetapi semua halaman gagal
memuat data.

## B3. Deploy & domain

Klik **Deploy**. Setelah selesai, **Settings → Domains** → tambahkan
`harbux.com` dan ikuti petunjuk DNS dari Vercel.

Setiap `git push` ke `main` akan men-deploy ulang secara otomatis.

## B4. Admin pertama

Buka `https://harbux.com`. Karena database masih kosong, halaman login
otomatis berubah menjadi **Buat Admin Pertama**. Isi username dan password
(minimal 8 karakter), lalu kamu langsung masuk.

Setelah itu, tambah pengguna lain lewat menu **Users**. Tiap pengguna hanya
melihat akun dan transaksi miliknya sendiri; admin melihat semuanya.

---

# BAGIAN C — Perawatan

## C1. Backup harian

```bash
cp /opt/harbux/deploy/backup-harbux.sh /usr/local/bin/backup-harbux
chmod +x /usr/local/bin/backup-harbux
nano /root/.my.cnf     # isi user & password MySQL (lihat komentar di skrip)
chmod 600 /root/.my.cnf
crontab -e             # tambahkan:  0 2 * * * /usr/local/bin/backup-harbux
```

## C2. Memperbarui aplikasi

Web di Vercel ikut ter-update sendiri setiap `git push`. Yang manual cuma API:

```bash
cd /opt/harbux
git pull
cd api && go build -o harbux-api .
chown -R harbux:harbux /opt/harbux
systemctl restart harbux-api
```

## C3. Memindahkan data lama dari laptop

Kalau data di laptop (SQLite) mau dibawa ke server:

```bash
# salin cashflow.db dari laptop ke server, lalu:
cd /opt/harbux/api
go run ./cmd/import-sqlite -db /tmp/cashflow.db \
  -dsn "harbux:GANTI_SANDI@tcp(127.0.0.1:3306)/harbux"
```

Jalankan **setelah** API pernah dinyalakan sekali (langkah A6), supaya
tabelnya sudah terbentuk.

---

# Kalau ada masalah

```bash
journalctl -u harbux-api -n 50 --no-pager     # log API
systemctl status harbux-api
curl https://api.domain-kamu.com/healthz
```

Log web ada di dashboard Vercel → tab **Logs**.

| Gejala | Kemungkinan penyebab |
|---|---|
| Login berhasil tapi langsung balik ke halaman login | `HARBUX_SECURE_COOKIE=1` padahal API belum HTTPS (A8) |
| Semua halaman kosong / gagal memuat data | `API_ORIGIN` salah, masih `http://`, atau ada garis miring di akhir |
| Build Vercel gagal "No Next.js version detected" | **Root Directory** belum diisi `web` (B1) |
| API mati saat start | DSN MySQL salah, atau `HARBUX_SECRET` kurang dari 32 karakter |
| `curl https://api.../healthz` gagal | certbot belum jalan, atau DNS `api.` belum mengarah ke server |
| "terlalu banyak percobaan login" | Batas 5 kali gagal per 15 menit; tunggu atau restart `harbux-api` |
| Semua pengunjung terhitung satu IP | `HARBUX_TRUST_PROXY=1` belum diisi di `.env` |

---

## Catatan: menjalankan web di server yang sama (tanpa Vercel)

Masih memungkinkan. Pasang Node 22, `npm ci`, build dengan
`API_ORIGIN=http://127.0.0.1:8080 npm run build`, pasang
`deploy/harbux-web.service`, lalu ubah `location /` di Nginx agar
meneruskan ke `http://127.0.0.1:3000` (bukan `return 404`). Dengan cara ini
cukup satu domain dan `HARBUX_TRUST_PROXY=1` tetap berlaku.
