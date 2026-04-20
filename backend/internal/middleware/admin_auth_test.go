package middleware_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"api-billing-platform/internal/middleware"
	"api-billing-platform/internal/repository"
	"api-billing-platform/internal/service"

	"github.com/go-chi/chi/v5"
)

type fakeAdminRepo struct{}

func (f *fakeAdminRepo) FindByEmail(email string) (*repository.Admin, error) { return nil, nil }

func (f *fakeAdminRepo) CreateSession(_ context.Context, _ int, _ string, _ time.Time) error {
	return nil
}

func (f *fakeAdminRepo) FindBySessionHash(_ context.Context, tokenHash string) (*repository.Admin, error) {
	h := sha256.Sum256([]byte("valid-admin-token"))
	if tokenHash == hex.EncodeToString(h[:]) {
		return &repository.Admin{ID: 1, Email: "admin@example.com", Name: "Admin"}, nil
	}
	return nil, nil
}

func (f *fakeAdminRepo) DeleteSession(_ context.Context, _ string) error { return nil }

func TestAdminAuthMiddleware_NoCookie(t *testing.T) {
	mw := middleware.NewAdminAuthMiddleware(service.NewAdminService(&fakeAdminRepo{}))
	r := chi.NewRouter()
	r.With(mw.Require()).Get("/providers", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/providers", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("cookie なし: 401 期待, got %d", w.Code)
	}
}

func TestAdminAuthMiddleware_InvalidToken(t *testing.T) {
	mw := middleware.NewAdminAuthMiddleware(service.NewAdminService(&fakeAdminRepo{}))
	r := chi.NewRouter()
	r.With(mw.Require()).Get("/providers", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/providers", nil)
	req.AddCookie(&http.Cookie{Name: "admin_session_token", Value: "bad-token"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("無効トークン: 401 期待, got %d", w.Code)
	}
}

func TestAdminAuthMiddleware_ValidToken(t *testing.T) {
	mw := middleware.NewAdminAuthMiddleware(service.NewAdminService(&fakeAdminRepo{}))
	r := chi.NewRouter()
	r.With(mw.Require()).Get("/providers", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/providers", nil)
	req.AddCookie(&http.Cookie{Name: "admin_session_token", Value: "valid-admin-token"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("有効トークン: 200 期待, got %d", w.Code)
	}
}

type expiredAdminRepo struct{}

func (f *expiredAdminRepo) FindByEmail(_ string) (*repository.Admin, error) { return nil, nil }
func (f *expiredAdminRepo) CreateSession(_ context.Context, _ int, _ string, _ time.Time) error {
	return nil
}
func (f *expiredAdminRepo) FindBySessionHash(_ context.Context, _ string) (*repository.Admin, error) {
	return nil, nil // 期限切れ → DBの expires_at > now() 条件にヒットしない場合と同等
}
func (f *expiredAdminRepo) DeleteSession(_ context.Context, _ string) error { return nil }

func TestAdminAuthMiddleware_ExpiredSession(t *testing.T) {
	mw := middleware.NewAdminAuthMiddleware(service.NewAdminService(&expiredAdminRepo{}))
	r := chi.NewRouter()
	r.With(mw.Require()).Get("/providers", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/providers", nil)
	req.AddCookie(&http.Cookie{Name: "admin_session_token", Value: "expired-admin-token"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("期限切れセッション: 401 期待, got %d", w.Code)
	}
}
