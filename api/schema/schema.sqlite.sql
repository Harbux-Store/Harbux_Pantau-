-- Skema Harbux Cashflow untuk SQLite (api/data/cashflow.db).
--
-- Dokumentasi + setup manual; aplikasi menjalankan DDL yang sama saat start
-- (api/main.go, schemaFor(false) + migrate). Kolom active/stock_status/user_id
-- di bawah ditambahkan migrate() lewat ALTER TABLE pada DB lama — di sini
-- sudah langsung disertakan agar DB baru identik dengan hasil migrasi.
--
-- Pakai:
--   sqlite3 data/cashflow.db < schema/schema.sqlite.sql
--
-- Catatan: jaga file ini tetap sinkron dengan schemaFor(false) di api/main.go.

CREATE TABLE IF NOT EXISTS users (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	username TEXT NOT NULL UNIQUE,
	pass_hash TEXT NOT NULL,
	role TEXT NOT NULL DEFAULT 'staff'
);

CREATE TABLE IF NOT EXISTS accounts (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL,
	username_roblox TEXT NOT NULL DEFAULT '',
	owner TEXT NOT NULL DEFAULT '',
	initial_balance_robux INTEGER NOT NULL DEFAULT 0,
	active INTEGER NOT NULL DEFAULT 1,
	stock_status TEXT NOT NULL DEFAULT 'ready',
	user_id INTEGER
);

CREATE INDEX IF NOT EXISTS idx_accounts_user ON accounts (user_id);

-- type: 'topup' | 'penjualan' | 'transfer' | lainnya (fee/biaya).
-- status: 'selesai' | 'pending' | 'dibatalkan'; hanya 'selesai' yang dihitung ke saldo.
-- created_at: teks "YYYY-MM-DD HH:MM" (16 karakter).
-- account_id dipakai topup/penjualan; from_account_id & to_account_id dipakai transfer.
CREATE TABLE IF NOT EXISTS transactions (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	type TEXT NOT NULL,
	account_id INTEGER,
	from_account_id INTEGER,
	to_account_id INTEGER,
	counterpart TEXT NOT NULL DEFAULT '',
	robux_amount INTEGER NOT NULL DEFAULT 0,
	rate_idr INTEGER NOT NULL DEFAULT 0,
	fee_idr INTEGER NOT NULL DEFAULT 0,
	idr_total INTEGER NOT NULL DEFAULT 0,
	status TEXT NOT NULL DEFAULT 'selesai',
	note TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	created_by INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_tx_account ON transactions (account_id);
CREATE INDEX IF NOT EXISTS idx_tx_from ON transactions (from_account_id);
CREATE INDEX IF NOT EXISTS idx_tx_to ON transactions (to_account_id);
CREATE INDEX IF NOT EXISTS idx_tx_date ON transactions (created_at);
