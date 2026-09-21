-- Skema Harbux Cashflow untuk MySQL / MariaDB.
--
-- Dokumentasi + setup manual. Aplikasi (api/main.go, fungsi schemaFor & migrate)
-- tetap menjalankan DDL yang sama secara otomatis saat start, jadi file ini
-- opsional: gunakan bila ingin menyiapkan database lebih dulu (mis. user DB
-- tidak punya hak CREATE TABLE saat runtime).
--
-- Pakai:
--   mysql -u <user> -p <nama_database> < schema/schema.mysql.sql
--
-- Catatan: jaga file ini tetap sinkron dengan schemaFor(true) di api/main.go.

CREATE TABLE IF NOT EXISTS users (
	id INT AUTO_INCREMENT PRIMARY KEY,
	username VARCHAR(64) NOT NULL UNIQUE,
	pass_hash VARCHAR(255) NOT NULL,
	role VARCHAR(16) NOT NULL DEFAULT 'staff'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS accounts (
	id INT AUTO_INCREMENT PRIMARY KEY,
	name VARCHAR(128) NOT NULL,
	username_roblox VARCHAR(128) NOT NULL DEFAULT '',
	owner VARCHAR(128) NOT NULL DEFAULT '',
	initial_balance_robux BIGINT NOT NULL DEFAULT 0,
	active TINYINT NOT NULL DEFAULT 1,
	stock_status VARCHAR(16) NOT NULL DEFAULT 'ready',
	user_id INT NULL,
	INDEX idx_accounts_user (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- type: 'topup' | 'penjualan' | 'transfer' | 'subscribe' (langganan akun) | lainnya (fee/biaya).
-- status: 'selesai' | 'pending' | 'dibatalkan'; hanya 'selesai' yang dihitung ke saldo.
-- created_at: teks "YYYY-MM-DD HH:MM" (16 karakter), bukan tipe DATETIME.
-- account_id dipakai topup/penjualan; from_account_id & to_account_id dipakai transfer.
CREATE TABLE IF NOT EXISTS transactions (
	id INT AUTO_INCREMENT PRIMARY KEY,
	type VARCHAR(16) NOT NULL,
	account_id INT NULL,
	from_account_id INT NULL,
	to_account_id INT NULL,
	counterpart VARCHAR(128) NOT NULL DEFAULT '',
	robux_amount BIGINT NOT NULL DEFAULT 0,
	rate_idr BIGINT NOT NULL DEFAULT 0,
	fee_idr BIGINT NOT NULL DEFAULT 0,
	idr_total BIGINT NOT NULL DEFAULT 0,
	status VARCHAR(16) NOT NULL DEFAULT 'selesai',
	note VARCHAR(255) NOT NULL DEFAULT '',
	created_at VARCHAR(16) NOT NULL,
	created_by INT NOT NULL,
	INDEX idx_tx_account (account_id),
	INDEX idx_tx_from (from_account_id),
	INDEX idx_tx_to (to_account_id),
	INDEX idx_tx_date (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
