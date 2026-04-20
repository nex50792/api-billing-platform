package middleware_test

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"api-billing-platform/internal/middleware"
	"api-billing-platform/internal/repository"

	_ "github.com/glebarez/sqlite"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func setupAPIKeyAuthSQLite(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	db.Exec(`CREATE TABLE api_keys (
		id TEXT PRIMARY KEY,
		customer_id TEXT,
		provided_api_id TEXT,
		key_hash TEXT NOT NULL UNIQUE,
		usage_count INTEGER DEFAULT 0,
		usage_reset_at TEXT,
		created_at TEXT
	)`)
	return db
}

func apiKeyAuthPgDB(t *testing.T) *sql.DB {
	t.Helper()
	getenv := func(k, d string) string {
		if v := os.Getenv(k); v != "" {
			return v
		}
		return d
	}
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		getenv("DB_HOST", "localhost"),
		getenv("DB_PORT", "5436"),
		getenv("DB_USER", "postgres"),
		getenv("DB_PASSWORD", "postgres"),
		getenv("DB_NAME", "api_billing_test"),
	)
	db, _ := sql.Open("pgx", dsn)
	if err := db.Ping(); err != nil {
		t.Skipf("test DB not available: %v", err)
	}
	return db
}

func TestAPIKeyAuthMiddleware_MissingHeader(t *testing.T) {
	mw := middleware.NewAPIKeyAuthMiddleware(nil)
	req := httptest.NewRequest(http.MethodPost, "/billing/check", nil)
	w := httptest.NewRecorder()

	mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func TestAPIKeyAuthMiddleware_InvalidBearerPrefix(t *testing.T) {
	mw := middleware.NewAPIKeyAuthMiddleware(nil)
	req := httptest.NewRequest(http.MethodPost, "/billing/check", nil)
	req.Header.Set("Authorization", "Token some-key")
	w := httptest.NewRecorder()

	mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func TestAPIKeyAuthMiddleware_UnknownToken(t *testing.T) {
	db := setupAPIKeyAuthSQLite(t)
	repo := repository.NewAPIKeyRepository(db)
	mw := middleware.NewAPIKeyAuthMiddleware(repo)

	req := httptest.NewRequest(http.MethodPost, "/billing/check", nil)
	req.Header.Set("Authorization", "Bearer unknown-key-xyz")
	w := httptest.NewRecorder()

	mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func TestAPIKeyAuthMiddleware_ValidToken(t *testing.T) {
	db := apiKeyAuthPgDB(t)
	defer db.Close()

	keyRepo := repository.NewAPIKeyRepository(db)

	var customerID string
	err := db.QueryRow(
		`INSERT INTO customers (id, name, email, password_hash) VALUES (gen_random_uuid(), 'APIKeyAuth Test', $1, 'hash') RETURNING id`,
		"apikey-auth-mw-test@example.com",
	).Scan(&customerID)
	if err != nil {
		t.Fatalf("failed to insert customer: %v", err)
	}
	t.Cleanup(func() { db.Exec(`DELETE FROM customers WHERE id = $1`, customerID) })

	plainKey := "ak_test_mw_valid_" + t.Name()
	apiKey, err := keyRepo.Create(context.Background(), customerID, plainKey, nil)
	if err != nil {
		t.Fatalf("failed to create api key: %v", err)
	}
	t.Cleanup(func() { db.Exec(`DELETE FROM api_keys WHERE id = $1`, apiKey.ID) })

	mw := middleware.NewAPIKeyAuthMiddleware(keyRepo)

	var gotContextKey interface{}
	checkHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContextKey = r.Context().Value(middleware.ContextKeyAPIKey)
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/billing/check", nil)
	req.Header.Set("Authorization", "Bearer "+plainKey)
	w := httptest.NewRecorder()

	mw.Handler(checkHandler).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	if gotContextKey == nil {
		t.Error("expected api key to be set in context")
	}
}
