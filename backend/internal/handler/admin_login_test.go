package handler_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"golang.org/x/crypto/bcrypt"

	"api-billing-platform/internal/handler"
	"api-billing-platform/internal/repository"
	"api-billing-platform/internal/service"
)

type mockAdminRepo struct {
	findByEmailFunc       func(email string) (*repository.Admin, error)
	createSessionFunc     func(ctx context.Context, adminID int, tokenHash string, expiresAt time.Time) error
	findBySessionHashFunc func(ctx context.Context, tokenHash string) (*repository.Admin, error)
	deleteSessionFunc     func(ctx context.Context, tokenHash string) error
}

func (m *mockAdminRepo) FindByEmail(email string) (*repository.Admin, error) {
	if m.findByEmailFunc != nil {
		return m.findByEmailFunc(email)
	}
	return nil, sql.ErrNoRows
}

func (m *mockAdminRepo) CreateSession(ctx context.Context, adminID int, tokenHash string, expiresAt time.Time) error {
	if m.createSessionFunc != nil {
		return m.createSessionFunc(ctx, adminID, tokenHash, expiresAt)
	}
	return nil
}

func (m *mockAdminRepo) FindBySessionHash(ctx context.Context, tokenHash string) (*repository.Admin, error) {
	if m.findBySessionHashFunc != nil {
		return m.findBySessionHashFunc(ctx, tokenHash)
	}
	return nil, nil
}

func (m *mockAdminRepo) DeleteSession(ctx context.Context, tokenHash string) error {
	if m.deleteSessionFunc != nil {
		return m.deleteSessionFunc(ctx, tokenHash)
	}
	return nil
}

func newTestAdminHandler(repo repository.AdminRepositoryInterface) *handler.AdminHandler {
	svc := service.NewAdminService(repo)
	return handler.NewAdminHandler(svc)
}

