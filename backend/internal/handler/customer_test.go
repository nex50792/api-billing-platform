package handler

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func setupCustomerTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	db.Exec(`CREATE TABLE customers (
		id TEXT PRIMARY KEY,
		name TEXT,
		email TEXT UNIQUE,
		password_hash TEXT,
		is_active INTEGER DEFAULT 0,
		plan TEXT DEFAULT 'free',
		subscription_status TEXT,
		created_at TEXT
	)`)
	db.Exec(`CREATE TABLE customer_sessions (
		id TEXT PRIMARY KEY,
		customer_id TEXT,
		token_hash TEXT UNIQUE,
		created_at TEXT,
		expires_at TEXT
	)`)
	db.Exec(`CREATE TABLE customer_verification_codes (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		customer_id TEXT,
		code TEXT,
		expires_at TEXT,
		is_used INTEGER DEFAULT 0,
		created_at TEXT DEFAULT CURRENT_TIMESTAMP
	)`)
	db.Exec(`CREATE TABLE customer_password_resets (
		id TEXT PRIMARY KEY,
		customer_id TEXT NOT NULL,
		token_hash TEXT NOT NULL UNIQUE,
		expires_at TEXT NOT NULL,
		used_at TEXT
	)`)
	return db
}

func TestCustomerSignup_MissingFields(t *testing.T) {
	h := NewCustomerHandler(nil)
	body := bytes.NewBufferString(`{"name":"","email":"","password":""}`)
	req := httptest.NewRequest(http.MethodPost, "/customer/signup", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Signup(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestCustomerSignup_DuplicateActiveEmail(t *testing.T) {
	db := setupCustomerTestDB(t)
	db.Exec(`INSERT INTO customers (id, name, email, password_hash, is_active) VALUES ('existing-id', 'Alice', 'dup@example.com', 'hash', 1)`)

	h := NewCustomerHandler(db)
	body := bytes.NewBufferString(`{"name":"Bob","email":"dup@example.com","password":"somepass"}`)
	req := httptest.NewRequest(http.MethodPost, "/customer/signup", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Signup(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("want 409, got %d", w.Code)
	}
}

func TestCustomerLogin_InvalidCredentials(t *testing.T) {
	db := setupCustomerTestDB(t)

	h := NewCustomerHandler(db)
	body := bytes.NewBufferString(`{"email":"notfound@example.com","password":"pass"}`)
	req := httptest.NewRequest(http.MethodPost, "/customer/login", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Login(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func TestCustomerLogin_WrongPassword(t *testing.T) {
	db := setupCustomerTestDB(t)
	hash, _ := bcrypt.GenerateFromPassword([]byte("correctpass"), bcrypt.MinCost)
	db.Exec(`INSERT INTO customers (id, name, email, password_hash, is_active) VALUES ('cust-wp', 'Test', 'wp@example.com', ?, 1)`, string(hash))

	h := NewCustomerHandler(db)
	body := bytes.NewBufferString(`{"email":"wp@example.com","password":"wrongpass"}`)
	req := httptest.NewRequest(http.MethodPost, "/customer/login", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Login(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func TestCustomerLogin_Success(t *testing.T) {
	db := setupCustomerTestDB(t)
	hash, _ := bcrypt.GenerateFromPassword([]byte("mypassword"), bcrypt.MinCost)
	db.Exec(`INSERT INTO customers (id, name, email, password_hash, is_active) VALUES ('cust-login', 'Test', 'login@example.com', ?, 1)`, string(hash))

	h := NewCustomerHandler(db)
	body := bytes.NewBufferString(`{"email":"login@example.com","password":"mypassword"}`)
	req := httptest.NewRequest(http.MethodPost, "/customer/login", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Login(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var hasSessionCookie bool
	for _, c := range w.Result().Cookies() {
		if c.Name == "customer_session_token" {
			hasSessionCookie = true
			break
		}
	}
	if !hasSessionCookie {
		t.Error("expected customer_session_token cookie to be set")
	}
}

func TestCustomerMe_NoSession(t *testing.T) {
	h := NewCustomerHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/customer/me", nil)
	w := httptest.NewRecorder()

	h.Me(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func TestCustomerMe_InvalidSession(t *testing.T) {
	db := setupCustomerTestDB(t)
	h := NewCustomerHandler(db)
	req := httptest.NewRequest(http.MethodGet, "/customer/me", nil)
	req.AddCookie(&http.Cookie{Name: "customer_session_token", Value: "invalid-token-not-in-db"})
	w := httptest.NewRecorder()

	h.Me(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func TestCustomerLogout_NoCookie(t *testing.T) {
	h := NewCustomerHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/customer/logout", nil)
	w := httptest.NewRecorder()

	h.Logout(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}
}

func TestCustomerVerifyCode_CustomerNotFound(t *testing.T) {
	db := setupCustomerTestDB(t)

	h := NewCustomerHandler(db)
	body := bytes.NewBufferString(`{"email":"nobody@example.com","code":"123456"}`)
	req := httptest.NewRequest(http.MethodPost, "/customer/verify", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.VerifyCode(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", w.Code)
	}
}

func TestCustomerVerifyCode_NoCodeFound(t *testing.T) {
	db := setupCustomerTestDB(t)
	db.Exec(`INSERT INTO customers (id, name, email, password_hash, is_active) VALUES ('cust-nocode-1', 'No Code', 'nocode@example.com', 'hash', 0)`)

	h := NewCustomerHandler(db)
	body := bytes.NewBufferString(`{"email":"nocode@example.com","code":"123456"}`)
	req := httptest.NewRequest(http.MethodPost, "/customer/verify", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.VerifyCode(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "no verification code found") {
		t.Errorf("want 'no verification code found' in body, got: %s", w.Body.String())
	}
}

// expires_at の time.Time スキャンが必要なため PostgreSQL の testDB() を使用する。
func TestCustomerVerifyCode_InvalidCode(t *testing.T) {
	db := testDB()
	if err := db.Ping(); err != nil {
		t.Skipf("test DB not available: %v", err)
	}

	var customerID string
	err := db.QueryRow(
		`INSERT INTO customers (id, name, email, password_hash) VALUES (gen_random_uuid(), 'VC Test', $1, 'hash') RETURNING id`,
		"vc-invalid-code@example.com",
	).Scan(&customerID)
	if err != nil {
		t.Fatalf("failed to insert customer: %v", err)
	}
	t.Cleanup(func() {
		db.Exec(`DELETE FROM customer_verification_codes WHERE customer_id = $1`, customerID)
		db.Exec(`DELETE FROM customers WHERE id = $1`, customerID)
	})
	_, err = db.Exec(
		`INSERT INTO customer_verification_codes (customer_id, code, expires_at) VALUES ($1, '654321', now() + interval '15 minutes')`,
		customerID,
	)
	if err != nil {
		t.Fatalf("failed to insert code: %v", err)
	}

	h := NewCustomerHandler(db)
	body := bytes.NewBufferString(`{"email":"vc-invalid-code@example.com","code":"000000"}`)
	req := httptest.NewRequest(http.MethodPost, "/customer/verify", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.VerifyCode(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestCustomerVerifyCode_Success(t *testing.T) {
	db := testDB()
	if err := db.Ping(); err != nil {
		t.Skipf("test DB not available: %v", err)
	}

	var customerID string
	err := db.QueryRow(
		`INSERT INTO customers (id, name, email, password_hash, is_active) VALUES (gen_random_uuid(), 'VC OK', $1, 'hash', false) RETURNING id`,
		"vc-success@example.com",
	).Scan(&customerID)
	if err != nil {
		t.Fatalf("failed to insert customer: %v", err)
	}
	t.Cleanup(func() {
		db.Exec(`DELETE FROM customer_verification_codes WHERE customer_id = $1`, customerID)
		db.Exec(`DELETE FROM customers WHERE id = $1`, customerID)
	})
	_, err = db.Exec(
		`INSERT INTO customer_verification_codes (customer_id, code, expires_at) VALUES ($1, '123456', now() + interval '15 minutes')`,
		customerID,
	)
	if err != nil {
		t.Fatalf("failed to insert code: %v", err)
	}

	h := NewCustomerHandler(db)
	body := bytes.NewBufferString(`{"email":"vc-success@example.com","code":"123456"}`)
	req := httptest.NewRequest(http.MethodPost, "/customer/verify", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.VerifyCode(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var isActive bool
	db.QueryRow(`SELECT is_active FROM customers WHERE id = $1`, customerID).Scan(&isActive)
	if !isActive {
		t.Error("customer should be activated after valid code")
	}
}

func TestCustomerVerifyCode_ExpiredCode(t *testing.T) {
	db := testDB()
	if err := db.Ping(); err != nil {
		t.Skipf("test DB not available: %v", err)
	}

	var customerID string
	err := db.QueryRow(
		`INSERT INTO customers (id, name, email, password_hash, is_active) VALUES (gen_random_uuid(), 'VC Exp', $1, 'hash', false) RETURNING id`,
		"vc-expired@example.com",
	).Scan(&customerID)
	if err != nil {
		t.Fatalf("failed to insert customer: %v", err)
	}
	t.Cleanup(func() {
		db.Exec(`DELETE FROM customer_verification_codes WHERE customer_id = $1`, customerID)
		db.Exec(`DELETE FROM customers WHERE id = $1`, customerID)
	})
	_, err = db.Exec(
		`INSERT INTO customer_verification_codes (customer_id, code, expires_at) VALUES ($1, '999999', now() - interval '1 minute')`,
		customerID,
	)
	if err != nil {
		t.Fatalf("failed to insert expired code: %v", err)
	}

	h := NewCustomerHandler(db)
	body := bytes.NewBufferString(`{"email":"vc-expired@example.com","code":"999999"}`)
	req := httptest.NewRequest(http.MethodPost, "/customer/verify", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.VerifyCode(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestCustomerSignup_InactiveEmailReissue(t *testing.T) {
	db := testDB()
	if err := db.Ping(); err != nil {
		t.Skipf("test DB not available: %v", err)
	}

	email := "signup-reissue-" + t.Name() + "@example.com"
	var existingID string
	err := db.QueryRow(
		`INSERT INTO customers (id, name, email, password_hash, is_active) VALUES (gen_random_uuid(), 'Old Name', $1, 'oldhash', false) RETURNING id`,
		email,
	).Scan(&existingID)
	if err != nil {
		t.Fatalf("failed to insert inactive customer: %v", err)
	}
	t.Cleanup(func() {
		db.Exec(`DELETE FROM customer_verification_codes WHERE customer_id = $1`, existingID)
		db.Exec(`DELETE FROM customers WHERE id = $1`, existingID)
	})

	h := NewCustomerHandler(db)
	body, _ := json.Marshal(map[string]string{"name": "New Name", "email": email, "password": "newpass123"})
	req := httptest.NewRequest(http.MethodPost, "/customer/signup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Signup(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["message"] != "signup ok (reissued)" {
		t.Errorf("want message='signup ok (reissued)', got %v", resp["message"])
	}
}

func TestCustomerMe_ValidSession(t *testing.T) {
	db := testDB()
	if err := db.Ping(); err != nil {
		t.Skipf("test DB not available: %v", err)
	}

	var customerID string
	err := db.QueryRow(
		`INSERT INTO customers (id, name, email, password_hash, is_active, plan) VALUES (gen_random_uuid(), 'Me Test', $1, 'hash', true, 'pro') RETURNING id`,
		"customer-me-valid@example.com",
	).Scan(&customerID)
	if err != nil {
		t.Fatalf("failed to insert customer: %v", err)
	}
	t.Cleanup(func() {
		db.Exec(`DELETE FROM customer_sessions WHERE customer_id = $1`, customerID)
		db.Exec(`DELETE FROM customers WHERE id = $1`, customerID)
	})

	plainToken, err := generateSessionToken()
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	tokenHash := hashSessionToken(plainToken)
	_, err = db.Exec(
		`INSERT INTO customer_sessions (customer_id, token_hash, expires_at) VALUES ($1, $2, now() + interval '24 hours')`,
		customerID, tokenHash,
	)
	if err != nil {
		t.Fatalf("failed to insert session: %v", err)
	}

	h := NewCustomerHandler(db)
	req := httptest.NewRequest(http.MethodGet, "/customer/me", nil)
	req.AddCookie(&http.Cookie{Name: "customer_session_token", Value: plainToken})
	w := httptest.NewRecorder()

	h.Me(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if resp["email"] != "customer-me-valid@example.com" {
		t.Errorf("want email=customer-me-valid@example.com, got %v", resp["email"])
	}
	if resp["plan"] != "pro" {
		t.Errorf("want plan=pro, got %v", resp["plan"])
	}
}

func TestCustomerForgotPassword_MissingEmail(t *testing.T) {
	h := NewCustomerHandler(nil)
	body := bytes.NewBufferString(`{"email":""}`)
	req := httptest.NewRequest("POST", "/customer/forgot-password", body)
	w := httptest.NewRecorder()

	h.ForgotPassword(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

// ユーザー列挙防止: 存在しないメールでも 200 を返す。
func TestCustomerForgotPassword_EmailNotFound(t *testing.T) {
	db := setupCustomerTestDB(t)
	h := NewCustomerHandler(db)
	body := bytes.NewBufferString(`{"email":"nobody@example.com"}`)
	req := httptest.NewRequest("POST", "/customer/forgot-password", body)
	w := httptest.NewRecorder()

	h.ForgotPassword(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}
}

func TestCustomerForgotPassword_Success(t *testing.T) {
	db := testDB()
	if err := db.Ping(); err != nil {
		t.Skipf("test DB not available: %v", err)
	}

	var customerID string
	err := db.QueryRow(
		`INSERT INTO customers (id, name, email, password_hash, is_active) VALUES (gen_random_uuid(), 'FP Test', $1, 'hash', true) RETURNING id`,
		"fp-success@example.com",
	).Scan(&customerID)
	if err != nil {
		t.Fatalf("failed to insert customer: %v", err)
	}
	t.Cleanup(func() {
		db.Exec(`DELETE FROM customer_password_resets WHERE customer_id = $1`, customerID)
		db.Exec(`DELETE FROM customers WHERE id = $1`, customerID)
	})

	h := NewCustomerHandler(db)
	body := bytes.NewBufferString(`{"email":"fp-success@example.com"}`)
	req := httptest.NewRequest("POST", "/customer/forgot-password", body)
	w := httptest.NewRecorder()

	h.ForgotPassword(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var count int
	db.QueryRow(`SELECT COUNT(*) FROM customer_password_resets WHERE customer_id = $1`, customerID).Scan(&count)
	if count != 1 {
		t.Error("expected 1 reset token to be inserted")
	}
}

func TestCustomerResetPassword_MissingFields(t *testing.T) {
	h := NewCustomerHandler(nil)
	body := bytes.NewBufferString(`{"token":"","password":""}`)
	req := httptest.NewRequest("POST", "/customer/reset-password", body)
	w := httptest.NewRecorder()

	h.ResetPassword(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestCustomerResetPassword_InvalidToken(t *testing.T) {
	db := setupCustomerTestDB(t)
	h := NewCustomerHandler(db)
	body := bytes.NewBufferString(`{"token":"bad-token","password":"newpass123"}`)
	req := httptest.NewRequest("POST", "/customer/reset-password", body)
	w := httptest.NewRecorder()

	h.ResetPassword(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestCustomerResetPassword_ExpiredToken(t *testing.T) {
	db := testDB()
	if err := db.Ping(); err != nil {
		t.Skipf("test DB not available: %v", err)
	}

	var customerID string
	err := db.QueryRow(
		`INSERT INTO customers (id, name, email, password_hash, is_active) VALUES (gen_random_uuid(), 'RP Exp', $1, 'hash', true) RETURNING id`,
		"rp-expired@example.com",
	).Scan(&customerID)
	if err != nil {
		t.Fatalf("failed to insert customer: %v", err)
	}
	plainToken, _ := generateSessionToken()
	tokenHash := hashSessionToken(plainToken)
	resetID := "rp-exp-id"
	db.Exec(
		`INSERT INTO customer_password_resets (id, customer_id, token_hash, expires_at) VALUES ($1, $2, $3, now() - interval '1 minute')`,
		resetID, customerID, tokenHash,
	)
	t.Cleanup(func() {
		db.Exec(`DELETE FROM customer_password_resets WHERE id = $1`, resetID)
		db.Exec(`DELETE FROM customers WHERE id = $1`, customerID)
	})

	h := NewCustomerHandler(db)
	body, _ := json.Marshal(map[string]string{"token": plainToken, "password": "newpass123"})
	req := httptest.NewRequest("POST", "/customer/reset-password", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.ResetPassword(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestCustomerResetPassword_AlreadyUsed(t *testing.T) {
	db := testDB()
	if err := db.Ping(); err != nil {
		t.Skipf("test DB not available: %v", err)
	}

	var customerID string
	err := db.QueryRow(
		`INSERT INTO customers (id, name, email, password_hash, is_active) VALUES (gen_random_uuid(), 'RP Used', $1, 'hash', true) RETURNING id`,
		"rp-used@example.com",
	).Scan(&customerID)
	if err != nil {
		t.Fatalf("failed to insert customer: %v", err)
	}
	plainToken, _ := generateSessionToken()
	tokenHash := hashSessionToken(plainToken)
	resetID := "rp-used-id"
	db.Exec(
		`INSERT INTO customer_password_resets (id, customer_id, token_hash, expires_at, used_at) VALUES ($1, $2, $3, now() + interval '30 minutes', now())`,
		resetID, customerID, tokenHash,
	)
	t.Cleanup(func() {
		db.Exec(`DELETE FROM customer_password_resets WHERE id = $1`, resetID)
		db.Exec(`DELETE FROM customers WHERE id = $1`, customerID)
	})

	h := NewCustomerHandler(db)
	body, _ := json.Marshal(map[string]string{"token": plainToken, "password": "newpass123"})
	req := httptest.NewRequest("POST", "/customer/reset-password", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.ResetPassword(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestCustomerResetPassword_Success(t *testing.T) {
	db := testDB()
	if err := db.Ping(); err != nil {
		t.Skipf("test DB not available: %v", err)
	}

	var customerID string
	err := db.QueryRow(
		`INSERT INTO customers (id, name, email, password_hash, is_active) VALUES (gen_random_uuid(), 'RP OK', $1, 'oldhash', true) RETURNING id`,
		"rp-success@example.com",
	).Scan(&customerID)
	if err != nil {
		t.Fatalf("failed to insert customer: %v", err)
	}
	plainToken, _ := generateSessionToken()
	tokenHash := hashSessionToken(plainToken)
	resetID := "rp-ok-id"
	db.Exec(
		`INSERT INTO customer_password_resets (id, customer_id, token_hash, expires_at) VALUES ($1, $2, $3, now() + interval '30 minutes')`,
		resetID, customerID, tokenHash,
	)
	t.Cleanup(func() {
		db.Exec(`DELETE FROM customer_password_resets WHERE id = $1`, resetID)
		db.Exec(`DELETE FROM customers WHERE id = $1`, customerID)
	})

	h := NewCustomerHandler(db)
	body, _ := json.Marshal(map[string]string{"token": plainToken, "password": "newpass123"})
	req := httptest.NewRequest("POST", "/customer/reset-password", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.ResetPassword(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var newHash string
	db.QueryRow(`SELECT password_hash FROM customers WHERE id = $1`, customerID).Scan(&newHash)
	if newHash == "oldhash" {
		t.Error("password_hash should have been updated")
	}
	var usedAt *string
	db.QueryRow(`SELECT used_at FROM customer_password_resets WHERE id = $1`, resetID).Scan(&usedAt)
	if usedAt == nil {
		t.Error("used_at should be set after successful reset")
	}
}

func TestCustomerLogout_WithCookie(t *testing.T) {
	db := setupCustomerTestDB(t)
	plainToken, err := generateSessionToken()
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	tokenHash := hashSessionToken(plainToken)
	db.Exec(`INSERT INTO customer_sessions (id, customer_id, token_hash, expires_at) VALUES ('sess-logout', 'cust-logout', $1, '2099-01-01')`, tokenHash)

	h := NewCustomerHandler(db)
	req := httptest.NewRequest(http.MethodPost, "/customer/logout", nil)
	req.AddCookie(&http.Cookie{Name: "customer_session_token", Value: plainToken})
	w := httptest.NewRecorder()

	h.Logout(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}
	var count int
	db.QueryRow(`SELECT COUNT(*) FROM customer_sessions WHERE token_hash = $1`, tokenHash).Scan(&count)
	if count != 0 {
		t.Error("session should be deleted from DB after logout")
	}
}

func TestCustomerResetPassword_SessionsInvalidated(t *testing.T) {
	db := testDB()
	if err := db.Ping(); err != nil {
		t.Skipf("test DB not available: %v", err)
	}

	var customerID string
	if err := db.QueryRow(
		`INSERT INTO customers (id, name, email, password_hash, is_active) VALUES (gen_random_uuid(), 'RP Sess', $1, 'oldhash', true) RETURNING id`,
		"rp-sessions@example.com",
	).Scan(&customerID); err != nil {
		t.Fatalf("failed to insert customer: %v", err)
	}

	plainToken, _ := generateSessionToken()
	tokenHash := hashSessionToken(plainToken)
	resetID := "rp-sess-id"
	db.Exec(
		`INSERT INTO customer_password_resets (id, customer_id, token_hash, expires_at) VALUES ($1, $2, $3, now() + interval '30 minutes')`,
		resetID, customerID, tokenHash,
	)

	sess1Token, _ := generateSessionToken()
	sess2Token, _ := generateSessionToken()
	db.Exec(`INSERT INTO customer_sessions (customer_id, token_hash, expires_at) VALUES ($1, $2, now() + interval '24 hours')`, customerID, hashSessionToken(sess1Token))
	db.Exec(`INSERT INTO customer_sessions (customer_id, token_hash, expires_at) VALUES ($1, $2, now() + interval '24 hours')`, customerID, hashSessionToken(sess2Token))

	t.Cleanup(func() {
		db.Exec(`DELETE FROM customer_sessions WHERE customer_id = $1`, customerID)
		db.Exec(`DELETE FROM customer_password_resets WHERE id = $1`, resetID)
		db.Exec(`DELETE FROM customers WHERE id = $1`, customerID)
	})

	h := NewCustomerHandler(db)
	body, _ := json.Marshal(map[string]string{"token": plainToken, "password": "newpass123"})
	req := httptest.NewRequest("POST", "/customer/reset-password", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.ResetPassword(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var sessCount int
	db.QueryRow(`SELECT COUNT(*) FROM customer_sessions WHERE customer_id = $1`, customerID).Scan(&sessCount)
	if sessCount != 0 {
		t.Errorf("want all sessions deleted after password reset, got %d remaining", sessCount)
	}
}

func TestCustomerChangePassword_MissingFields(t *testing.T) {
	h := NewCustomerHandler(nil)
	body := bytes.NewBufferString(`{"current_password":"","new_password":""}`)
	req := httptest.NewRequest("POST", "/customer/change-password", body)
	w := httptest.NewRecorder()

	h.ChangePassword(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestCustomerChangePassword_ShortPassword(t *testing.T) {
	h := NewCustomerHandler(nil)
	body := bytes.NewBufferString(`{"current_password":"oldpass123","new_password":"short"}`)
	req := httptest.NewRequest("POST", "/customer/change-password", body)
	w := httptest.NewRecorder()

	h.ChangePassword(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestCustomerChangePassword_NoSession(t *testing.T) {
	db := setupCustomerTestDB(t)
	h := NewCustomerHandler(db)
	body := bytes.NewBufferString(`{"current_password":"oldpass123","new_password":"newpass123"}`)
	req := httptest.NewRequest("POST", "/customer/change-password", body)
	w := httptest.NewRecorder()

	h.ChangePassword(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func TestCustomerChangePassword_WrongCurrentPassword(t *testing.T) {
	db := testDB()
	if err := db.Ping(); err != nil {
		t.Skipf("test DB not available: %v", err)
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte("correctpass123"), bcrypt.MinCost)
	var customerID string
	err := db.QueryRow(
		`INSERT INTO customers (id, name, email, password_hash, is_active) VALUES (gen_random_uuid(), 'CP Wrong', $1, $2, true) RETURNING id`,
		"cp-wrong@example.com", string(hash),
	).Scan(&customerID)
	if err != nil {
		t.Fatalf("failed to insert customer: %v", err)
	}
	plainToken, _ := generateSessionToken()
	tokenHash := hashSessionToken(plainToken)
	db.Exec(`INSERT INTO customer_sessions (customer_id, token_hash, expires_at) VALUES ($1, $2, now() + interval '24 hours')`, customerID, tokenHash)
	t.Cleanup(func() {
		db.Exec(`DELETE FROM customer_sessions WHERE customer_id = $1`, customerID)
		db.Exec(`DELETE FROM customers WHERE id = $1`, customerID)
	})

	h := NewCustomerHandler(db)
	body, _ := json.Marshal(map[string]string{"current_password": "wrongpass", "new_password": "newpass123"})
	req := httptest.NewRequest("POST", "/customer/change-password", bytes.NewReader(body))
	req.AddCookie(&http.Cookie{Name: "customer_session_token", Value: plainToken})
	w := httptest.NewRecorder()

	h.ChangePassword(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func TestCustomerChangePassword_Success(t *testing.T) {
	db := testDB()
	if err := db.Ping(); err != nil {
		t.Skipf("test DB not available: %v", err)
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte("oldpass123"), bcrypt.MinCost)
	var customerID string
	err := db.QueryRow(
		`INSERT INTO customers (id, name, email, password_hash, is_active) VALUES (gen_random_uuid(), 'CP OK', $1, $2, true) RETURNING id`,
		"cp-success@example.com", string(hash),
	).Scan(&customerID)
	if err != nil {
		t.Fatalf("failed to insert customer: %v", err)
	}
	plainToken, _ := generateSessionToken()
	tokenHash := hashSessionToken(plainToken)
	db.Exec(`INSERT INTO customer_sessions (customer_id, token_hash, expires_at) VALUES ($1, $2, now() + interval '24 hours')`, customerID, tokenHash)
	t.Cleanup(func() {
		db.Exec(`DELETE FROM customer_sessions WHERE customer_id = $1`, customerID)
		db.Exec(`DELETE FROM customers WHERE id = $1`, customerID)
	})

	h := NewCustomerHandler(db)
	body, _ := json.Marshal(map[string]string{"current_password": "oldpass123", "new_password": "newpass123"})
	req := httptest.NewRequest("POST", "/customer/change-password", bytes.NewReader(body))
	req.AddCookie(&http.Cookie{Name: "customer_session_token", Value: plainToken})
	w := httptest.NewRecorder()

	h.ChangePassword(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var newHash string
	db.QueryRow(`SELECT password_hash FROM customers WHERE id = $1`, customerID).Scan(&newHash)
	if err := bcrypt.CompareHashAndPassword([]byte(newHash), []byte("newpass123")); err != nil {
		t.Error("password_hash should have been updated to newpass123")
	}
}

// PostgreSQL の now() を使うため testDB() を使用し、DB が利用不可の場合はスキップする。
func TestCustomerSignup_Success(t *testing.T) {
	db := testDB()
	if err := db.Ping(); err != nil {
		t.Skipf("test DB not available: %v", err)
	}

	email := "signup-test-" + t.Name() + "@example.com"
	t.Cleanup(func() {
		db.Exec(`DELETE FROM customer_verification_codes WHERE customer_id IN (SELECT id FROM customers WHERE email = $1)`, email)
		db.Exec(`DELETE FROM customers WHERE email = $1`, email)
	})

	h := NewCustomerHandler(db)
	body, _ := json.Marshal(map[string]string{"name": "Test User", "email": email, "password": "testpass123"})
	req := httptest.NewRequest(http.MethodPost, "/customer/signup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Signup(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCustomerSignup_RateLimit(t *testing.T) {
	h := NewCustomerHandler(nil)

	const testIP = "10.1.0.1"
	send := func() int {
		// パスワードを7文字にしてDBアクセス前のバリデーション（len<8）で止める
		body := bytes.NewBufferString(`{"name":"Test","email":"rl@example.com","password":"short12"}`)
		req := httptest.NewRequest(http.MethodPost, "/customer/signup", body)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Forwarded-For", testIP)
		w := httptest.NewRecorder()
		h.Signup(w, req)
		return w.Code
	}

	for i := 0; i < 5; i++ {
		if code := send(); code == http.StatusTooManyRequests {
			t.Fatalf("attempt %d: unexpected 429 before limit", i+1)
		}
	}
	if code := send(); code != http.StatusTooManyRequests {
		t.Errorf("want 429 on 6th attempt, got %d", code)
	}
}

func TestCustomerSignup_InvalidEmail(t *testing.T) {
	h := NewCustomerHandler(nil)
	body := bytes.NewBufferString(`{"name":"Test","email":"notanemail","password":"validpass123"}`)
	req := httptest.NewRequest(http.MethodPost, "/customer/signup", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Signup(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestCustomerSignup_ShortPassword(t *testing.T) {
	h := NewCustomerHandler(nil)
	body := bytes.NewBufferString(`{"name":"Test","email":"test@example.com","password":"short"}`)
	req := httptest.NewRequest(http.MethodPost, "/customer/signup", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Signup(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestCustomerLogin_InactiveAccount(t *testing.T) {
	db := setupCustomerTestDB(t)
	hash, _ := bcrypt.GenerateFromPassword([]byte("mypassword"), bcrypt.MinCost)
	db.Exec(`INSERT INTO customers (id, name, email, password_hash, is_active) VALUES ('inactive-id', 'Inactive', 'inactive@example.com', ?, 0)`, string(hash))

	h := NewCustomerHandler(db)
	body := bytes.NewBufferString(`{"email":"inactive@example.com","password":"mypassword"}`)
	req := httptest.NewRequest(http.MethodPost, "/customer/login", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Login(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func TestCustomerResetPassword_ShortPassword(t *testing.T) {
	h := NewCustomerHandler(nil)
	body := bytes.NewBufferString(`{"token":"sometoken","password":"short"}`)
	req := httptest.NewRequest("POST", "/customer/reset-password", body)
	w := httptest.NewRecorder()

	h.ResetPassword(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestCustomerForgotPassword_RateLimit(t *testing.T) {
	db := setupCustomerTestDB(t)
	h := NewCustomerHandler(db)

	const testIP = "10.2.0.1"
	send := func() int {
		body := bytes.NewBufferString(`{"email":"someone@example.com"}`)
		req := httptest.NewRequest("POST", "/customer/forgot-password", body)
		req.Header.Set("X-Forwarded-For", testIP)
		w := httptest.NewRecorder()
		h.ForgotPassword(w, req)
		return w.Code
	}

	for i := 0; i < 3; i++ {
		if code := send(); code == http.StatusTooManyRequests {
			t.Fatalf("attempt %d: unexpected 429 before limit", i+1)
		}
	}
	if code := send(); code != http.StatusTooManyRequests {
		t.Errorf("want 429 on 4th attempt, got %d", code)
	}
}

func TestCustomerSignup_OldVerificationCodesInvalidated(t *testing.T) {
	db := testDB()
	if err := db.Ping(); err != nil {
		t.Skipf("test DB not available: %v", err)
	}

	email := "reissue-codes-" + t.Name() + "@example.com"
	var customerID string
	err := db.QueryRow(
		`INSERT INTO customers (id, name, email, password_hash, is_active) VALUES (gen_random_uuid(), 'Old', $1, 'oldhash', false) RETURNING id`,
		email,
	).Scan(&customerID)
	if err != nil {
		t.Fatalf("failed to insert customer: %v", err)
	}
	t.Cleanup(func() {
		db.Exec(`DELETE FROM customer_verification_codes WHERE customer_id = $1`, customerID)
		db.Exec(`DELETE FROM customers WHERE id = $1`, customerID)
	})

	_, err = db.Exec(
		`INSERT INTO customer_verification_codes (customer_id, code, expires_at) VALUES ($1, '111111', now() + interval '15 minutes')`,
		customerID,
	)
	if err != nil {
		t.Fatalf("failed to insert old code: %v", err)
	}

	h := NewCustomerHandler(db)
	body, _ := json.Marshal(map[string]string{"name": "New", "email": email, "password": "newpass123"})
	req := httptest.NewRequest(http.MethodPost, "/customer/signup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Signup(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d: %s", w.Code, w.Body.String())
	}
	var oldCodeUsed bool
	db.QueryRow(`SELECT is_used FROM customer_verification_codes WHERE customer_id = $1 AND code = '111111'`, customerID).Scan(&oldCodeUsed)
	if !oldCodeUsed {
		t.Error("old verification code should be marked as used after re-registration")
	}
}

func TestCustomerSignup_MailerFailure_StillSucceeds(t *testing.T) {
	db := testDB()
	if err := db.Ping(); err != nil {
		t.Skipf("test DB not available: %v", err)
	}

	email := "mailer-fail-" + t.Name() + "@example.com"
	t.Cleanup(func() {
		db.Exec(`DELETE FROM customer_verification_codes WHERE customer_id IN (SELECT id FROM customers WHERE email = $1)`, email)
		db.Exec(`DELETE FROM customers WHERE email = $1`, email)
	})

	h := NewCustomerHandler(db)
	h.Mailer = &FailingMockMailer{}
	body, _ := json.Marshal(map[string]string{"name": "Test", "email": email, "password": "testpass123"})
	req := httptest.NewRequest(http.MethodPost, "/customer/signup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Signup(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("want 201 even when mailer fails, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCustomerChangePassword_OtherSessionsInvalidated(t *testing.T) {
	db := testDB()
	if err := db.Ping(); err != nil {
		t.Skipf("test DB not available: %v", err)
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte("oldpass123"), bcrypt.MinCost)
	var customerID string
	err := db.QueryRow(
		`INSERT INTO customers (id, name, email, password_hash, is_active) VALUES (gen_random_uuid(), 'SI Test', $1, $2, true) RETURNING id`,
		"cp-si@example.com", string(hash),
	).Scan(&customerID)
	if err != nil {
		t.Fatalf("failed to insert customer: %v", err)
	}

	currentToken, _ := generateSessionToken()
	otherToken, _ := generateSessionToken()
	db.Exec(`INSERT INTO customer_sessions (customer_id, token_hash, expires_at) VALUES ($1, $2, now() + interval '24 hours')`, customerID, hashSessionToken(currentToken))
	db.Exec(`INSERT INTO customer_sessions (customer_id, token_hash, expires_at) VALUES ($1, $2, now() + interval '24 hours')`, customerID, hashSessionToken(otherToken))
	t.Cleanup(func() {
		db.Exec(`DELETE FROM customer_sessions WHERE customer_id = $1`, customerID)
		db.Exec(`DELETE FROM customers WHERE id = $1`, customerID)
	})

	h := NewCustomerHandler(db)
	body, _ := json.Marshal(map[string]string{"current_password": "oldpass123", "new_password": "newpass123"})
	req := httptest.NewRequest("POST", "/customer/change-password", bytes.NewReader(body))
	req.AddCookie(&http.Cookie{Name: "customer_session_token", Value: currentToken})
	w := httptest.NewRecorder()

	h.ChangePassword(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var currentCount int
	db.QueryRow(`SELECT COUNT(*) FROM customer_sessions WHERE customer_id = $1 AND token_hash = $2`, customerID, hashSessionToken(currentToken)).Scan(&currentCount)
	if currentCount != 1 {
		t.Error("current session should still be valid after password change")
	}
	var otherCount int
	db.QueryRow(`SELECT COUNT(*) FROM customer_sessions WHERE customer_id = $1 AND token_hash = $2`, customerID, hashSessionToken(otherToken)).Scan(&otherCount)
	if otherCount != 0 {
		t.Error("other sessions should be invalidated after password change")
	}
}

func TestCustomerLogin_RateLimit(t *testing.T) {
	db := setupCustomerTestDB(t)
	h := NewCustomerHandler(db)

	const testIP = "10.6.0.1"
	send := func() int {
		body := bytes.NewBufferString(`{"email":"nobody@example.com","password":"wrongpass"}`)
		req := httptest.NewRequest(http.MethodPost, "/customer/login", body)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Forwarded-For", testIP)
		w := httptest.NewRecorder()
		h.Login(w, req)
		return w.Code
	}

	for i := 0; i < 10; i++ {
		if code := send(); code == http.StatusTooManyRequests {
			t.Fatalf("attempt %d: unexpected 429 before limit", i+1)
		}
	}
	if code := send(); code != http.StatusTooManyRequests {
		t.Errorf("want 429 on 11th attempt, got %d", code)
	}
}

func TestCustomerSignup_InvalidJSON(t *testing.T) {
	h := NewCustomerHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/customer/signup", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Signup(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestCustomerChangePassword_InvalidJSON(t *testing.T) {
	h := NewCustomerHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/customer/change-password", bytes.NewReader([]byte("not-json")))
	w := httptest.NewRecorder()

	h.ChangePassword(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestCustomerResetPassword_InvalidJSON(t *testing.T) {
	h := NewCustomerHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/customer/reset-password", bytes.NewReader([]byte("not-json")))
	w := httptest.NewRecorder()

	h.ResetPassword(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestCustomerLogin_InvalidJSON(t *testing.T) {
	db := setupCustomerTestDB(t)
	h := NewCustomerHandler(db)
	req := httptest.NewRequest(http.MethodPost, "/customer/login", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Login(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestCustomerVerifyCode_InvalidJSON(t *testing.T) {
	db := setupCustomerTestDB(t)
	h := NewCustomerHandler(db)
	req := httptest.NewRequest(http.MethodPost, "/customer/verify", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.VerifyCode(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestCustomerVerifyCode_RateLimit(t *testing.T) {
	db := setupCustomerTestDB(t)
	h := NewCustomerHandler(db)

	const testIP = "10.0.0.1"
	send := func() int {
		body := bytes.NewBufferString(`{"email":"nobody@example.com","code":"000000"}`)
		req := httptest.NewRequest(http.MethodPost, "/customer/verify", body)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Forwarded-For", testIP)
		w := httptest.NewRecorder()
		h.VerifyCode(w, req)
		return w.Code
	}

	for i := 0; i < 5; i++ {
		if code := send(); code == http.StatusTooManyRequests {
			t.Fatalf("attempt %d: unexpected 429 before limit", i+1)
		}
	}
	if code := send(); code != http.StatusTooManyRequests {
		t.Errorf("want 429 on 6th attempt, got %d", code)
	}
}
