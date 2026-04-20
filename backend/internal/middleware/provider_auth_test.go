package middleware_test

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"api-billing-platform/internal/middleware"

	"github.com/go-chi/chi/v5"
)

type fakeProviderSessionStore struct{}

func (f *fakeProviderSessionStore) GetProviderByTokenHash(_ context.Context, tokenHash string) (string, time.Time, error) {
	h := sha256.Sum256([]byte("valid-provider-token"))
	if tokenHash == hex.EncodeToString(h[:]) {
		return "provider-uuid-1", time.Now().Add(time.Hour), nil
	}
	return "", time.Time{}, sql.ErrNoRows
}

func TestProviderAuthMiddleware_NoCookie(t *testing.T) {
	mw := middleware.NewProviderAuthMiddleware(&fakeProviderSessionStore{})
	r := chi.NewRouter()
	r.With(mw.Require()).Post("/provided-apis", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/provided-apis", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("cookie なし: 401 期待, got %d", w.Code)
	}
}

func TestProviderAuthMiddleware_InvalidToken(t *testing.T) {
	mw := middleware.NewProviderAuthMiddleware(&fakeProviderSessionStore{})
	r := chi.NewRouter()
	r.With(mw.Require()).Post("/provided-apis", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/provided-apis", nil)
	req.AddCookie(&http.Cookie{Name: "provider_session_token", Value: "bad-token"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("無効トークン: 401 期待, got %d", w.Code)
	}
}

func TestProviderAuthMiddleware_ValidToken(t *testing.T) {
	mw := middleware.NewProviderAuthMiddleware(&fakeProviderSessionStore{})
	r := chi.NewRouter()
	r.With(mw.Require()).Post("/provided-apis", func(w http.ResponseWriter, r *http.Request) {
		providerID, ok := r.Context().Value(middleware.ContextKeyProviderID).(string)
		if !ok || providerID == "" {
			http.Error(w, "provider id missing in context", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/provided-apis", nil)
	req.AddCookie(&http.Cookie{Name: "provider_session_token", Value: "valid-provider-token"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("有効トークン: 200 期待, got %d", w.Code)
	}
}

func TestProviderAuthMiddleware_ExpiredToken(t *testing.T) {
	store := &expiredProviderSessionStore{}
	mw := middleware.NewProviderAuthMiddleware(store)
	r := chi.NewRouter()
	r.With(mw.Require()).Post("/provided-apis", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/provided-apis", nil)
	req.AddCookie(&http.Cookie{Name: "provider_session_token", Value: "expired-token"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("期限切れトークン: 401 期待, got %d", w.Code)
	}
}

type expiredProviderSessionStore struct{}

func (f *expiredProviderSessionStore) GetProviderByTokenHash(_ context.Context, _ string) (string, time.Time, error) {
	return "provider-uuid-1", time.Now().Add(-time.Hour), nil
}
