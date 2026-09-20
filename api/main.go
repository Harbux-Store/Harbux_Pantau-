package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

// schemaFor mengembalikan statement CREATE TABLE per dialek (dijalankan satu per satu,
// karena driver MySQL menolak beberapa statement dalam satu Exec).
func schemaFor(mysql bool) []string {
	if mysql {
		return []string{
			`CREATE TABLE IF NOT EXISTS users (
				id INT AUTO_INCREMENT PRIMARY KEY,
				username VARCHAR(64) NOT NULL UNIQUE,
				pass_hash VARCHAR(255) NOT NULL,
				role VARCHAR(16) NOT NULL DEFAULT 'staff'
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
			`CREATE TABLE IF NOT EXISTS accounts (
				id INT AUTO_INCREMENT PRIMARY KEY,
				name VARCHAR(128) NOT NULL,
				username_roblox VARCHAR(128) NOT NULL DEFAULT '',
				owner VARCHAR(128) NOT NULL DEFAULT '',
				initial_balance_robux BIGINT NOT NULL DEFAULT 0,
				active TINYINT NOT NULL DEFAULT 1,
				stock_status VARCHAR(16) NOT NULL DEFAULT 'ready',
				user_id INT NULL,
				INDEX idx_accounts_user (user_id)
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
			`CREATE TABLE IF NOT EXISTS transactions (
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
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		}
	}
	return []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			pass_hash TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'staff'
		)`,
		`CREATE TABLE IF NOT EXISTS accounts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			username_roblox TEXT NOT NULL DEFAULT '',
			owner TEXT NOT NULL DEFAULT '',
			initial_balance_robux INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS transactions (
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
		)`,
	}
}

// balanceExpr: perubahan saldo R$ sebuah transaksi t terhadap akun a.
// Hanya topup yang menambah; penjualan & transfer keluar mengurangi; fee/lain tidak menyentuh R$.
const balanceExpr = `CASE
	WHEN t.type = 'topup' AND t.account_id = a.id THEN t.robux_amount
	WHEN t.type = 'penjualan' AND t.account_id = a.id THEN -t.robux_amount
	WHEN t.type = 'transfer' AND t.to_account_id = a.id THEN t.robux_amount
	WHEN t.type = 'transfer' AND t.from_account_id = a.id THEN -t.robux_amount
	ELSE 0 END`

const accountSelect = `SELECT a.id, a.name, a.username_roblox, a.owner, a.initial_balance_robux, a.active, a.stock_status,
	a.initial_balance_robux + CAST(COALESCE((SELECT SUM(` + balanceExpr + `)
		FROM transactions t WHERE t.status = 'selesai'
		AND (t.account_id = a.id OR t.from_account_id = a.id OR t.to_account_id = a.id)), 0) AS SIGNED) AS bal
	FROM accounts a`

const timeLayout = "2006-01-02 15:04"

// hasColumn memeriksa keberadaan kolom (SQLite & MySQL punya cara berbeda).
func hasColumn(db *sql.DB, mysql bool, table, col string) bool {
	var n int
	if mysql {
		db.QueryRow(`SELECT COUNT(*) FROM information_schema.columns
			WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?`, table, col).Scan(&n)
	} else {
		db.QueryRow("SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?", table, col).Scan(&n)
	}
	return n > 0
}

func migrate(db *sql.DB, mysql bool) error {
	for _, stmt := range schemaFor(mysql) {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	// kolom tambahan untuk database lama (skema MySQL sudah menyertakannya sejak awal)
	for col, ddl := range map[string]string{
		"active":       "ALTER TABLE accounts ADD COLUMN active INTEGER NOT NULL DEFAULT 1",
		"user_id":      "ALTER TABLE accounts ADD COLUMN user_id INTEGER",
		"stock_status": "ALTER TABLE accounts ADD COLUMN stock_status TEXT NOT NULL DEFAULT 'ready'",
	} {
		if !hasColumn(db, mysql, "accounts", col) {
			if _, err := db.Exec(ddl); err != nil {
				return err
			}
		}
	}
	// akun lama tanpa pemilik -> milik admin pertama
	db.Exec(`UPDATE accounts SET user_id = (SELECT id FROM (SELECT MIN(id) id FROM users WHERE role = 'admin') x) WHERE user_id IS NULL`)
	// samakan format tanggal lama ("2026-09-19T10:00" / "2026-09-19") ke "2026-09-19 10:00"
	concat := "created_at || ' 00:00'"
	if mysql {
		concat = "CONCAT(created_at, ' 00:00')"
	}
	_, err := db.Exec(`UPDATE transactions SET created_at = CASE
		WHEN length(created_at) = 10 THEN ` + concat + `
		ELSE substr(replace(created_at, 'T', ' '), 1, 16) END
		WHERE created_at LIKE '%T%' OR length(created_at) != 16`)
	return err
}

// normalizeTime menerima "2006-01-02T15:04", "2006-01-02 15:04[:05]", atau "2006-01-02".
func normalizeTime(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Now().Format(timeLayout), true
	}
	for _, l := range []string{"2006-01-02T15:04", "2006-01-02T15:04:05", "2006-01-02 15:04", "2006-01-02 15:04:05", "2006-01-02"} {
		if t, err := time.ParseInLocation(l, s, time.Local); err == nil {
			return t.Format(timeLayout), true
		}
	}
	return "", false
}

// accScope membatasi akun ke milik user (admin melihat semua). prefix = alias tabel ("a." atau "").
// ID diambil dari token (int64), jadi aman disisipkan langsung ke SQL.
func accScope(u *user, prefix string) string {
	if u.Role == "admin" {
		return "1=1"
	}
	return fmt.Sprintf("%suser_id = %d", prefix, u.ID)
}

// txScope membatasi transaksi ke yang menyentuh akun milik user. alias = nama/alias tabel transaksi.
func txScope(u *user, alias string) string {
	if u.Role == "admin" {
		return "1=1"
	}
	mine := fmt.Sprintf("(SELECT id FROM accounts WHERE user_id = %d)", u.ID)
	return fmt.Sprintf("(%[1]s.account_id IN %[2]s OR %[1]s.from_account_id IN %[2]s OR %[1]s.to_account_id IN %[2]s)", alias, mine)
}

func validStatus(s string) bool {
	return s == "selesai" || s == "pending" || s == "dibatalkan"
}

func scanAccounts(rows *sql.Rows) []map[string]any {
	out := []map[string]any{}
	for rows.Next() {
		var ac account
		var active, bal int64
		var stock string
		if err := rows.Scan(&ac.ID, &ac.Name, &ac.UsernameRoblox, &ac.Owner, &ac.InitialBalanceRobux, &active, &stock, &bal); err != nil {
			continue
		}
		out = append(out, map[string]any{
			"id": ac.ID, "name": ac.Name, "username_roblox": ac.UsernameRoblox,
			"owner": ac.Owner, "initial_balance_robux": ac.InitialBalanceRobux,
			"active": active == 1, "stock_status": stock, "current_robux": bal,
		})
	}
	return out
}

type txRow struct {
	ID            int64  `json:"id"`
	Type          string `json:"type"`
	AccountID     *int64 `json:"account_id"`
	FromAccountID *int64 `json:"from_account_id"`
	ToAccountID   *int64 `json:"to_account_id"`
	Counterpart   string `json:"counterpart"`
	RobuxAmount   int64  `json:"robux_amount"`
	RateIDR       int64  `json:"rate_idr"`
	FeeIDR        int64  `json:"fee_idr"`
	IDRTotal      int64  `json:"idr_total"`
	Status        string `json:"status"`
	Note          string `json:"note"`
	CreatedAt     string `json:"created_at"`
	CreatedBy     int64  `json:"created_by"`
	AccountName   string `json:"account_name"`
	FromName      string `json:"from_name"`
	ToName        string `json:"to_name"`
}

type user struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

type account struct {
	ID                  int64  `json:"id"`
	Name                string `json:"name"`
	UsernameRoblox      string `json:"username_roblox"`
	Owner               string `json:"owner"`
	InitialBalanceRobux int64  `json:"initial_balance_robux"`
}

// loginGuard membatasi percobaan login yang gagal (anti tebak-tebakan password).
type loginGuard struct {
	mu   sync.Mutex
	fail map[string][]time.Time
}

const (
	maxLoginFail  = 5
	loginFailSpan = 15 * time.Minute
)

// allow melaporkan apakah key (IP+username) masih boleh mencoba login.
func (g *loginGuard) allow(key string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := time.Now()
	keep := g.fail[key][:0]
	for _, t := range g.fail[key] {
		if now.Sub(t) < loginFailSpan {
			keep = append(keep, t)
		}
	}
	g.fail[key] = keep
	return len(keep) < maxLoginFail
}

func (g *loginGuard) fail_(key string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.fail[key] = append(g.fail[key], time.Now())
}

func (g *loginGuard) reset(key string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.fail, key)
}

type app struct {
	db     *sql.DB
	mysql  bool
	secret []byte
	// writeMu menyerialkan cek-saldo + insert agar dua request bersamaan tidak membuat saldo minus.
	writeMu      sync.Mutex
	secureCookie bool
	guard        loginGuard
}

func main() {
	// HARBUX_DSN diisi -> MySQL/MariaDB, mis.
	//   harbux:sandi@tcp(127.0.0.1:3306)/harbux?parseTime=true&charset=utf8mb4&loc=Asia%2FJakarta
	// Kosong -> SQLite di HARBUX_DB (praktis untuk pengembangan di laptop).
	dsn := os.Getenv("HARBUX_DSN")
	useMySQL := dsn != ""
	driver, target := "sqlite", getenv("HARBUX_DB", "data/cashflow.db")
	if useMySQL {
		driver, target = "mysql", dsn
	} else if dir := filepath.Dir(target); dir != "" {
		os.MkdirAll(dir, 0o755)
	}
	db, err := sql.Open(driver, target)
	if err != nil {
		log.Fatal(err)
	}
	if useMySQL {
		db.SetMaxOpenConns(20)
		db.SetMaxIdleConns(5)
		db.SetConnMaxLifetime(3 * time.Minute)
	}
	if err := db.Ping(); err != nil {
		log.Fatal("gagal konek database: ", err)
	}
	if err := migrate(db, useMySQL); err != nil {
		log.Fatal(err)
	}
	log.Println("Database:", driver)
	secret, err := loadSecret()
	if err != nil {
		log.Fatal(err)
	}
	a := newApp(db, useMySQL, secret, os.Getenv("HARBUX_SECURE_COOKIE") == "1")
	port := getenv("PORT", "8080")
	log.Println("API di http://localhost:" + port)
	log.Fatal(http.ListenAndServe(":"+port, a.routes()))
}

// loadSecret mengambil kunci penanda-tangan login dari env HARBUX_SECRET.
// Bila kosong, kunci acak dibuat sekali lalu disimpan ke file (HARBUX_SECRET_FILE)
// supaya pengguna tidak ter-logout setiap kali server di-restart.
func loadSecret() ([]byte, error) {
	if v := os.Getenv("HARBUX_SECRET"); v != "" {
		if len(v) < 32 {
			return nil, fmt.Errorf("HARBUX_SECRET terlalu pendek: minimal 32 karakter")
		}
		return []byte(v), nil
	}
	path := getenv("HARBUX_SECRET_FILE", "data/secret.key")
	if b, err := os.ReadFile(path); err == nil && len(b) >= 32 {
		return b, nil
	}
	b := make([]byte, 48)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	key := []byte(base64.RawURLEncoding.EncodeToString(b))
	if dir := filepath.Dir(path); dir != "" {
		os.MkdirAll(dir, 0o755)
	}
	if err := os.WriteFile(path, key, 0o600); err != nil {
		return nil, fmt.Errorf("gagal menyimpan kunci login ke %s: %w", path, err)
	}
	log.Println("HARBUX_SECRET tidak diset; kunci acak dibuat & disimpan di", path)
	return key, nil
}

func getenv(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func newApp(db *sql.DB, mysql bool, secret []byte, secureCookie bool) *app {
	return &app{db: db, mysql: mysql, secret: secret, secureCookie: secureCookie,
		guard: loginGuard{fail: map[string][]time.Time{}}}
}

func (a *app) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /auth/setup", a.setupNeeded)
	mux.HandleFunc("POST /auth/register", a.register)
	mux.HandleFunc("POST /auth/login", a.login)
	mux.HandleFunc("POST /auth/logout", a.logout)
	mux.HandleFunc("GET /auth/me", a.requireAuth(a.me))
	mux.HandleFunc("GET /api/users", a.requireAdmin(a.listUsers))
	mux.HandleFunc("POST /api/users", a.requireAdmin(a.createUser))
	mux.HandleFunc("DELETE /api/users/{id}", a.requireAdmin(a.deleteUser))

	mux.HandleFunc("GET /api/accounts", a.requireAuth(a.listAccounts))
	mux.HandleFunc("POST /api/accounts", a.requireAuth(a.createAccount))
	mux.HandleFunc("PUT /api/accounts/{id}", a.requireAuth(a.updateAccount))
	mux.HandleFunc("DELETE /api/accounts/{id}", a.requireAuth(a.deleteAccount))

	mux.HandleFunc("GET /api/transactions", a.requireAuth(a.listTransactions))
	mux.HandleFunc("POST /api/transactions", a.requireAuth(a.createTransaction))
	mux.HandleFunc("POST /api/transactions/transfer", a.requireAuth(a.createTransfer))
	mux.HandleFunc("PUT /api/transactions/{id}", a.requireAuth(a.updateTransaction))
	mux.HandleFunc("DELETE /api/transactions/{id}", a.requireAuth(a.deleteTransaction))

	mux.HandleFunc("GET /api/summary", a.requireAuth(a.summary))
	mux.HandleFunc("GET /api/report/export", a.requireAuth(a.exportCSV))
	return mux
}

type ctxKey string

func (a *app) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, err := a.userFromReq(r)
		if err != nil {
			writeErr(w, http.StatusUnauthorized, "login dulu")
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), ctxKey("user"), u)))
	}
}

