package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func newTestApp(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := migrate(db, false); err != nil {
		t.Fatal(err)
	}
	a := &app{db: db, secret: []byte("01234567890123456789012345678901")}
	srv := httptest.NewServer(a.routes())
	t.Cleanup(func() { srv.Close(); db.Close() })
	return srv, srv.URL
}

func req(t *testing.T, method, url, token string, body any) (int, map[string]any) {
	t.Helper()
	var buf io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		buf = bytes.NewReader(b)
	}
	r, _ := http.NewRequest(method, url, buf)
	r.Header.Set("Content-Type", "application/json")
	if token != "" {
		r.AddCookie(&http.Cookie{Name: "token", Value: token})
	}
	res, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	data := map[string]any{}
	json.NewDecoder(res.Body).Decode(&data)
	return res.StatusCode, data
}

func login(t *testing.T, url, username, password string) string {
	t.Helper()
	r, _ := http.NewRequest("POST", url+"/auth/login", bytes.NewReader([]byte(`{"username":"`+username+`","password":"`+password+`"}`)))
	r.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("login gagal: %d", res.StatusCode)
	}
	for _, c := range res.Cookies() {
		if c.Name == "token" {
			return c.Value
		}
	}
	t.Fatal("cookie token tidak ada")
	return ""
}

func TestFlow(t *testing.T) {
	_, url := newTestApp(t)

	code, _ := req(t, "POST", url+"/auth/register", "", map[string]string{"username": "admin", "password": "rahasia123"})
	if code != 201 {
		t.Fatalf("register: %d", code)
	}
	if code, _ := req(t, "POST", url+"/auth/register", "", map[string]string{"username": "x", "password": "y"}); code != 403 {
		t.Fatalf("registrasi kedua harus diblokir, dapat %d", code)
	}

	// login langsung, ambil token via manual JWT (set cookie di client browser, di test pakai token)
	token := login(t, url, "admin", "rahasia123")

	// tanpa auth harus ditolak
	if code, _ := req(t, "GET", url+"/api/accounts", "", nil); code != 401 {
		t.Fatalf("tanpa login harus 401, dapat %d", code)
	}

	code, d := req(t, "POST", url+"/api/accounts", token, map[string]any{"name": "AkunA"})
	if code != 201 {
		t.Fatalf("buat akun: %d %v", code, d)
	}
	accA := int64(d["id"].(float64))
	code, d = req(t, "POST", url+"/api/accounts", token, map[string]any{"name": "AkunB", "initial_balance_robux": 1000})
	if code != 201 {
		t.Fatalf("buat akun: %d %v", code, d)
	}
	accB := int64(d["id"].(float64))

	// topup 500 R$ @ 40 IDR + fee
	code, _ = req(t, "POST", url+"/api/transactions", token, map[string]any{
		"type": "topup", "account_id": accA, "robux_amount": 500, "rate_idr": 40, "fee_idr": 1000})
	if code != 201 {
		t.Fatalf("topup: %d", code)
	}
	// penjualan 300 R$ @ 60 IDR ke pembeli
	code, _ = req(t, "POST", url+"/api/transactions", token, map[string]any{
		"type": "penjualan", "account_id": accA, "counterpart": "Budi", "robux_amount": 300, "rate_idr": 60})
	if code != 201 {
		t.Fatalf("penjualan: %d", code)
	}
	// transfer 100 dari A ke B
	code, _ = req(t, "POST", url+"/api/transactions/transfer", token, map[string]any{
		"from_account_id": accA, "to_account_id": accB, "robux_amount": 100})
	if code != 201 {
		t.Fatalf("transfer: %d", code)
	}
	// transfer melebihi saldo harus ditolak (A sisa 100 setelah topup500-jual300-trf100)
	code, _ = req(t, "POST", url+"/api/transactions/transfer", token, map[string]any{
		"from_account_id": accA, "to_account_id": accB, "robux_amount": 500})
	if code != 400 {
		t.Fatalf("transfer melebihi saldo harus ditolak, dapat %d", code)
	}

	// fee dengan kolom robux terisi tidak boleh menambah saldo
	if code, d := req(t, "POST", url+"/api/transactions", token, map[string]any{
		"type": "fee", "account_id": accA, "robux_amount": 999, "fee_idr": 0}); code != 201 {
		t.Fatalf("fee: %d %v", code, d)
	}
	// penjualan melebihi saldo harus ditolak (A sisa 100)
	if code, _ := req(t, "POST", url+"/api/transactions", token, map[string]any{
		"type": "penjualan", "account_id": accA, "robux_amount": 101, "rate_idr": 60}); code != 400 {
		t.Fatalf("penjualan melebihi saldo harus ditolak, dapat %d", code)
	}
	// status aneh ditolak, tanggal format form (T) diterima
	if code, _ := req(t, "POST", url+"/api/transactions", token, map[string]any{
		"type": "topup", "account_id": accB, "robux_amount": 1, "status": "ngasal"}); code != 400 {
		t.Fatalf("status tidak valid harus ditolak, dapat %d", code)
	}

	code, d = req(t, "GET", url+"/api/summary", token, nil)
	if code != 200 {
		t.Fatalf("summary: %d", code)
	}
	if d["income"].(float64) != 18000 { // 300*60
		t.Fatalf("income salah: %v", d["income"])
	}
	// harga beli rata-rata = (500*40+1000)/500 = 42; HPP = 300*42 = 12600
	if d["cost"].(float64) != 12600 {
		t.Fatalf("cost salah: %v", d["cost"])
	}
	if d["profit"].(float64) != 5400 {
		t.Fatalf("profit salah: %v", d["profit"])
	}
	accs := d["accounts"].([]any)
	var aBal, bBal float64
	for _, x := range accs {
		m := x.(map[string]any)
		if m["name"] == "AkunA" {
			aBal = m["current_robux"].(float64)
		}
		if m["name"] == "AkunB" {
			bBal = m["current_robux"].(float64)
		}
	}
	if aBal != 100 || bBal != 1100 { // A: 0+500-300-100; B: 1000+100
		t.Fatalf("saldo salah: A=%v B=%v", aBal, bBal)
	}

	// export CSV
	srv2, _ := http.NewRequest("GET", url+"/api/report/export", nil)
	srv2.AddCookie(&http.Cookie{Name: "token", Value: token})
	res, err := http.DefaultClient.Do(srv2)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 || !bytes.Contains(body, []byte("penjualan")) {
		t.Fatalf("CSV gagal: %d", res.StatusCode)
	}
	// hapus akun = nonaktif; transaksi tetap ada
	if code, _ := req(t, "DELETE", url+"/api/accounts/"+strconv.FormatInt(accA, 10), token, nil); code != 200 {
		t.Fatalf("hapus akun: %d", code)
	}
	if code, d := req(t, "GET", url+"/api/summary", token, nil); code != 200 || d["income"].(float64) != 18000 {
		t.Fatalf("riwayat hilang setelah akun dinonaktifkan: %v", d["income"])
	}
	_ = os.Getenv
}

