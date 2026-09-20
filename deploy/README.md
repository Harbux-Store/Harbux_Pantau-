# Deploy Harbux di Debian

Panduan pasang Harbux di server Debian dengan MySQL/MariaDB, Nginx, dan HTTPS.
Jalankan sebagai root (atau pakai `sudo`). Ganti `GANTI_DOMAIN` dengan domain kamu.

Hasil akhir:

```
Internet → Nginx (443, HTTPS) → Next.js (3000) → API Go (8080) → MariaDB
```

---

## 1. Paket yang dibutuhkan

```bash
apt update
apt install -y mariadb-server nginx git curl ca-certificates
curl -fsSL https://deb.nodesource.com/setup_22.x | bash -
apt install -y nodejs
```

Amankan MariaDB (buat sandi root, buang akun anonim):

```bash
mariadb-secure-installation
```

## 2. Database

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

## 3. Pengguna sistem & kode aplikasi

```bash
adduser --system --group --home /opt/harbux harbux
git clone https://github.com/Harbux-Store/Harbux_Pantau-.git /opt/harbux
chown -R harbux:harbux /opt/harbux
```

## 4. Build API (Go)

Build bisa dilakukan **di server** (butuh Go) atau **di laptop** lalu hasilnya disalin.

Di server:

```bash
apt install -y golang-go
cd /opt/harbux/api
go build -o harbux-api .
```

Atau dari laptop Windows (PowerShell), lalu salin dengan `scp`:

```powershell
$env:GOOS="linux"; $env:GOARCH="amd64"; go build -o harbux-api .
```

## 5. Konfigurasi API

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
HARBUX_SECURE_COOKIE=1
PORT=8080
```

```bash
chown harbux:harbux .env && chmod 600 .env
mkdir -p data && chown harbux:harbux data
```

## 6. Build Web (Next.js)

```bash
cd /opt/harbux/web
npm ci
API_ORIGIN=http://127.0.0.1:8080 npm run build
chown -R harbux:harbux /opt/harbux/web
```

## 7. Jalankan otomatis (systemd)

```bash
cp /opt/harbux/deploy/harbux-api.service /etc/systemd/system/
cp /opt/harbux/deploy/harbux-web.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable --now harbux-api harbux-web
systemctl status harbux-api harbux-web
```

Cek API hidup:

```bash
curl http://127.0.0.1:8080/healthz      # {"status":"ok"}
```

## 8. Nginx

Tambahkan baris ini di dalam blok `http { }` pada `/etc/nginx/nginx.conf`:

```
limit_req_zone $binary_remote_addr zone=harbux_login:10m rate=10r/m;
```

Lalu:

```bash
cp /opt/harbux/deploy/nginx-harbux.conf /etc/nginx/sites-available/harbux
sed -i 's/GANTI_DOMAIN/domain-kamu.com/' /etc/nginx/sites-available/harbux
ln -s /etc/nginx/sites-available/harbux /etc/nginx/sites-enabled/
rm -f /etc/nginx/sites-enabled/default
nginx -t && systemctl reload nginx
```

## 9. HTTPS (Let's Encrypt)

Pastikan domain sudah diarahkan ke IP server, lalu:

```bash
apt install -y certbot python3-certbot-nginx
certbot --nginx -d domain-kamu.com
```

Sertifikat diperpanjang otomatis. **`HARBUX_SECURE_COOKIE=1` baru boleh aktif setelah HTTPS jalan**, karena tanpa HTTPS cookie login tidak akan terkirim.

## 10. Admin pertama

Buka `https://domain-kamu.com`. Karena database masih kosong, halaman login otomatis berubah menjadi **Buat Admin Pertama**. Isi username dan password (minimal 8 karakter), lalu kamu langsung masuk.

## 11. Backup harian

```bash
cp /opt/harbux/deploy/backup-harbux.sh /usr/local/bin/backup-harbux
chmod +x /usr/local/bin/backup-harbux
nano /root/.my.cnf     # isi user & password MySQL (lihat komentar di skrip)
chmod 600 /root/.my.cnf
crontab -e             # tambahkan:  0 2 * * * /usr/local/bin/backup-harbux
```

## 12. Firewall (opsional tapi disarankan)

```bash
apt install -y ufw
ufw allow OpenSSH
ufw allow 'Nginx Full'
ufw enable
```

---

## Memindahkan data lama dari laptop

Kalau data di laptop (SQLite) mau dibawa ke server:

```bash
# salin cashflow.db dari laptop ke server, lalu:
cd /opt/harbux/api
go run ./cmd/import-sqlite -db /tmp/cashflow.db \
  -dsn "harbux:GANTI_SANDI@tcp(127.0.0.1:3306)/harbux"
```

Jalankan setelah server pernah dinyalakan sekali, supaya tabelnya sudah terbentuk.

---

## Memperbarui aplikasi

```bash
cd /opt/harbux
git pull
cd api && go build -o harbux-api . && cd ..
cd web && npm ci && API_ORIGIN=http://127.0.0.1:8080 npm run build && cd ..
chown -R harbux:harbux /opt/harbux
systemctl restart harbux-api harbux-web
```

## Kalau ada masalah

```bash
journalctl -u harbux-api -n 50 --no-pager     # log API
journalctl -u harbux-web -n 50 --no-pager     # log web
systemctl status harbux-api harbux-web
curl http://127.0.0.1:8080/healthz
```

| Gejala | Kemungkinan penyebab |
|---|---|
| Tidak bisa login, selalu kembali ke halaman login | `HARBUX_SECURE_COOKIE=1` padahal belum HTTPS |
| API mati saat start | DSN MySQL salah, atau `HARBUX_SECRET` kurang dari 32 karakter |
| Halaman terbuka tapi data gagal dimuat | `harbux-api` mati, atau `API_ORIGIN` salah |
| "terlalu banyak percobaan login" | Batas 5 kali gagal per 15 menit; tunggu atau restart `harbux-api` |