func (a *app) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, err := a.userFromReq(r)
		if err != nil {
			writeErr(w, http.StatusUnauthorized, "login dulu")
			return
		}
		if u.Role != "admin" {
			writeErr(w, http.StatusForbidden, "butuh role admin")
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), ctxKey("user"), u)))
	}
}

func (a *app) userFromReq(r *http.Request) (*user, error) {
	c, err := r.Cookie("token")
	if err != nil {
		return nil, err
	}
	claims := jwt.MapClaims{}
	_, err = jwt.ParseWithClaims(c.Value, claims, func(t *jwt.Token) (any, error) {
		return a.secret, nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		return nil, err
	}
	uid, ok1 := claims["uid"].(float64)
	name, ok2 := claims["name"].(string)
	role, ok3 := claims["role"].(string)
	if !ok1 || !ok2 || !ok3 {
		return nil, fmt.Errorf("token tidak valid")
	}
	// pastikan user masih ada (bisa sudah dihapus admin) & pakai role terbaru dari database
	u := &user{ID: int64(uid), Username: name, Role: role}
	if err := a.db.QueryRow("SELECT username, role FROM users WHERE id = ?", u.ID).Scan(&u.Username, &u.Role); err != nil {
		return nil, fmt.Errorf("user tidak ada")
	}
	return u, nil
}

func (a *app) setToken(w http.ResponseWriter, u user) {
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"uid":  u.ID,
		"name": u.Username,
		"role": u.Role,
		"exp":  time.Now().Add(7 * 24 * time.Hour).Unix(),
	})
	s, err := tok.SignedString(a.secret)
	if err != nil {
		writeErr(w, 500, "gagal buat token")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: "token", Value: s, Path: "/", HttpOnly: true,
		SameSite: http.SameSiteLaxMode, MaxAge: 7 * 24 * 3600, Secure: a.secureCookie,
	})
}

type authReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

// setupNeeded dipakai halaman login untuk tahu apakah admin pertama belum dibuat.
func (a *app) setupNeeded(w http.ResponseWriter, r *http.Request) {
	var n int
	if err := a.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&n); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"setup_needed": n == 0})
}

func (a *app) register(w http.ResponseWriter, r *http.Request) {
	var n int
	a.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&n)
	if n > 0 {
		writeErr(w, http.StatusForbidden, "registrasi hanya untuk admin pertama")
		return
	}
	var req authReq
	if !decode(w, r, &req) {
		return
	}
	if len(req.Username) < 3 || len(req.Password) < 8 {
		writeErr(w, http.StatusBadRequest, "username min 3 karakter, password min 8")
		return
	}
	if err := a.insertUser(req.Username, req.Password, "admin"); err != nil {
		writeErr(w, http.StatusConflict, "username sudah dipakai")
		return
	}
	var u user
	a.db.QueryRow("SELECT id, username, role FROM users WHERE username = ?", req.Username).Scan(&u.ID, &u.Username, &u.Role)
	a.setToken(w, u)
	writeJSON(w, http.StatusCreated, u)
}

func (a *app) login(w http.ResponseWriter, r *http.Request) {
	var req authReq
	if !decode(w, r, &req) {
		return
	}
	key := clientIP(r) + "|" + strings.ToLower(req.Username)
	if !a.guard.allow(key) {
		writeErr(w, http.StatusTooManyRequests, "terlalu banyak percobaan login; coba lagi dalam 15 menit")
		return
	}
	var u user
	var hash string
	err := a.db.QueryRow("SELECT id, username, pass_hash, role FROM users WHERE username = ?", req.Username).
		Scan(&u.ID, &u.Username, &hash, &u.Role)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)) != nil {
		a.guard.fail_(key)
		writeErr(w, http.StatusUnauthorized, "username/password salah")
		return
	}
	a.guard.reset(key)
	a.setToken(w, u)
	writeJSON(w, http.StatusOK, u)
}

