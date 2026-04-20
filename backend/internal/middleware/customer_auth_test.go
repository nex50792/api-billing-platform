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

type fakeCustomerSessionStore struct{}

func (f *fakeCustomerSessionStore) GetCustomerByTokenHash(_ context.Context, tokenHash string) (string, time.Time, error) {
	h := sha256.Sum256([]byte("valid-customer-token"))
	if tokenHash == hex.EncodeToString(h[:]) {
		return "customer-uuid-1", time.Now().Add(time.Hour), nil
	}
	return "", time.Time{}, sql.ErrNoRows
}

func TestCustomerAuthMiddleware_NoCookie(t *testing.T) {
	mw := middleware.NewCustomerAuthMiddleware(&fakeCustomerSessionStore{})
	r := chi.NewRouter()
	r.With(mw.Require()).Post("/apikeys", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/apikeys", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("cookie なし: 401 期待, got %d", w.Code)
	}
}

func TestCustomerAuthMiddleware_InvalidToken(t *testing.T) {
	mw := middleware.NewCustomerAuthMiddleware(&fakeCustomerSessionStore{})
	r := chi.NewRouter()
	r.With(mw.Require()).Post("/apikeys", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/apikeys", nil)
	req.AddCookie(&http.Cookie{Name: "customer_session_token", Value: "bad-token"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("無効トークン: 401 期待, got %d", w.Code)
	}
}

func TestCustomerAuthMiddleware_ExpiredToken(t *testing.T) {
	store := &expiredCustomerSessionStore{}
	mw := middleware.NewCustomerAuthMiddleware(store)
	r := chi.NewRouter()
	r.With(mw.Require()).Post("/apikeys", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/apikeys", nil)
	req.AddCookie(&http.Cookie{Name: "customer_session_token", Value: "expired-token"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("期限切れトークン: 401 期待, got %d", w.Code)
	}
}

type expiredCustomerSessionStore struct{}

func (f *expiredCustomerSessionStore) GetCustomerByTokenHash(_ context.Context, _ string) (string, time.Time, error) {
	return "customer-uuid-1", time.Now().Add(-time.Hour), nil
}

func TestCustomerAuthMiddleware_ValidToken(t *testing.T) {
	mw := middleware.NewCustomerAuthMiddleware(&fakeCustomerSessionStore{})
	r := chi.NewRouter()
	r.With(mw.Require()).Post("/apikeys", func(w http.ResponseWriter, r *http.Request) {
		customerID, ok := r.Context().Value(middleware.ContextKeyCustomerID).(string)
		if !ok || customerID == "" {
			http.Error(w, "customer id missing in context", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/apikeys", nil)
	req.AddCookie(&http.Cookie{Name: "customer_session_token", Value: "valid-customer-token"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("有効トークン: 200 期待, got %d", w.Code)
	}
}