func TestPerUserData(t *testing.T) {
	_, url := newTestApp(t)
	req(t, "POST", url+"/auth/register", "", map[string]string{"username": "admin", "password": "rahasia123"})
	admin := login(t, url, "admin", "rahasia123")
	req(t, "POST", url+"/api/users", admin, map[string]string{"username": "budi", "password": "budi1234", "role": "staff"})
	req(t, "POST", url+"/api/users", admin, map[string]string{"username": "sari", "password": "sari1234", "role": "staff"})
	budi := login(t, url, "budi", "budi1234")
	sari := login(t, url, "sari", "sari1234")

	// staff boleh menambah akun sendiri
	code, d := req(t, "POST", url+"/api/accounts", budi, map[string]any{"name": "AkunBudi", "initial_balance_robux": 1000})
	if code != 201 {
		t.Fatalf("staff harus bisa tambah akun, dapat %d %v", code, d)
	}
	budiAcc := int64(d["id"].(float64))
	_, d = req(t, "POST", url+"/api/accounts", sari, map[string]any{"name": "AkunSari", "initial_balance_robux": 500})
	sariAcc := int64(d["id"].(float64))

	if code, _ := req(t, "POST", url+"/api/transactions", budi, map[string]any{
		"type": "penjualan", "account_id": budiAcc, "robux_amount": 100, "rate_idr": 100}); code != 201 {
		t.Fatalf("budi jual: %d", code)
	}
	// budi tidak boleh pakai akun sari
	if code, _ := req(t, "POST", url+"/api/transactions", budi, map[string]any{
		"type": "topup", "account_id": sariAcc, "robux_amount": 1}); code != 400 {
		t.Fatalf("budi tidak boleh transaksi di akun sari, dapat %d", code)
	}
	if code, _ := req(t, "POST", url+"/api/transactions/transfer", budi, map[string]any{
		"from_account_id": budiAcc, "to_account_id": sariAcc, "robux_amount": 1}); code != 400 {
		t.Fatalf("budi tidak boleh transfer ke akun sari, dapat %d", code)
	}
	if code, _ := req(t, "PUT", url+"/api/accounts/"+strconv.FormatInt(sariAcc, 10), budi, map[string]any{"name": "hack"}); code != 404 {
		t.Fatalf("budi tidak boleh edit akun sari, dapat %d", code)
	}

	count := func(tok string) int {
		r, _ := http.NewRequest("GET", url+"/api/accounts", nil)
		r.AddCookie(&http.Cookie{Name: "token", Value: tok})
		res, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var list []any
		json.NewDecoder(res.Body).Decode(&list)
		return len(list)
	}
	if count(budi) != 1 || count(sari) != 1 || count(admin) != 2 {
		t.Fatalf("jumlah akun terlihat salah: budi=%d sari=%d admin=%d", count(budi), count(sari), count(admin))
	}
	_, d = req(t, "GET", url+"/api/summary", sari, nil)
	if d["income"].(float64) != 0 {
		t.Fatalf("sari tidak boleh melihat penjualan budi: %v", d["income"])
	}
	_, d = req(t, "GET", url+"/api/summary", admin, nil)
	if d["income"].(float64) != 10000 {
		t.Fatalf("admin harus melihat semua: %v", d["income"])
	}
}