// clientIP mengambil IP pengguna, termasuk bila di belakang reverse proxy (Nginx).
func clientIP(r *http.Request) string {
	if v := r.Header.Get("X-Forwarded-For"); v != "" {
		if i := strings.IndexByte(v, ','); i > 0 {
			return strings.TrimSpace(v[:i])
		}
		return strings.TrimSpace(v)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (a *app) logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: "token", Value: "", Path: "/", HttpOnly: true, MaxAge: -1})
	writeJSON(w, http.StatusOK, map[string]string{"ok": "logout"})
}

func (a *app) me(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, currentUser(r))
}

func (a *app) insertUser(username, password, role string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = a.db.Exec("INSERT INTO users (username, pass_hash, role) VALUES (?, ?, ?)", username, string(hash), role)
	return err
}

func (a *app) listUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query("SELECT id, username, role FROM users ORDER BY id")
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	defer rows.Close()
	var out []user
	for rows.Next() {
		var u user
		rows.Scan(&u.ID, &u.Username, &u.Role)
		out = append(out, u)
	}
	if out == nil {
		out = []user{}
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *app) createUser(w http.ResponseWriter, r *http.Request) {
	var req authReq
	if !decode(w, r, &req) {
		return
	}
	if req.Role != "admin" && req.Role != "staff" {
		req.Role = "staff"
	}
	if len(req.Username) < 3 || len(req.Password) < 8 {
		writeErr(w, http.StatusBadRequest, "username min 3 karakter, password min 8")
		return
	}
	if err := a.insertUser(req.Username, req.Password, req.Role); err != nil {
		writeErr(w, http.StatusConflict, "username sudah dipakai")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"ok": "user dibuat"})
}

// deleteUser menghapus pengguna beserta semua akun Roblox & transaksinya.
func (a *app) deleteUser(w http.ResponseWriter, r *http.Request) {
	id := idParam(r)
	if id == currentUser(r).ID {
		writeErr(w, http.StatusBadRequest, "tidak bisa menghapus akun sendiri")
		return
	}
	var role string
	if err := a.db.QueryRow("SELECT role FROM users WHERE id = ?", id).Scan(&role); err != nil {
		writeErr(w, 404, "pengguna tidak ada")
		return
	}
	if role == "admin" {
		var admins int
		a.db.QueryRow("SELECT COUNT(*) FROM users WHERE role = 'admin'").Scan(&admins)
		if admins <= 1 {
			writeErr(w, http.StatusBadRequest, "tidak bisa menghapus admin terakhir")
			return
		}
	}
	a.writeMu.Lock()
	defer a.writeMu.Unlock()
	tx, err := a.db.Begin()
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	defer tx.Rollback()
	mine := "(SELECT id FROM accounts WHERE user_id = ?)"
	res, err := tx.Exec("DELETE FROM transactions WHERE account_id IN "+mine+" OR from_account_id IN "+mine+" OR to_account_id IN "+mine, id, id, id)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	nTx, _ := res.RowsAffected()
	res, err = tx.Exec("DELETE FROM accounts WHERE user_id = ?", id)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	nAcc, _ := res.RowsAffected()
	if _, err := tx.Exec("DELETE FROM users WHERE id = ?", id); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if err := tx.Commit(); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": "pengguna dihapus", "accounts_deleted": nAcc, "transactions_deleted": nTx})
}

func (a *app) listAccounts(w http.ResponseWriter, r *http.Request) {
	where := []string{accScope(currentUser(r), "a.")}
	if r.URL.Query().Get("all") != "1" {
		where = append(where, "a.active = 1")
	}
	rows, err := a.db.Query(accountSelect + " WHERE " + strings.Join(where, " AND ") + " ORDER BY a.active DESC, a.name")
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	defer rows.Close()
	writeJSON(w, http.StatusOK, scanAccounts(rows))
}

type accountReq struct {
	Name                string `json:"name"`
	UsernameRoblox      string `json:"username_roblox"`
	Owner               string `json:"owner"`
	InitialBalanceRobux int64  `json:"initial_balance_robux"`
	Active              *bool  `json:"active"`
	StockStatus         string `json:"stock_status"`
}

func validStockStatus(s string) bool {
	return s == "ready" || s == "pending" || s == "borrow"
}

// normalizeAccount: nama akun opsional — kalau kosong pakai username Roblox.
func (req *accountReq) normalize() string {
	req.Name = strings.TrimSpace(req.Name)
	req.UsernameRoblox = strings.TrimSpace(req.UsernameRoblox)
	if req.Name == "" {
		req.Name = req.UsernameRoblox
	}
	if req.Name == "" {
		return "username Roblox wajib"
	}
	if req.InitialBalanceRobux < 0 {
		return "jumlah robux tidak boleh negatif"
	}
	if req.StockStatus == "" {
		req.StockStatus = "ready"
	}
	if !validStockStatus(req.StockStatus) {
		return "status tidak valid (borrow/pending/ready)"
	}
	return ""
}