func TestAdminLogin_InvalidCredentials(t *testing.T) {
	h := newTestAdminHandler(&mockAdminRepo{})

	body, _ := json.Marshal(map[string]string{"email": "nobody@example.com", "password": "pass"})
	req := httptest.NewRequest(http.MethodPost, "/admin/login", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Login(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func TestAdminLogin_WrongPassword(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("correct-pass"), bcrypt.MinCost)
	h := newTestAdminHandler(&mockAdminRepo{
		findByEmailFunc: func(email string) (*repository.Admin, error) {
			return &repository.Admin{ID: 1, Email: email, PasswordHash: string(hash), Name: "Admin"}, nil
		},
	})

	body, _ := json.Marshal(map[string]string{"email": "admin@example.com", "password": "wrong-pass"})
	req := httptest.NewRequest(http.MethodPost, "/admin/login", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Login(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func TestAdminLogin_Success(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("correct-pass"), bcrypt.MinCost)
	h := newTestAdminHandler(&mockAdminRepo{
		findByEmailFunc: func(email string) (*repository.Admin, error) {
			return &repository.Admin{ID: 1, Email: email, PasswordHash: string(hash), Name: "Admin"}, nil
		},
	})

	body, _ := json.Marshal(map[string]string{"email": "admin@example.com", "password": "correct-pass"})
	req := httptest.NewRequest(http.MethodPost, "/admin/login", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Login(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var found bool
	for _, c := range w.Result().Cookies() {
		if c.Name == "admin_session_token" && c.Value != "" {
			found = true
		}
	}
	if !found {
		t.Error("expected admin_session_token cookie to be set")
	}
}

func TestAdminLogout_NoCookie(t *testing.T) {
	h := newTestAdminHandler(&mockAdminRepo{})
	req := httptest.NewRequest(http.MethodPost, "/admin/logout", nil)
	w := httptest.NewRecorder()

	h.Logout(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}
}

func TestAdminLogout_WithCookie(t *testing.T) {
	var deleteCalled bool
	h := newTestAdminHandler(&mockAdminRepo{
		deleteSessionFunc: func(_ context.Context, _ string) error {
			deleteCalled = true
			return nil
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/admin/logout", nil)
	req.AddCookie(&http.Cookie{Name: "admin_session_token", Value: "some-token"})
	w := httptest.NewRecorder()

	h.Logout(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}
	if !deleteCalled {
		t.Error("expected DeleteSession to be called when cookie is present")
	}
}

func TestAdminMe_NoCookie(t *testing.T) {
	h := newTestAdminHandler(&mockAdminRepo{})
	req := httptest.NewRequest(http.MethodGet, "/admin/me", nil)
	w := httptest.NewRecorder()

	h.Me(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func TestAdminMe_InvalidSession(t *testing.T) {
	h := newTestAdminHandler(&mockAdminRepo{
		findBySessionHashFunc: func(_ context.Context, _ string) (*repository.Admin, error) {
			return nil, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/admin/me", nil)
	req.AddCookie(&http.Cookie{Name: "admin_session_token", Value: "invalid-token"})
	w := httptest.NewRecorder()

	h.Me(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func testAdminDockerDB() *sql.DB {
	host := os.Getenv("DB_HOST")
	if host == "" {
		host = "db_test"
	}
	port := os.Getenv("DB_PORT")
	if port == "" {
		port = "5436"
	}
	user := os.Getenv("DB_USER")
	if user == "" {
		user = "postgres"
	}
	pass := os.Getenv("DB_PASSWORD")
	if pass == "" {
		pass = "postgres"
	}
	dbname := os.Getenv("DB_NAME")
	if dbname == "" {
		dbname = "api_billing_test"
	}
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", host, port, user, pass, dbname)
	db, _ := sql.Open("pgx", dsn)
	return db
}

func newTestAdminHandlerWithDB(repo repository.AdminRepositoryInterface, db *sql.DB) *handler.AdminHandler {
	svc := service.NewAdminService(repo)
	return handler.NewAdminHandlerWithDB(svc, db)
}

func TestAdminChangePassword_MissingFields(t *testing.T) {
	h := newTestAdminHandlerWithDB(&mockAdminRepo{}, nil)
	body := bytes.NewBufferString(`{"current_password":"","new_password":""}`)
	req := httptest.NewRequest("POST", "/admin/change-password", body)
	w := httptest.NewRecorder()
	h.ChangePassword(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestAdminChangePassword_ShortPassword(t *testing.T) {
	h := newTestAdminHandlerWithDB(&mockAdminRepo{}, nil)
	body := bytes.NewBufferString(`{"current_password":"oldpass123","new_password":"short"}`)
	req := httptest.NewRequest("POST", "/admin/change-password", body)
	w := httptest.NewRecorder()
	h.ChangePassword(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestAdminChangePassword_NoSession(t *testing.T) {
	h := newTestAdminHandlerWithDB(&mockAdminRepo{}, nil)
	body := bytes.NewBufferString(`{"current_password":"oldpass123","new_password":"newpass123"}`)
	req := httptest.NewRequest("POST", "/admin/change-password", body)
	w := httptest.NewRecorder()
	h.ChangePassword(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func TestAdminChangePassword_WrongCurrentPassword(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("correctpass123"), bcrypt.MinCost)
	h := newTestAdminHandlerWithDB(&mockAdminRepo{
		findBySessionHashFunc: func(_ context.Context, _ string) (*repository.Admin, error) {
			return &repository.Admin{ID: 1, Email: "admin@example.com", PasswordHash: string(hash), Name: "Admin"}, nil
		},
	}, nil)
	body, _ := json.Marshal(map[string]string{"current_password": "wrongpass", "new_password": "newpass123"})
	req := httptest.NewRequest("POST", "/admin/change-password", bytes.NewReader(body))
	req.AddCookie(&http.Cookie{Name: "admin_session_token", Value: "some-token"})
	w := httptest.NewRecorder()
	h.ChangePassword(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func TestAdminChangePassword_Success(t *testing.T) {
	db := testAdminDockerDB()
	if err := db.Ping(); err != nil {
		t.Skipf("test DB not available: %v", err)
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte("oldpass123"), bcrypt.MinCost)
	var adminID int
	err := db.QueryRow(
		`INSERT INTO admins (email, password_hash, name) VALUES ($1, $2, $3) RETURNING id`,
		"cp-admin-test@example.com", string(hash), "Test Admin",
	).Scan(&adminID)
	if err != nil {
		t.Fatalf("failed to insert admin: %v", err)
	}
	t.Cleanup(func() {
		db.Exec(`DELETE FROM admins WHERE id = $1`, adminID)
	})

	h := newTestAdminHandlerWithDB(&mockAdminRepo{
		findBySessionHashFunc: func(_ context.Context, _ string) (*repository.Admin, error) {
			return &repository.Admin{ID: adminID, Email: "cp-admin-test@example.com", PasswordHash: string(hash), Name: "Test Admin"}, nil
		},
	}, db)

	body, _ := json.Marshal(map[string]string{"current_password": "oldpass123", "new_password": "newpass123"})
	req := httptest.NewRequest("POST", "/admin/change-password", bytes.NewReader(body))
	req.AddCookie(&http.Cookie{Name: "admin_session_token", Value: "some-token"})
	w := httptest.NewRecorder()

	h.ChangePassword(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var newHash string
	db.QueryRow(`SELECT password_hash FROM admins WHERE id = $1`, adminID).Scan(&newHash)
	if err := bcrypt.CompareHashAndPassword([]byte(newHash), []byte("newpass123")); err != nil {
		t.Error("password_hash should have been updated to newpass123")
	}
}

func TestAdminMe_ValidSession(t *testing.T) {
	h := newTestAdminHandler(&mockAdminRepo{
		findBySessionHashFunc: func(_ context.Context, _ string) (*repository.Admin, error) {
			return &repository.Admin{ID: 1, Email: "admin@example.com", Name: "Admin"}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/admin/me", nil)
	req.AddCookie(&http.Cookie{Name: "admin_session_token", Value: "some-valid-token"})
	w := httptest.NewRecorder()

	h.Me(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if resp["email"] != "admin@example.com" {
		t.Errorf("want email=admin@example.com, got %v", resp["email"])
	}
}

func TestAdminLogin_RateLimit(t *testing.T) {
	h := newTestAdminHandler(&mockAdminRepo{})

	const testIP = "10.5.0.1"
	send := func() int {
		body, _ := json.Marshal(map[string]string{"email": "nobody@example.com", "password": "pass"})
		req := httptest.NewRequest(http.MethodPost, "/admin/login", bytes.NewReader(body))
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

// handler.hashSessionToken と同じロジックをテストから参照できるように複製している。
func adminTokenHash(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

func TestAdminLogin_InvalidJSON(t *testing.T) {
	h := newTestAdminHandler(&mockAdminRepo{})
	req := httptest.NewRequest(http.MethodPost, "/admin/login", bytes.NewReader([]byte("not-json")))
	w := httptest.NewRecorder()

	h.Login(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestAdminChangePassword_InvalidJSON(t *testing.T) {
	h := newTestAdminHandlerWithDB(&mockAdminRepo{}, nil)
	req := httptest.NewRequest(http.MethodPost, "/admin/change-password", bytes.NewReader([]byte("not-json")))
	w := httptest.NewRecorder()

	h.ChangePassword(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestAdminChangePassword_NoDB(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("oldpass123"), bcrypt.MinCost)
	h := newTestAdminHandlerWithDB(&mockAdminRepo{
		findBySessionHashFunc: func(_ context.Context, _ string) (*repository.Admin, error) {
			return &repository.Admin{ID: 1, Email: "admin@example.com", PasswordHash: string(hash), Name: "Admin"}, nil
		},
	}, nil)

	body, _ := json.Marshal(map[string]string{"current_password": "oldpass123", "new_password": "newpass123"})
	req := httptest.NewRequest("POST", "/admin/change-password", bytes.NewReader(body))
	req.AddCookie(&http.Cookie{Name: "admin_session_token", Value: "some-token"})
	w := httptest.NewRecorder()

	h.ChangePassword(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500 when db is nil, got %d", w.Code)
	}
}

func TestAdminChangePassword_OtherSessionsInvalidated(t *testing.T) {
	db := testAdminDockerDB()
	if err := db.Ping(); err != nil {
		t.Skipf("test DB not available: %v", err)
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte("oldpass123"), bcrypt.MinCost)
	var adminID int
	err := db.QueryRow(
		`INSERT INTO admins (email, password_hash, name) VALUES ($1, $2, $3) RETURNING id`,
		"cp-si-admin@example.com", string(hash), "SI Test Admin",
	).Scan(&adminID)
	if err != nil {
		t.Fatalf("failed to insert admin: %v", err)
	}
	t.Cleanup(func() {
		db.Exec(`DELETE FROM admin_sessions WHERE admin_id = $1`, adminID)
		db.Exec(`DELETE FROM admins WHERE id = $1`, adminID)
	})

	currentToken := fmt.Sprintf("current-token-%d", adminID)
	otherToken := fmt.Sprintf("other-token-%d", adminID)
	db.Exec(`INSERT INTO admin_sessions (admin_id, token_hash, expires_at) VALUES ($1, $2, now() + interval '24 hours')`, adminID, adminTokenHash(currentToken))
	db.Exec(`INSERT INTO admin_sessions (admin_id, token_hash, expires_at) VALUES ($1, $2, now() + interval '24 hours')`, adminID, adminTokenHash(otherToken))

	h := newTestAdminHandlerWithDB(&mockAdminRepo{
		findBySessionHashFunc: func(_ context.Context, tokenHash string) (*repository.Admin, error) {
			if tokenHash == adminTokenHash(currentToken) {
				return &repository.Admin{ID: adminID, Email: "cp-si-admin@example.com", PasswordHash: string(hash), Name: "SI Test Admin"}, nil
			}
			return nil, nil
		},
	}, db)

	body, _ := json.Marshal(map[string]string{"current_password": "oldpass123", "new_password": "newpass123"})
	req := httptest.NewRequest("POST", "/admin/change-password", bytes.NewReader(body))
	req.AddCookie(&http.Cookie{Name: "admin_session_token", Value: currentToken})
	w := httptest.NewRecorder()

	h.ChangePassword(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var currentCount int
	db.QueryRow(`SELECT COUNT(*) FROM admin_sessions WHERE admin_id = $1 AND token_hash = $2`, adminID, adminTokenHash(currentToken)).Scan(&currentCount)
	if currentCount != 1 {
		t.Error("current session should still be valid after password change")
	}
	var otherCount int
	db.QueryRow(`SELECT COUNT(*) FROM admin_sessions WHERE admin_id = $1 AND token_hash = $2`, adminID, adminTokenHash(otherToken)).Scan(&otherCount)
	if otherCount != 0 {
		t.Error("other sessions should be invalidated after password change")
	}
}
