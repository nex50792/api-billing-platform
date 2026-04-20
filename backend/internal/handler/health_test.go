package handler_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"api-billing-platform/internal/handler"
)

var ErrDBDown = errors.New("db connection failed")

type mockDB struct {
	pingErr error
}

func (m *mockDB) Ping() error {
	return m.pingErr
}

func TestHealthHandler_Success(t *testing.T) {
	mockDB := &mockDB{pingErr: nil}
	h := handler.HealthHandler(mockDB)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	h(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
	if body := w.Body.String(); body != "ok" {
		t.Errorf("expected body 'ok', got '%s'", body)
	}
}

func TestHealthHandler_DBError(t *testing.T) {
	mockDB := &mockDB{pingErr: ErrDBDown}
	h := handler.HealthHandler(mockDB)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	h(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", resp.StatusCode)
	}
	if body := w.Body.String(); body != "db NG" {
		t.Errorf("expected body 'db NG', got '%s'", body)
	}
}