func (a *app) createAccount(w http.ResponseWriter, r *http.Request) {
	var req accountReq
	if !decode(w, r, &req) {
		return
	}
	if msg := req.normalize(); msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	res, err := a.db.Exec("INSERT INTO accounts (name, username_roblox, owner, initial_balance_robux, stock_status, user_id) VALUES (?, ?, ?, ?, ?, ?)",
		req.Name, req.UsernameRoblox, req.Owner, req.InitialBalanceRobux, req.StockStatus, currentUser(r).ID)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	id, _ := res.LastInsertId()
	writeJSON(w, http.StatusCreated, map[string]int64{"id": id})
}

func (a *app) updateAccount(w http.ResponseWriter, r *http.Request) {
	id := idParam(r)
	var req accountReq
	if !decode(w, r, &req) {
		return
	}
	if msg := req.normalize(); msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	var active any // nil = status aktif tidak diubah
	if req.Active != nil {
		active = 0
		if *req.Active {
			active = 1
		}
	}
	res, err := a.db.Exec("UPDATE accounts SET name=?, username_roblox=?, owner=?, initial_balance_robux=?, stock_status=?, active=COALESCE(?, active) WHERE id=? AND "+accScope(currentUser(r), ""),
		req.Name, req.UsernameRoblox, req.Owner, req.InitialBalanceRobux, req.StockStatus, active, id)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeErr(w, 404, "akun tidak ada")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"ok": "diupdate"})
}

// deleteAccount menonaktifkan akun (soft delete) supaya riwayat transaksi & laporan tetap utuh.
func (a *app) deleteAccount(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("permanent") == "1" {
		a.purgeAccount(w, r)
		return
	}
	res, err := a.db.Exec("UPDATE accounts SET active = 0 WHERE id = ? AND "+accScope(currentUser(r), ""), idParam(r))
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeErr(w, 404, "akun tidak ada")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"ok": "akun dinonaktifkan"})
}

// purgeAccount menghapus akun beserta transaksinya secara permanen.
// Ditolak bila akun punya transfer dengan akun lain, karena menghapusnya akan mengubah saldo akun lain itu.
func (a *app) purgeAccount(w http.ResponseWriter, r *http.Request) {
	id := idParam(r)
	a.writeMu.Lock()
	defer a.writeMu.Unlock()
	var exists int
	a.db.QueryRow("SELECT COUNT(*) FROM accounts WHERE id = ? AND "+accScope(currentUser(r), ""), id).Scan(&exists)
	if exists == 0 {
		writeErr(w, 404, "akun tidak ada")
		return
	}
	var transfers int
	a.db.QueryRow("SELECT COUNT(*) FROM transactions WHERE type = 'transfer' AND (from_account_id = ? OR to_account_id = ?)", id, id).Scan(&transfers)
	if transfers > 0 {
		writeErr(w, http.StatusBadRequest, fmt.Sprintf("akun punya %d transfer dengan akun lain; hapus transfernya dulu atau nonaktifkan saja", transfers))
		return
	}
	tx, err := a.db.Begin()
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	defer tx.Rollback()
	res, err := tx.Exec("DELETE FROM transactions WHERE account_id = ?", id)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	n, _ := res.RowsAffected()
	if _, err := tx.Exec("DELETE FROM accounts WHERE id = ?", id); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if err := tx.Commit(); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": "akun dihapus", "transactions_deleted": n})
}

func (a *app) listTransactions(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	args := []any{}
	clauses := []string{txScope(currentUser(r), "t")}
	if v := q.Get("account_id"); v != "" {
		clauses = append(clauses, "(t.account_id = ? OR t.from_account_id = ? OR t.to_account_id = ?)")
		args = append(args, v, v, v)
	}
	if v := q.Get("type"); v != "" {
		clauses = append(clauses, "t.type = ?")
		args = append(args, v)
	}
	if v := q.Get("status"); v != "" {
		clauses = append(clauses, "t.status = ?")
		args = append(args, v)
	}
	if v := q.Get("from"); v != "" {
		clauses = append(clauses, "date(t.created_at) >= ?")
		args = append(args, v)
	}
	if v := q.Get("to"); v != "" {
		clauses = append(clauses, "date(t.created_at) <= ?")
		args = append(args, v)
	}
	limit := ""
	if n, err := strconv.Atoi(q.Get("limit")); err == nil && n > 0 {
		limit = fmt.Sprintf(" LIMIT %d", n)
	}
	query := fmt.Sprintf(`SELECT t.id, t.type, t.account_id, t.from_account_id, t.to_account_id,
		t.counterpart, t.robux_amount, t.rate_idr, t.fee_idr, t.idr_total,
		t.status, t.note, t.created_at, t.created_by,
		COALESCE((SELECT name FROM accounts WHERE id = t.account_id), ''),
		COALESCE((SELECT name FROM accounts WHERE id = t.from_account_id), ''),
		COALESCE((SELECT name FROM accounts WHERE id = t.to_account_id), '')
	FROM transactions t WHERE %s ORDER BY t.created_at DESC, t.id DESC%s`, strings.Join(clauses, " AND "), limit)
	rows, err := a.db.Query(query, args...)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	defer rows.Close()
	out := []txRow{}
	for rows.Next() {
		var t txRow
		if err := rows.Scan(&t.ID, &t.Type, &t.AccountID, &t.FromAccountID, &t.ToAccountID,
			&t.Counterpart, &t.RobuxAmount, &t.RateIDR, &t.FeeIDR, &t.IDRTotal,
			&t.Status, &t.Note, &t.CreatedAt, &t.CreatedBy, &t.AccountName, &t.FromName, &t.ToName); err != nil {
			continue
		}
		if t.Type == "transfer" {
			t.AccountName = t.FromName + " → " + t.ToName
		}
		out = append(out, t)
	}
	writeJSON(w, http.StatusOK, out)
}

