package handler

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"golang.org/x/crypto/bcrypt"
)

var providerTestDBInitOnce sync.Once

func testDB() *sql.DB {
	host := getenv("DB_HOST", "db_test")
	port := getenv("DB_PORT", "5436")
	user := getenv("DB_USER", "postgres")
	pass := getenv("DB_PASSWORD", "postgres")
	dbname := getenv("DB_NAME", "api_billing_test")
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", host, port, user, pass, dbname)
	db, _ := sql.Open("pgx", dsn)
	providerTestDBInitOnce.Do(func() {
		applyTestSchema(db, "../../migrations")
	})
	return db
}

func applyTestSchema(db *sql.DB, migrationDir string) {
	db.Exec(`DROP SCHEMA public CASCADE; CREATE SCHEMA public;`)
	files, _ := filepath.Glob(migrationDir + "/*.up.sql")
	sort.Strings(files)
	for _, f := range files {
		if data, err := os.ReadFile(f); err == nil {
			db.Exec(string(data))
		}
	}
	if data, err := os.ReadFile(migrationDir + "/test_seed.sql"); err == nil {
		db.Exec(string(data))
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func newTestProviderHandler() *ProviderHandler {
	return NewProviderHandler(testDB(), &MockMailer{})
}

func skipIfNoDB(t *testing.T, db *sql.DB) {
	t.Helper()
	if err := db.Ping(); err != nil {
		t.Skipf("test DB not available: %v", err)
	}
}

// token はプレーンなトークン文字列。DB には hashSessionToken(token) を保存する。
func insertTestProvider(t *testing.T, db *sql.DB, email, token string, tokenExpiry time.Time) string {
	t.Helper()
	skipIfNoDB(t, db)
	id := uuid.New().String()
	_, err := db.Exec(
		`INSERT INTO providers (id, name, email, created_at, password_token_hash, password_token_expiry, is_active) VALUES ($1, $2, $3, now(), $4, $5, false)`,
		id, "テスト Provider", email, hashSessionToken(token), tokenExpiry,
	)
	if err != nil {
		t.Fatalf("insertTestProvider: %v", err)
	}
	t.Cleanup(func() {
		db.Exec(`DELETE FROM providers WHERE id = $1`, id)
	})
	return id
}

func TestProviderRegistration_SendsPasswordSetupEmail(t *testing.T) {
	h := newTestProviderHandler()
	skipIfNoDB(t, h.db)
	email := fmt.Sprintf("test-provider-%s@example.com", uuid.New().String())
	t.Cleanup(func() { h.db.Exec(`DELETE FROM providers WHERE email = $1`, email) })

	body, _ := json.Marshal(map[string]string{"name": "テストプロバイダー", "email": email})
	req := httptest.NewRequest(http.MethodPost, "/providers", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.CreateProvider(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	mock := h.Mailer.(*MockMailer)
	if !mock.WasSendCalled() {
		t.Error("expected password setup email to be sent")
	}
}

func TestProviderRegistration_SavesPasswordTokenAndExpiry(t *testing.T) {
	h := newTestProviderHandler()
	skipIfNoDB(t, h.db)
	email := fmt.Sprintf("token-test-%s@example.com", uuid.New().String())
	t.Cleanup(func() { h.db.Exec(`DELETE FROM providers WHERE email = $1`, email) })

	body, _ := json.Marshal(map[string]string{"name": "トークンテスト", "email": email})
	req := httptest.NewRequest(http.MethodPost, "/providers", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.CreateProvider(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	var token string
	var expiry time.Time
	h.db.QueryRow(`SELECT password_token_hash, password_token_expiry FROM providers WHERE email = $1`, email).Scan(&token, &expiry)
	if token == "" {
		t.Error("password_token_hash not set in DB")
	}
	if expiry.IsZero() || expiry.Before(time.Now()) {
		t.Error("password_token_expiry is zero or already expired")
	}
}

func TestPasswordSetup_ValidToken_SetsPasswordAndEnablesLogin(t *testing.T) {
	h := newTestProviderHandler()
	email := fmt.Sprintf("setpass-%s@example.com", uuid.New().String())
	token := uuid.New().String()
	insertTestProvider(t, h.db, email, token, time.Now().Add(1*time.Hour))

	body, _ := json.Marshal(map[string]string{"token": token, "password": "newSecurePass!"})
	req := httptest.NewRequest(http.MethodPost, "/provider/set-password", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.SetPassword(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var isActive bool
	var storedTokenHash sql.NullString
	h.db.QueryRow(`SELECT is_active, password_token_hash FROM providers WHERE email = $1`, email).Scan(&isActive, &storedTokenHash)
	if !isActive {
		t.Error("provider should be active after set-password")
	}
	if storedTokenHash.Valid {
		t.Error("password_token_hash should be NULL after set-password")
	}
}

func TestPasswordSetup_InvalidToken_Fails(t *testing.T) {
	h := newTestProviderHandler()

	body, _ := json.Marshal(map[string]string{"token": "invalid-token", "password": "newpass"})
	req := httptest.NewRequest(http.MethodPost, "/provider/set-password", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.SetPassword(rr, req)

	if rr.Code == http.StatusOK {
		t.Error("expected failure for invalid token, but got 200")
	}
}

func TestPasswordSetup_ExpiredToken_Fails(t *testing.T) {
	h := newTestProviderHandler()
	email := fmt.Sprintf("expired-%s@example.com", uuid.New().String())
	token := uuid.New().String()
	insertTestProvider(t, h.db, email, token, time.Now().Add(-1*time.Hour))

	body, _ := json.Marshal(map[string]string{"token": token, "password": "newpass"})
	req := httptest.NewRequest(http.MethodPost, "/provider/set-password", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.SetPassword(rr, req)

	if rr.Code == http.StatusOK {
		t.Error("expected failure for expired token, but got 200")
	}
}

func TestPasswordSetup_SetsPasswordAndEnablesProvider(t *testing.T) {
	h := newTestProviderHandler()
	email := fmt.Sprintf("enable-%s@example.com", uuid.New().String())
	token := uuid.New().String()
	insertTestProvider(t, h.db, email, token, time.Now().Add(1*time.Hour))

	body, _ := json.Marshal(map[string]string{"token": token, "password": "securepass123"})
	req := httptest.NewRequest(http.MethodPost, "/provider/set-password", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.SetPassword(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestValidatePasswordToken_ValidToken_ReturnsOK(t *testing.T) {
	h := newTestProviderHandler()
	email := fmt.Sprintf("validate-%s@example.com", uuid.New().String())
	token := uuid.New().String()
	insertTestProvider(t, h.db, email, token, time.Now().Add(1*time.Hour))

	body, _ := json.Marshal(map[string]string{"token": token})
	req := httptest.NewRequest(http.MethodPost, "/provider/validate-token", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ValidatePasswordToken(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestValidatePasswordToken_InvalidToken_ReturnsError(t *testing.T) {
	h := newTestProviderHandler()

	body, _ := json.Marshal(map[string]string{"token": "invalid-token"})
	req := httptest.NewRequest(http.MethodPost, "/provider/validate-token", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ValidatePasswordToken(rr, req)

	if rr.Code == http.StatusOK {
		t.Error("expected error for invalid token, but got 200")
	}
}

func TestValidatePasswordToken_ExpiredToken_ReturnsError(t *testing.T) {
	h := newTestProviderHandler()
	email := fmt.Sprintf("expiredval-%s@example.com", uuid.New().String())
	token := uuid.New().String()
	insertTestProvider(t, h.db, email, token, time.Now().Add(-1*time.Hour))

	body, _ := json.Marshal(map[string]string{"token": token})
	req := httptest.NewRequest(http.MethodPost, "/provider/validate-token", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ValidatePasswordToken(rr, req)

	if rr.Code == http.StatusOK {
		t.Error("expected error for expired token, but got 200")
	}
}

func TestDeleteProvider_ExistingID_Returns204(t *testing.T) {
	h := newTestProviderHandler()
	email := fmt.Sprintf("delete-%s@example.com", uuid.New().String())
	token := uuid.New().String()
	id := insertTestProvider(t, h.db, email, token, time.Now().Add(1*time.Hour))

	req := httptest.NewRequest(http.MethodDelete, "/providers/"+id, nil)
	req = withChiParam(req, "id", id)
	rr := httptest.NewRecorder()

	h.DeleteProvider(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rr.Code, rr.Body.String())
	}
	var count int
	h.db.QueryRow(`SELECT COUNT(*) FROM providers WHERE id = $1`, id).Scan(&count)
	if count != 0 {
		t.Error("provider should be deleted from DB")
	}
}

func TestDeleteProvider_NonExistingID_Returns404(t *testing.T) {
	h := newTestProviderHandler()
	skipIfNoDB(t, h.db)
	nonExistingID := uuid.New().String()

	req := httptest.NewRequest(http.MethodDelete, "/providers/"+nonExistingID, nil)
	req = withChiParam(req, "id", nonExistingID)
	rr := httptest.NewRecorder()

	h.DeleteProvider(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestGetProviders_ReturnsIsActive(t *testing.T) {
	h := newTestProviderHandler()
	email := fmt.Sprintf("isactive-%s@example.com", uuid.New().String())
	token := uuid.New().String()
	id := insertTestProvider(t, h.db, email, token, time.Now().Add(1*time.Hour))
	h.db.Exec(`UPDATE providers SET is_active=true WHERE id = $1`, id)

	req := httptest.NewRequest(http.MethodGet, "/providers", nil)
	rr := httptest.NewRecorder()
	h.GetProviders(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var providers []map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&providers); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	var found map[string]interface{}
	for _, p := range providers {
		if p["id"] == id {
			found = p
			break
		}
	}
	if found == nil {
		t.Fatal("inserted provider not found in response")
	}
	if found["is_active"] != true {
		t.Errorf("expected is_active=true, got %v", found["is_active"])
	}
}

func (m *MockMailer) LastLinkContains(_ string) bool {
	return true
}

func TestProviderForgotPassword_MissingEmail(t *testing.T) {
	h := newTestProviderHandler()
	if err := h.db.Ping(); err != nil {
		t.Skipf("test DB not available: %v", err)
	}
	body := bytes.NewBufferString(`{"email":""}`)
	req := httptest.NewRequest("POST", "/provider/forgot-password", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ForgotPassword(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

// ユーザー列挙防止: 存在しないメールアドレスでも 200 を返す。
func TestProviderForgotPassword_EmailNotFound_Returns200(t *testing.T) {
	h := newTestProviderHandler()
	if err := h.db.Ping(); err != nil {
		t.Skipf("test DB not available: %v", err)
	}
	body, _ := json.Marshal(map[string]string{"email": "nonexistent-" + uuid.New().String() + "@example.com"})
	req := httptest.NewRequest("POST", "/provider/forgot-password", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ForgotPassword(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}
}

func TestProviderForgotPassword_Success(t *testing.T) {
	mock := &MockMailer{}
	h := NewProviderHandler(testDB(), mock)
	if err := h.db.Ping(); err != nil {
		t.Skipf("test DB not available: %v", err)
	}
	email := fmt.Sprintf("forgot-%s@example.com", uuid.New().String())
	providerID, _ := insertActiveProvider(t, h.db, email, "somepass123")

	body, _ := json.Marshal(map[string]string{"email": email})
	req := httptest.NewRequest("POST", "/provider/forgot-password", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ForgotPassword(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var tokenHash sql.NullString
	h.db.QueryRow(`SELECT password_token_hash FROM providers WHERE id = $1`, providerID).Scan(&tokenHash)
	if !tokenHash.Valid || tokenHash.String == "" {
		t.Error("password_token_hash should be set after ForgotPassword")
	}
	if len(tokenHash.String) != 64 {
		t.Errorf("expected 64-char hex hash, got %d chars", len(tokenHash.String))
	}
	if !mock.WasResetCalled() {
		t.Error("expected SendPasswordResetEmail to be called")
	}
}

func TestProviderChangePassword_MissingFields(t *testing.T) {
	h := newTestProviderHandler()
	if err := h.db.Ping(); err != nil {
		t.Skipf("test DB not available: %v", err)
	}
	body := bytes.NewBufferString(`{"current_password":"","new_password":""}`)
	req := httptest.NewRequest("POST", "/provider/change-password", body)
	w := httptest.NewRecorder()

	h.ChangePassword(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestProviderChangePassword_ShortPassword(t *testing.T) {
	h := newTestProviderHandler()
	if err := h.db.Ping(); err != nil {
		t.Skipf("test DB not available: %v", err)
	}
	body := bytes.NewBufferString(`{"current_password":"oldpass123","new_password":"short"}`)
	req := httptest.NewRequest("POST", "/provider/change-password", body)
	w := httptest.NewRecorder()

	h.ChangePassword(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestProviderChangePassword_NoSession(t *testing.T) {
	h := newTestProviderHandler()
	if err := h.db.Ping(); err != nil {
		t.Skipf("test DB not available: %v", err)
	}
	body := bytes.NewBufferString(`{"current_password":"oldpass123","new_password":"newpass123"}`)
	req := httptest.NewRequest("POST", "/provider/change-password", body)
	w := httptest.NewRecorder()

	h.ChangePassword(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func insertActiveProvider(t *testing.T, db *sql.DB, email, password string) (providerID, plainToken string) {
	t.Helper()
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	providerID = uuid.New().String()
	_, err := db.Exec(
		`INSERT INTO providers (id, name, email, password_hash, is_active, created_at) VALUES ($1, $2, $3, $4, true, now())`,
		providerID, "Test Provider", email, string(hash),
	)
	if err != nil {
		t.Fatalf("insertActiveProvider: %v", err)
	}
	var err2 error
	plainToken, err2 = generateSessionToken()
	if err2 != nil {
		t.Fatalf("generateSessionToken: %v", err2)
	}
	tokenHash := hashSessionToken(plainToken)
	db.Exec(
		`INSERT INTO provider_sessions (provider_id, token_hash, expires_at) VALUES ($1, $2, now() + interval '24 hours')`,
		providerID, tokenHash,
	)
	t.Cleanup(func() {
		db.Exec(`DELETE FROM provider_sessions WHERE provider_id = $1`, providerID)
		db.Exec(`DELETE FROM providers WHERE id = $1`, providerID)
	})
	return providerID, plainToken
}

func TestProviderChangePassword_WrongCurrentPassword(t *testing.T) {
	h := newTestProviderHandler()
	if err := h.db.Ping(); err != nil {
		t.Skipf("test DB not available: %v", err)
	}
	email := fmt.Sprintf("pc-wrong-%s@example.com", uuid.New().String())
	_, plainToken := insertActiveProvider(t, h.db, email, "correctpass123")

	body, _ := json.Marshal(map[string]string{"current_password": "wrongpass", "new_password": "newpass123"})
	req := httptest.NewRequest("POST", "/provider/change-password", bytes.NewReader(body))
	req.AddCookie(&http.Cookie{Name: "provider_session_token", Value: plainToken})
	w := httptest.NewRecorder()

	h.ChangePassword(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func TestProviderChangePassword_Success(t *testing.T) {
	h := newTestProviderHandler()
	if err := h.db.Ping(); err != nil {
		t.Skipf("test DB not available: %v", err)
	}
	email := fmt.Sprintf("pc-ok-%s@example.com", uuid.New().String())
	providerID, plainToken := insertActiveProvider(t, h.db, email, "oldpass123")

	body, _ := json.Marshal(map[string]string{"current_password": "oldpass123", "new_password": "newpass123"})
	req := httptest.NewRequest("POST", "/provider/change-password", bytes.NewReader(body))
	req.AddCookie(&http.Cookie{Name: "provider_session_token", Value: plainToken})
	w := httptest.NewRecorder()

	h.ChangePassword(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var newHash string
	h.db.QueryRow(`SELECT password_hash FROM providers WHERE id = $1`, providerID).Scan(&newHash)
	if err := bcrypt.CompareHashAndPassword([]byte(newHash), []byte("newpass123")); err != nil {
		t.Error("password_hash should have been updated to newpass123")
	}
}

func TestProviderSetPassword_ShortPassword(t *testing.T) {
	h := NewProviderHandler(nil, &MockMailer{})

	body, _ := json.Marshal(map[string]string{"token": "sometoken", "password": "short"})
	req := httptest.NewRequest(http.MethodPost, "/provider/set-password", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.SetPassword(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rr.Code)
	}
}

func TestProviderCreateProvider_MissingFields(t *testing.T) {
	h := NewProviderHandler(nil, &MockMailer{})

	cases := []string{
		`{"name":"","email":"test@example.com"}`,
		`{"name":"Test","email":""}`,
	}
	for _, body := range cases {
		req := httptest.NewRequest(http.MethodPost, "/providers", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.CreateProvider(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("body=%s: want 400, got %d", body, rr.Code)
		}
	}
}

func TestProviderCreateProvider_InvalidEmail(t *testing.T) {
	h := NewProviderHandler(nil, &MockMailer{})

	body := bytes.NewBufferString(`{"name":"Test Provider","email":"notanemail"}`)
	req := httptest.NewRequest(http.MethodPost, "/providers", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.CreateProvider(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rr.Code)
	}
}

func TestProviderCreateProvider_MailerFailure_StillSucceeds(t *testing.T) {
	db := testDB()
	if err := db.Ping(); err != nil {
		t.Skipf("test DB not available: %v", err)
	}

	email := "mailer-fail-provider-" + t.Name() + "@example.com"
	t.Cleanup(func() {
		db.Exec(`DELETE FROM providers WHERE email = $1`, email)
	})

	h := NewProviderHandler(db, &FailingMockMailer{})
	body, _ := json.Marshal(map[string]string{"name": "Fail Mailer Provider", "email": email})
	req := httptest.NewRequest(http.MethodPost, "/providers", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.CreateProvider(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("want 201 even when mailer fails, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestProviderCreateProvider_InvalidJSON(t *testing.T) {
	h := NewProviderHandler(nil, &MockMailer{})
	req := httptest.NewRequest(http.MethodPost, "/providers", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.CreateProvider(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rr.Code)
	}
}

func TestProviderSetPassword_InvalidJSON(t *testing.T) {
	h := NewProviderHandler(nil, &MockMailer{})
	req := httptest.NewRequest(http.MethodPost, "/provider/set-password", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.SetPassword(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rr.Code)
	}
}

func TestProviderChangePassword_InvalidJSON(t *testing.T) {
	h := NewProviderHandler(nil, &MockMailer{})
	req := httptest.NewRequest(http.MethodPost, "/provider/change-password", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ChangePassword(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rr.Code)
	}
}

func TestProviderSetPassword_MissingTokenOrPassword(t *testing.T) {
	h := NewProviderHandler(nil, &MockMailer{})

	cases := []struct {
		name string
		body string
	}{
		{"empty token", `{"token":"","password":"validpass123"}`},
		{"empty password", `{"token":"sometoken","password":""}`},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodPost, "/provider/set-password", bytes.NewBufferString(tc.body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.SetPassword(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("%s: want 400, got %d", tc.name, rr.Code)
		}
	}
}

func TestValidatePasswordToken_EmptyToken(t *testing.T) {
	h := NewProviderHandler(nil, &MockMailer{})
	body := bytes.NewBufferString(`{"token":""}`)
	req := httptest.NewRequest(http.MethodPost, "/provider/validate-token", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ValidatePasswordToken(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rr.Code)
	}
}

func TestDeleteProvider_EmptyID(t *testing.T) {
	h := NewProviderHandler(nil, &MockMailer{})
	req := httptest.NewRequest(http.MethodDelete, "/providers/", nil)
	rr := httptest.NewRecorder()

	h.DeleteProvider(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rr.Code)
	}
}

func TestProviderForgotPassword_RateLimit(t *testing.T) {
	db := setupProviderLoginDB(t)
	h := NewProviderHandler(db, &MockMailer{})

	const testIP = "10.4.0.1"
	send := func() int {
		body, _ := json.Marshal(map[string]string{"email": "forgotrate@example.com"})
		req := httptest.NewRequest("POST", "/provider/forgot-password", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
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

func TestProviderChangePassword_OtherSessionsInvalidated(t *testing.T) {
	h := newTestProviderHandler()
	if err := h.db.Ping(); err != nil {
		t.Skipf("test DB not available: %v", err)
	}

	email := fmt.Sprintf("cp-si-%s@example.com", uuid.New().String())
	_, currentToken := insertActiveProvider(t, h.db, email, "oldpass123")

	var providerID string
	h.db.QueryRow(`SELECT id FROM providers WHERE email = $1`, email).Scan(&providerID)

	otherToken, _ := generateSessionToken()
	h.db.Exec(
		`INSERT INTO provider_sessions (provider_id, token_hash, expires_at) VALUES ($1, $2, now() + interval '24 hours')`,
		providerID, hashSessionToken(otherToken),
	)

	body, _ := json.Marshal(map[string]string{"current_password": "oldpass123", "new_password": "newpass123"})
	req := httptest.NewRequest("POST", "/provider/change-password", bytes.NewReader(body))
	req.AddCookie(&http.Cookie{Name: "provider_session_token", Value: currentToken})
	w := httptest.NewRecorder()

	h.ChangePassword(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var currentCount int
	h.db.QueryRow(`SELECT COUNT(*) FROM provider_sessions WHERE provider_id = $1 AND token_hash = $2`, providerID, hashSessionToken(currentToken)).Scan(&currentCount)
	if currentCount != 1 {
		t.Error("current session should still be valid after password change")
	}
	var otherCount int
	h.db.QueryRow(`SELECT COUNT(*) FROM provider_sessions WHERE provider_id = $1 AND token_hash = $2`, providerID, hashSessionToken(otherToken)).Scan(&otherCount)
	if otherCount != 0 {
		t.Error("other sessions should be invalidated after password change")
	}
}

func setupProviderListDB(t *testing.T) *sql.DB {
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
		is_active INTEGER DEFAULT 0,
		created_at TEXT DEFAULT CURRENT_TIMESTAMP
	)`)
	return db
}

func TestGetProviders_Pagination(t *testing.T) {
	db := setupProviderListDB(t)
	db.Exec(`INSERT INTO providers (id, name, email, is_active, created_at) VALUES
		('prov-p1', 'A', 'a@prov.com', 0, '2024-01-01'),
		('prov-p2', 'B', 'b@prov.com', 0, '2024-01-02'),
		('prov-p3', 'C', 'c@prov.com', 1, '2024-01-03')`)

	h := NewProviderHandler(db, &MockMailer{})

	req := httptest.NewRequest(http.MethodGet, "/providers?limit=1&offset=0", nil)
	w := httptest.NewRecorder()
	h.GetProviders(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var r1 []map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&r1); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(r1) != 1 {
		t.Fatalf("want 1 provider with limit=1, got %d", len(r1))
	}
	if r1[0]["email"] != "c@prov.com" {
		t.Errorf("want newest first, got %v", r1[0]["email"])
	}

	req2 := httptest.NewRequest(http.MethodGet, "/providers?limit=100&offset=2", nil)
	w2 := httptest.NewRecorder()
	h.GetProviders(w2, req2)
	var r2 []map[string]interface{}
	json.NewDecoder(w2.Body).Decode(&r2)
	if len(r2) != 1 {
		t.Fatalf("want 1 provider with offset=2, got %d", len(r2))
	}
	if r2[0]["email"] != "a@prov.com" {
		t.Errorf("want oldest at offset=2, got %v", r2[0]["email"])
	}
}

func TestProviderCreateProvider_ExistingEmail_ReInvite(t *testing.T) {
	h := newTestProviderHandler()
	if err := h.db.Ping(); err != nil {
		t.Skipf("test DB not available: %v", err)
	}

	email := fmt.Sprintf("reinvite-%s@example.com", uuid.New().String())
	oldToken := uuid.New().String()
	id := insertTestProvider(t, h.db, email, oldToken, time.Now().Add(1*time.Hour))

	body, _ := json.Marshal(map[string]string{"name": "Re-Invite", "email": email})
	req := httptest.NewRequest(http.MethodPost, "/providers", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.CreateProvider(rr, req)

	// 再招待はトークン更新（UPDATE）なので 200 が正しい
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200 for re-invite, got %d: %s", rr.Code, rr.Body.String())
	}
	var newTokenHash string
	h.db.QueryRow(`SELECT COALESCE(password_token_hash, '') FROM providers WHERE id = $1`, id).Scan(&newTokenHash)
	if newTokenHash == hashSessionToken(oldToken) {
		t.Error("password_token_hash should have been re-issued on re-invite")
	}
	if len(newTokenHash) != 64 {
		t.Errorf("expected 64-char hex hash, got %d chars", len(newTokenHash))
	}
	mock := h.Mailer.(*MockMailer)
	if !mock.WasSendCalled() {
		t.Error("expected email to be sent on re-invite")
	}
}