func TestPurgeAccount(t *testing.T) {
	_, url := newTestApp(t)
	req(t, "POST", url+"/auth/register", "", map[string]string{"username": "admin", "password": "rahasia123"})
	admin := login(t, url, "admin", "rahasia123")
	req(t, "POST", url+"/api/users", admin, map[string]string{"username": "budi", "password": "budi1234", "role": "staff"})
	budi := login(t, url, "budi", "budi1234")

	_, d := req(t, "POST", url+"/api/accounts", budi, map[string]any{"username_roblox": "a1", "initial_balance_robux": 100})
	a1 := strconv.FormatInt(int64(d["id"].(float64)), 10)
	_, d = req(t, "POST", url+"/api/accounts", budi, map[string]any{"username_roblox": "a2", "initial_balance_robux": 100})
	a2id := int64(d["id"].(float64))
	a2 := strconv.FormatInt(a2id, 10)
	_, d = req(t, "POST", url+"/api/accounts", admin, map[string]any{"username_roblox": "punyaAdmin"})
	adminAcc := strconv.FormatInt(int64(d["id"].(float64)), 10)

	req(t, "POST", url+"/api/transactions", budi, map[string]any{"type": "penjualan", "account_id": a2id, "robux_amount": 10, "rate_idr": 100})

	// budi tidak boleh hapus akun admin
	if code, _ := req(t, "DELETE", url+"/api/accounts/"+adminAcc+"?permanent=1", budi, nil); code != 404 {
		t.Fatalf("hapus akun orang lain harus 404, dapat %d", code)
	}
	// akun tanpa transfer: terhapus beserta transaksinya
	if code, d := req(t, "DELETE", url+"/api/accounts/"+a2+"?permanent=1", budi, nil); code != 200 || d["transactions_deleted"].(float64) != 1 {
		t.Fatalf("hapus akun: %d %v", code, d)
	}
	if _, d := req(t, "GET", url+"/api/summary", budi, nil); d["income"].(float64) != 0 {
		t.Fatalf("transaksi akun terhapus masih terhitung: %v", d["income"])
	}
	// akun dengan transfer: ditolak
	_, d = req(t, "POST", url+"/api/accounts", budi, map[string]any{"username_roblox": "a3"})
	a3 := int64(d["id"].(float64))
	a1id, _ := strconv.ParseInt(a1, 10, 64)
	req(t, "POST", url+"/api/transactions/transfer", budi, map[string]any{"from_account_id": a1id, "to_account_id": a3, "robux_amount": 5})
	if code, _ := req(t, "DELETE", url+"/api/accounts/"+a1+"?permanent=1", budi, nil); code != 400 {
		t.Fatalf("akun dengan transfer harus ditolak, dapat %d", code)
	}
}

func TestDeleteUser(t *testing.T) {
	_, url := newTestApp(t)
	req(t, "POST", url+"/auth/register", "", map[string]string{"username": "admin", "password": "rahasia123"})
	admin := login(t, url, "admin", "rahasia123")
	req(t, "POST", url+"/api/users", admin, map[string]string{"username": "budi", "password": "budi1234", "role": "staff"})
	budi := login(t, url, "budi", "budi1234")
	_, d := req(t, "POST", url+"/api/accounts", budi, map[string]any{"username_roblox": "b1", "initial_balance_robux": 100})
	acc := int64(d["id"].(float64))
	req(t, "POST", url+"/api/transactions", budi, map[string]any{"type": "penjualan", "account_id": acc, "robux_amount": 10, "rate_idr": 100})

	_, me := req(t, "GET", url+"/auth/me", admin, nil)
	adminID := strconv.FormatInt(int64(me["id"].(float64)), 10)
	_, bme := req(t, "GET", url+"/auth/me", budi, nil)
	budiID := strconv.FormatInt(int64(bme["id"].(float64)), 10)

	if code, _ := req(t, "DELETE", url+"/api/users/"+adminID, admin, nil); code != 400 {
		t.Fatalf("hapus diri sendiri harus ditolak, dapat %d", code)
	}
	if code, _ := req(t, "DELETE", url+"/api/users/"+adminID, budi, nil); code != 403 {
		t.Fatalf("staff tidak boleh hapus pengguna, dapat %d", code)
	}
	code, d := req(t, "DELETE", url+"/api/users/"+budiID, admin, nil)
	if code != 200 || d["accounts_deleted"].(float64) != 1 || d["transactions_deleted"].(float64) != 1 {
		t.Fatalf("hapus budi: %d %v", code, d)
	}
	// token budi langsung tidak berlaku
	if code, _ := req(t, "GET", url+"/auth/me", budi, nil); code != 401 {
		t.Fatalf("token pengguna terhapus harus 401, dapat %d", code)
	}
	if _, d := req(t, "GET", url+"/api/summary", admin, nil); d["income"].(float64) != 0 {
		t.Fatalf("transaksi budi masih terhitung: %v", d["income"])
	}
}