type txReq struct {
	Type          string `json:"type"`
	AccountID     *int64 `json:"account_id"`
	FromAccountID *int64 `json:"from_account_id"`
	ToAccountID   *int64 `json:"to_account_id"`
	Counterpart   string `json:"counterpart"`
	RobuxAmount   int64  `json:"robux_amount"`
	RateIDR       int64  `json:"rate_idr"`
	FeeIDR        int64  `json:"fee_idr"`
	Status        string `json:"status"`
	Note          string `json:"note"`
	CreatedAt     string `json:"created_at"`
}

// validateTx memeriksa & menormalkan request transaksi non-transfer. excludeID dipakai saat edit
// supaya transaksi lama tidak ikut dihitung saat cek saldo penjualan.
func (a *app) validateTx(u *user, req *txReq, excludeID int64) string {
	if !validType(req.Type) {
		return "tipe tidak valid (topup/penjualan/fee/lain)"
	}
	if req.AccountID == nil {
		return "account_id wajib"
	}
	if req.RobuxAmount < 0 || req.RateIDR < 0 || req.FeeIDR < 0 {
		return "nominal tidak boleh negatif"
	}
	if (req.Type == "topup" || req.Type == "penjualan") && req.RobuxAmount == 0 {
		return "jumlah robux wajib diisi untuk topup/penjualan"
	}
	if req.Status == "" {
		req.Status = "selesai"
	}
	if !validStatus(req.Status) {
		return "status tidak valid (selesai/pending/dibatalkan)"
	}
	ts, ok := normalizeTime(req.CreatedAt)
	if !ok {
		return "format tanggal tidak valid"
	}
	req.CreatedAt = ts
	var active int
	if err := a.db.QueryRow("SELECT active FROM accounts WHERE id = ? AND "+accScope(u, ""), *req.AccountID).Scan(&active); err != nil {
		return "akun tidak ditemukan"
	}
	if active == 0 && excludeID == 0 {
		return "akun sudah nonaktif"
	}
	if req.Type == "penjualan" && req.Status == "selesai" {
		bal, err := a.currentBalance(*req.AccountID, excludeID)
		if err != nil {
			return err.Error()
		}
		if bal < req.RobuxAmount {
			return fmt.Sprintf("saldo akun tidak cukup (tersedia %d R)", bal)
		}
	}
	return ""
}

