// Pindahkan isi database SQLite lama (cashflow.db) ke MySQL.
//
// Pakai:
//
//	go run ./cmd/import-sqlite -db data/cashflow.db -dsn "harbux:sandi@tcp(127.0.0.1:3306)/harbux"
//
// Tabel tujuan harus sudah dibuat (jalankan server sekali dengan HARBUX_DSN agar
// tabelnya terbentuk). Tambahkan -reset untuk mengosongkan tabel tujuan lebih dulu.
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"

	_ "github.com/go-sql-driver/mysql"
	_ "modernc.org/sqlite"
)

func main() {
	src := flag.String("db", "data/cashflow.db", "file SQLite sumber")
	dsn := flag.String("dsn", "", "DSN MySQL tujuan (wajib)")
	reset := flag.Bool("reset", false, "kosongkan tabel tujuan sebelum impor")
	flag.Parse()
	if *dsn == "" {
		log.Fatal("-dsn wajib diisi")
	}

	lite, err := sql.Open("sqlite", *src)
	must(err)
	defer lite.Close()
	my, err := sql.Open("mysql", *dsn)
	must(err)
	defer my.Close()
	must(lite.Ping())
	must(my.Ping())

	tx, err := my.Begin()
	must(err)
	defer tx.Rollback()

	if _, err := tx.Exec("SET FOREIGN_KEY_CHECKS = 0"); err != nil {
		must(err)
	}
	if *reset {
		for _, t := range []string{"transactions", "accounts", "users"} {
			_, err := tx.Exec("DELETE FROM " + t)
			must(err)
		}
	}

	n := copyTable(tx, lite,
		"SELECT id, username, pass_hash, role FROM users",
		"INSERT INTO users (id, username, pass_hash, role) VALUES (?, ?, ?, ?)", 4)
	fmt.Printf("users        : %d baris\n", n)

	n = copyTable(tx, lite,
		"SELECT id, name, username_roblox, owner, initial_balance_robux, active, stock_status, user_id FROM accounts",
		"INSERT INTO accounts (id, name, username_roblox, owner, initial_balance_robux, active, stock_status, user_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?)", 8)
	fmt.Printf("accounts     : %d baris\n", n)

	n = copyTable(tx, lite,
		`SELECT id, type, account_id, from_account_id, to_account_id, counterpart,
			robux_amount, rate_idr, fee_idr, idr_total, status, note, created_at, created_by FROM transactions`,
		`INSERT INTO transactions (id, type, account_id, from_account_id, to_account_id, counterpart,
			robux_amount, rate_idr, fee_idr, idr_total, status, note, created_at, created_by)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, 14)
	fmt.Printf("transactions : %d baris\n", n)

	_, err = tx.Exec("SET FOREIGN_KEY_CHECKS = 1")
	must(err)
	must(tx.Commit())
	fmt.Println("selesai.")
}

// copyTable membaca semua baris dari SQLite lalu menulisnya ke MySQL apa adanya
// (id ikut disalin supaya relasi antar tabel tetap cocok).
func copyTable(tx *sql.Tx, lite *sql.DB, selectSQL, insertSQL string, cols int) int {
	rows, err := lite.Query(selectSQL)
	must(err)
	defer rows.Close()

	stmt, err := tx.Prepare(insertSQL)
	must(err)
	defer stmt.Close()

	vals := make([]any, cols)
	ptrs := make([]any, cols)
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	count := 0
	for rows.Next() {
		must(rows.Scan(ptrs...))
		_, err := stmt.Exec(vals...)
		must(err)
		count++
	}
	must(rows.Err())
	return count
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
