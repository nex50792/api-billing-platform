package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"database/sql"

	_ "github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func setupProviderLoginDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	db.Exec(`CREATE TABLE providers (
		id TEXT PRIMARY KEY,
		name TEXT,
		email TEXT UNIQUE,
		password_hash TEXT,
		is_active INTEGER DEFAULT 0
	)`)
	db.Exec(`CREATE TABLE provider_sessions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		provider_id TEXT,
		token_hash TEXT UNIQUE,
		expires_at TEXT
	)`)
	return db
}

func TestProviderLogin_InvalidCredentials(t *testing.T) {
	db := setupProviderLoginDB(t)
	h := NewProviderHandler(db, &MockMailer{})

	body, _ := json.Marshal(map[string]string{"email": "nobody@example.com", "password": "pass"})
	req := httptest.NewRequest(http.MethodPost, "/provider/login", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Login(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func TestProviderLogin_WrongPassword(t *testing.T) {
	db := setupProviderLoginDB(t)
	h := NewProviderHandler(db, &MockMailer{})

	hash, _ := bcrypt.GenerateFromPassword([]byte("correct-pass"), bcrypt.MinCost)
	id := uuid.New().String()
	db.Exec(
		`INSERT INTO providers (id, name, email, password_hash, is_active) VALUES ($1, $2, $3, $4, 1)`,
		id, "Test", "test@example.com", string(hash),
	)

	body, _ := json.Marshal(map[string]string{"email": "test@example.com", "password": "wrong-pass"})
	req := httptest.NewRequest(http.MethodPost, "/provider/login", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Login(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func TestProviderLogin_Success(t *testing.T) {
	db := setupProviderLoginDB(t)
	h := NewProviderHandler(db, &MockMailer{})

	hash, _ := bcrypt.GenerateFromPassword([]byte("correct-pass"), bcrypt.MinCost)
	id := uuid.New().String()
	db.Exec(
		`INSERT INTO providers (id, name, email, password_hash, is_active) VALUES ($1, $2, $3, $4, 1)`,
		id, "Test", "login-ok@example.com", string(hash),
	)

	body, _ := json.Marshal(map[string]string{"email": "login-ok@example.com", "password": "correct-pass"})
	req := httptest.NewRequest(http.MethodPost, "/provider/login", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Login(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var found bool
	for _, c := range w.Result().Cookies() {
		if c.Name == "provider_session_token" && c.Value != "" {
			found = true
		}
	}
	if !found {
		t.Error("expected provider_session_token cookie to be set")
	}
}

func TestProviderLogout_NoCookie(t *testing.T) {
	h := NewProviderHandler(nil, &MockMailer{})
	req := httptest.NewRequest(http.MethodPost, "/provider/logout", nil)
	w := httptest.NewRecorder()

	h.Logout(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}
}

func TestProviderMe_NoSession(t *testing.T) {
	h := NewProviderHandler(nil, &MockMailer{})
	req := httptest.NewRequest(http.MethodGet, "/provider/me", nil)
	w := httptest.NewRecorder()

	h.Me(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func TestProviderMe_InvalidSession(t *testing.T) {
	db := setupProviderLoginDB(t)
	h := NewProviderHandler(db, &MockMailer{})
	req := httptest.NewRequest(http.MethodGet, "/provider/me", nil)
	req.AddCookie(&http.Cookie{Name: "provider_session_token", Value: "invalid-token-not-in-db"})
	w := httptest.NewRecorder()

	h.Me(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

// expires_at > now() を使うため PostgreSQL の testDB() を使用する。
func TestProviderMe_ValidSession(t *testing.T) {
	db := testDB()
	if err := db.Ping(); err != nil {
		t.Skipf("test DB not available: %v", err)
	}

	// test_seed.sql でシードされた固定プロバイダーを使用
	const providerID = "00000000-0000-0000-0000-000000000002"

	plainToken, err := generateSessionToken()
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	tokenHash := hashSessionToken(plainToken)
	_, err = db.Exec(
		`INSERT INTO provider_sessions (provider_id, token_hash, expires_at) VALUES ($1, $2, now() + interval '24 hours')`,
		providerID, tokenHash,
	)
	if err != nil {
		t.Fatalf("failed to insert session: %v", err)
	}
	t.Cleanup(func() {
		db.Exec(`DELETE FROM provider_sessions WHERE token_hash = $1`, tokenHash)
	})

	h := NewProviderHandler(db, &MockMailer{})
	req := httptest.NewRequest(http.MethodGet, "/provider/me", nil)
	req.AddCookie(&http.Cookie{Name: "provider_session_token", Value: plainToken})
	w := httptest.NewRecorder()

	h.Me(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if resp["id"] != providerID {
		t.Errorf("want id=%s, got %v", providerID, resp["id"])
	}
	if resp["email"] != "test@example.com" {
		t.Errorf("want email=test@example.com, got %v", resp["email"])
	}
}

func TestProviderLogout_WithCookie(t *testing.T) {
	db := setupProviderLoginDB(t)
	h := NewProviderHandler(db, &MockMailer{})

	plainToken, err := generateSessionToken()
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	tokenHash := hashSessionToken(plainToken)
	db.Exec(
		`INSERT INTO provider_sessions (provider_id, token_hash, expires_at) VALUES ('prov-logout', $1, '2099-01-01')`,
		tokenHash,
	)

	req := httptest.NewRequest(http.MethodPost, "/provider/logout", nil)
	req.AddCookie(&http.Cookie{Name: "provider_session_token", Value: plainToken})
	w := httptest.NewRecorder()

	h.Logout(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}
	var count int
	db.QueryRow(`SELECT COUNT(*) FROM provider_sessions WHERE token_hash = $1`, tokenHash).Scan(&count)
	if count != 0 {
		t.Error("session should be deleted from DB after logout")
	}
}

func TestProviderLogin_InvalidJSON(t *testing.T) {
	h := NewProviderHandler(nil, &MockMailer{})
	req := httptest.NewRequest(http.MethodPost, "/provider/login", bytes.NewReader([]byte("not-json")))
	w := httptest.NewRecorder()

	h.Login(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestProviderLogin_RateLimit(t *testing.T) {
	db := setupProviderLoginDB(t)
	h := NewProviderHandler(db, &MockMailer{})

	const testIP = "10.7.0.1"
	send := func() int {
		body, _ := json.Marshal(map[string]string{"email": "nobody@example.com", "password": "wrongpass"})
		req := httptest.NewRequest(http.MethodPost, "/provider/login", bytes.NewReader(body))
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