func (a *app) createTransaction(w http.ResponseWriter, r *http.Request) {
	var req txReq
	if !decode(w, r, &req) {
		return
	}
	a.writeMu.Lock()
	defer a.writeMu.Unlock()
	if msg := a.validateTx(currentUser(r), &req, 0); msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	idrTotal := req.RobuxAmount*req.RateIDR + req.FeeIDR
	res, err := a.db.Exec(`INSERT INTO transactions
		(type, account_id, from_account_id, to_account_id, counterpart, robux_amount, rate_idr, fee_idr, idr_total, status, note, created_at, created_by)
		VALUES (?, ?, NULL, NULL, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		req.Type, *req.AccountID, req.Counterpart, req.RobuxAmount, req.RateIDR, req.FeeIDR, idrTotal,
		req.Status, req.Note, req.CreatedAt, currentUser(r).ID)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	id, _ := res.LastInsertId()
	writeJSON(w, http.StatusCreated, map[string]int64{"id": id, "idr_total": idrTotal})
}

// currentBalance menghitung saldo R$ akun; transaksi dengan id excludeID diabaikan.
func (a *app) currentBalance(accountID, excludeID int64) (int64, error) {
	var bal int64
	err := a.db.QueryRow(accountSelect+" WHERE a.id = ?", accountID).Scan(new(int64), new(string), new(string), new(string), new(int64), new(int64), new(string), &bal)
	if err != nil || excludeID == 0 {
		return bal, err
	}
	var delta int64
	err = a.db.QueryRow(`SELECT CAST(COALESCE(SUM(`+balanceExpr+`), 0) AS SIGNED) FROM transactions t, accounts a
		WHERE a.id = ? AND t.id = ? AND t.status = 'selesai'`, accountID, excludeID).Scan(&delta)
	return bal - delta, err
}

func (a *app) createTransfer(w http.ResponseWriter, r *http.Request) {
	var req txReq
	if !decode(w, r, &req) {
		return
	}
	if req.FromAccountID == nil || req.ToAccountID == nil {
		writeErr(w, http.StatusBadRequest, "from_account_id & to_account_id wajib")
		return
	}
	if *req.FromAccountID == *req.ToAccountID {
		writeErr(w, http.StatusBadRequest, "akun asal dan tujuan tidak boleh sama")
		return
	}
	if req.RobuxAmount <= 0 {
		writeErr(w, http.StatusBadRequest, "jumlah transfer harus > 0")
		return
	}
	if req.Status == "" {
		req.Status = "selesai"
	}
	if !validStatus(req.Status) {
		writeErr(w, http.StatusBadRequest, "status tidak valid")
		return
	}
	ts, ok := normalizeTime(req.CreatedAt)
	if !ok {
		writeErr(w, http.StatusBadRequest, "format tanggal tidak valid")
		return
	}
	req.CreatedAt = ts
	var nActive int
	a.db.QueryRow("SELECT COUNT(*) FROM accounts WHERE active = 1 AND id IN (?, ?) AND "+accScope(currentUser(r), ""), *req.FromAccountID, *req.ToAccountID).Scan(&nActive)
	if nActive != 2 {
		writeErr(w, http.StatusBadRequest, "akun asal/tujuan tidak ditemukan atau nonaktif")
		return
	}
	a.writeMu.Lock()
	defer a.writeMu.Unlock()
	bal, err := a.currentBalance(*req.FromAccountID, 0)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if req.Status == "selesai" && bal < req.RobuxAmount {
		writeErr(w, http.StatusBadRequest, fmt.Sprintf("saldo akun asal tidak cukup (tersedia %d R)", bal))
		return
	}
	_, err = a.db.Exec(`INSERT INTO transactions
		(type, account_id, from_account_id, to_account_id, counterpart, robux_amount, rate_idr, fee_idr, idr_total, status, note, created_at, created_by)
		VALUES ('transfer', NULL, ?, ?, ?, ?, 0, 0, 0, ?, ?, ?, ?)`,
		*req.FromAccountID, *req.ToAccountID, req.Counterpart, req.RobuxAmount,
		req.Status, req.Note, req.CreatedAt, currentUser(r).ID)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"ok": "transfer dicatat"})
}

func (a *app) updateTransaction(w http.ResponseWriter, r *http.Request) {
	id := idParam(r)
	var typ, oldCreated string
	if err := a.db.QueryRow("SELECT type, created_at FROM transactions t WHERE id = ? AND "+txScope(currentUser(r), "t"), id).Scan(&typ, &oldCreated); err != nil {
		writeErr(w, 404, "transaksi tidak ada")
		return
	}
	if typ == "transfer" {
		writeErr(w, http.StatusBadRequest, "transaksi transfer tidak bisa diedit; hapus lalu buat ulang")
		return
	}
	var req txReq
	if !decode(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.CreatedAt) == "" {
		req.CreatedAt = oldCreated
	}
	a.writeMu.Lock()
	defer a.writeMu.Unlock()
	if msg := a.validateTx(currentUser(r), &req, id); msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	idrTotal := req.RobuxAmount*req.RateIDR + req.FeeIDR
	if _, err := a.db.Exec(`UPDATE transactions SET type=?, account_id=?, counterpart=?, robux_amount=?, rate_idr=?, fee_idr=?, idr_total=?, status=?, note=?, created_at=? WHERE id=?`,
		req.Type, *req.AccountID, req.Counterpart, req.RobuxAmount, req.RateIDR, req.FeeIDR, idrTotal,
		req.Status, req.Note, req.CreatedAt, id); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"ok": "diupdate"})
}

func (a *app) deleteTransaction(w http.ResponseWriter, r *http.Request) {
	res, err := a.db.Exec("DELETE FROM transactions WHERE id = ? AND "+txScope(currentUser(r), "transactions"), idParam(r))
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeErr(w, 404, "transaksi tidak ada")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"ok": "dihapus"})
}

// summary: profit = pendapatan - HPP (R$ terjual × harga beli rata-rata) - biaya (fee/lain).
// Harga beli rata-rata dihitung dari seluruh topup selesai (tidak tergantung periode) supaya
// membeli stok di satu hari tidak terbaca sebagai rugi.
func (a *app) summary(w http.ResponseWriter, r *http.Request) {
	from, to := rangeFrom(r)
	u := currentUser(r)
	scope := " AND " + txScope(u, "transactions")
	var topupIDR, topupRobux int64
	a.db.QueryRow(`SELECT CAST(COALESCE(SUM(idr_total),0) AS SIGNED), CAST(COALESCE(SUM(robux_amount),0) AS SIGNED)
		FROM transactions WHERE status = 'selesai' AND type = 'topup'`+scope).Scan(&topupIDR, &topupRobux)
	avgBuy := 0.0
	if topupRobux > 0 {
		avgBuy = float64(topupIDR) / float64(topupRobux)
	}

	rows, err := a.db.Query(`
		SELECT type, CAST(COALESCE(SUM(idr_total),0) AS SIGNED), CAST(COALESCE(SUM(robux_amount),0) AS SIGNED), COUNT(*)
		FROM transactions
		WHERE status = 'selesai' AND type != 'transfer'
		  AND date(created_at) BETWEEN COALESCE(?, date(created_at)) AND COALESCE(?, date(created_at))`+scope+`
		GROUP BY type`, from, to)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	var income, topupSpent, expenses, robuxSold, robuxBought, salesCount int64
	for rows.Next() {
		var typ string
		var idr, rb, cnt int64
		rows.Scan(&typ, &idr, &rb, &cnt)
		switch typ {
		case "penjualan":
			income += idr
			robuxSold += rb
			salesCount += cnt
		case "topup":
			topupSpent += idr
			robuxBought += rb
		case "fee", "lain":
			expenses += idr
		}
	}
	rows.Close()
	cogs := int64(float64(robuxSold)*avgBuy + 0.5)
	avgSell := 0.0
	if robuxSold > 0 {
		avgSell = float64(income) / float64(robuxSold)
	}

	var pendingCount int64
	a.db.QueryRow(`SELECT COUNT(*) FROM transactions WHERE status = 'pending'` + scope).Scan(&pendingCount)

	last30, todayExpr := "date('now','localtime','-29 days')", "date('now','localtime')"
	if a.mysql {
		last30, todayExpr = "CURDATE() - INTERVAL 29 DAY", "CURDATE()"
	}
	daily := []map[string]any{}
	dRows, err := a.db.Query(`SELECT date(created_at) d,
		CAST(COALESCE(SUM(CASE WHEN type='penjualan' THEN idr_total END),0) AS SIGNED),
		CAST(COALESCE(SUM(CASE WHEN type='penjualan' THEN robux_amount END),0) AS SIGNED),
		CAST(COALESCE(SUM(CASE WHEN type IN ('fee','lain') THEN idr_total END),0) AS SIGNED)
		FROM transactions
		WHERE status = 'selesai' AND type != 'transfer'
		  AND date(created_at) BETWEEN COALESCE(?, `+last30+`) AND COALESCE(?, `+todayExpr+`)`+scope+`
		GROUP BY d ORDER BY d`, from, to)
	if err == nil {
		for dRows.Next() {
			var d string
			var inc, rb, exp int64
			dRows.Scan(&d, &inc, &rb, &exp)
			daily = append(daily, map[string]any{
				"date": d, "income": inc, "robux_sold": rb,
				"profit": inc - int64(float64(rb)*avgBuy+0.5) - exp,
			})
		}
		dRows.Close()
	}

	perAccount := []map[string]any{}
	pRows, err := a.db.Query(`SELECT a.name, COUNT(t.id), CAST(COALESCE(SUM(t.robux_amount),0) AS SIGNED), CAST(COALESCE(SUM(t.idr_total),0) AS SIGNED)
		FROM transactions t JOIN accounts a ON a.id = t.account_id
		WHERE t.type = 'penjualan' AND t.status = 'selesai'
		  AND date(t.created_at) BETWEEN COALESCE(?, date(t.created_at)) AND COALESCE(?, date(t.created_at))
		  AND `+accScope(u, "a.")+`
		GROUP BY a.id ORDER BY 4 DESC`, from, to)
	if err == nil {
		for pRows.Next() {
			var name string
			var cnt, rb, inc int64
			pRows.Scan(&name, &cnt, &rb, &inc)
			perAccount = append(perAccount, map[string]any{"name": name, "count": cnt, "robux_sold": rb, "income": inc})
		}
		pRows.Close()
	}

	topBuyers := []map[string]any{}
	bRows, err := a.db.Query(`SELECT counterpart, COUNT(*), CAST(COALESCE(SUM(robux_amount),0) AS SIGNED), CAST(COALESCE(SUM(idr_total),0) AS SIGNED)
		FROM transactions
		WHERE type = 'penjualan' AND status = 'selesai' AND counterpart != ''
		  AND date(created_at) BETWEEN COALESCE(?, date(created_at)) AND COALESCE(?, date(created_at))`+scope+`
		GROUP BY lower(counterpart) ORDER BY 4 DESC LIMIT 10`, from, to)
	if err == nil {
		for bRows.Next() {
			var name string
			var cnt, rb, inc int64
			bRows.Scan(&name, &cnt, &rb, &inc)
			topBuyers = append(topBuyers, map[string]any{"name": name, "count": cnt, "robux": rb, "income": inc})
		}
		bRows.Close()
	}

	accRows, err := a.db.Query(accountSelect + " WHERE a.active = 1 AND " + accScope(u, "a.") + " ORDER BY a.name")
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	accounts := scanAccounts(accRows)
	accRows.Close()

	writeJSON(w, http.StatusOK, map[string]any{
		"income": income, "cogs": cogs, "expenses": expenses,
		"cost": cogs + expenses, "profit": income - cogs - expenses,
		"topup_spent": topupSpent,
		"robux_sold":  robuxSold, "robux_bought": robuxBought, "sales_count": salesCount,
		"avg_buy_rate": avgBuy, "avg_sell_rate": avgSell, "pending_count": pendingCount,
		"daily": daily, "per_account": perAccount, "top_buyers": topBuyers,
		"from": from, "to": to, "accounts": accounts,
	})
}

func (a *app) exportCSV(w http.ResponseWriter, r *http.Request) {
	from, to := rangeFrom(r)
	rows, err := a.db.Query(`SELECT t.id, t.created_at, t.type,
		COALESCE((SELECT name FROM accounts WHERE id = COALESCE(t.account_id, t.from_account_id, t.to_account_id)), ''),
		t.counterpart, t.robux_amount, t.rate_idr, t.fee_idr, t.idr_total, t.status, t.note,
		COALESCE((SELECT username FROM users WHERE id = t.created_by), '')
		FROM transactions t WHERE `+txScope(currentUser(r), "t")+` AND date(created_at) BETWEEN COALESCE(?, date(created_at)) AND COALESCE(?, date(created_at))
		ORDER BY t.created_at, t.id`, from, to)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	defer rows.Close()
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="laporan-`+time.Now().Format("2006-01-02")+`.csv"`)
	wr := csv.NewWriter(w)
	wr.Write([]string{"ID", "Tanggal", "Tipe", "Akun", "Counterpart", "Robux", "Rate", "Fee", "IDR Total", "Status", "Catatan", "Input oleh"})
	for rows.Next() {
		var id, rb, rate, fee, total int64
		var t, name, cpart, status, note, by, ctype string
		rows.Scan(&id, &t, &ctype, &name, &cpart, &rb, &rate, &fee, &total, &status, &note, &by)
		wr.Write([]string{strconv.FormatInt(id, 10), t, ctype, name, cpart,
			strconv.FormatInt(rb, 10), strconv.FormatInt(rate, 10), strconv.FormatInt(fee, 10),
			strconv.FormatInt(total, 10), status, note, by})
	}
	wr.Flush()
}

func validType(t string) bool {
	switch t {
	case "topup", "penjualan", "fee", "lain":
		return true
	}
	return false
}

func rangeFrom(r *http.Request) (any, any) {
	q := r.URL.Query()
	return strOrNil(q.Get("from")), strOrNil(q.Get("to"))
}

func strOrNil(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func idParam(r *http.Request) int64 {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id
}

func currentUser(r *http.Request) *user {
	if u, ok := r.Context().Value(ctxKey("user")).(*user); ok {
		return u
	}
	return &user{}
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeErr(w, http.StatusBadRequest, "JSON tidak valid")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
