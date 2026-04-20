package handler_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"api-billing-platform/internal/handler"
	"api-billing-platform/internal/middleware"
	"api-billing-platform/internal/model"
	"api-billing-platform/internal/repository"

	_ "github.com/glebarez/sqlite"
	"github.com/go-chi/chi/v5"
)

func withChiID(r *http.Request, id string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

type mockAPIKeyService struct {
	issueAPIKeyFunc  func(ctx context.Context, customerID string, providedAPIID *string) (string, *model.APIKey, error)
	verifyAPIKeyFunc func(ctx context.Context, plainKey, providedAPIID, providerID string) (*repository.VerifyResult, error)
	deleteAPIKeyFunc func(ctx context.Context, id, customerID string) error
}

func (m *mockAPIKeyService) IssueAPIKey(ctx context.Context, customerID string, providedAPIID *string) (string, *model.APIKey, error) {
	if m.issueAPIKeyFunc != nil {
		return m.issueAPIKeyFunc(ctx, customerID, providedAPIID)
	}
	return "plain-key-abc123", &model.APIKey{ID: "key-123", CustomerID: customerID}, nil
}

func (m *mockAPIKeyService) VerifyAPIKey(ctx context.Context, plainKey, providedAPIID, providerID string) (*repository.VerifyResult, error) {
	if m.verifyAPIKeyFunc != nil {
		return m.verifyAPIKeyFunc(ctx, plainKey, providedAPIID, providerID)
	}
	return nil, nil
}

func (m *mockAPIKeyService) DeleteAPIKey(ctx context.Context, id, customerID string) error {
	if m.deleteAPIKeyFunc != nil {
		return m.deleteAPIKeyFunc(ctx, id, customerID)
	}
	return nil
}

func TestAPIKeyHandler_IssueAPIKey_InvalidJSON(t *testing.T) {
	h := handler.NewAPIKeyHandler(nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/apikeys", bytes.NewReader([]byte("invalid")))
	w := httptest.NewRecorder()

	h.IssueAPIKey(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func TestAPIKeyHandler_IssueAPIKey_NoCustomerID_Returns401(t *testing.T) {
	h := handler.NewAPIKeyHandler(nil, nil)
	body := []byte(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/apikeys", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.IssueAPIKey(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func TestAPIKeyHandler_IssueAPIKey_Success(t *testing.T) {
	mock := &mockAPIKeyService{
		issueAPIKeyFunc: func(ctx context.Context, customerID string, providedAPIID *string) (string, *model.APIKey, error) {
			return "plain-key-123456", &model.APIKey{ID: "key-new", CustomerID: customerID, UsageCount: 0}, nil
		},
	}
	h := handler.NewAPIKeyHandler(mock, nil)

	body, _ := json.Marshal(map[string]interface{}{})
	req := httptest.NewRequest(http.MethodPost, "/apikeys", bytes.NewReader(body))
	ctx := context.WithValue(req.Context(), middleware.ContextKeyCustomerID, "customer-xyz")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	h.IssueAPIKey(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}

	var respData map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&respData)
	if respData["api_key"] == nil {
		t.Error("api_key should be in response")
	}
}

func setupCustomerAPIKeysTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	db.Exec(`CREATE TABLE customers (id TEXT PRIMARY KEY, name TEXT, email TEXT, plan TEXT DEFAULT 'free', stripe_customer_id TEXT, subscription_status TEXT, created_at TEXT)`)
	db.Exec(`CREATE TABLE api_keys (id TEXT PRIMARY KEY, customer_id TEXT, key_hash TEXT NOT NULL, usage_count INTEGER DEFAULT 0, usage_reset_at TEXT, created_at TEXT, provided_api_id TEXT)`)
	return db
}

func TestGetCustomerAPIKeys_ReturnsOwnedKeys(t *testing.T) {
	db := setupCustomerAPIKeysTestDB(t)
	const (
		custA = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		custB = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		key1  = "11111111-1111-1111-1111-111111111111"
		key2  = "22222222-2222-2222-2222-222222222222"
	)
	db.Exec(`INSERT INTO customers (id, name, email) VALUES (?, 'Customer A', 'a@example.com'), (?, 'Customer B', 'b@example.com')`, custA, custB)
	db.Exec(`INSERT INTO api_keys (id, customer_id, key_hash, usage_count, usage_reset_at, created_at) VALUES (?, ?, 'hash1', 5, '2024-01-01T00:00:00Z', '2024-01-01T00:00:00Z'), (?, ?, 'hash2', 0, '2024-01-01T00:00:00Z', '2024-01-01T00:00:00Z')`, key1, custA, key2, custB)

	h := handler.NewAPIKeyHandler(nil, db)
	req := httptest.NewRequest(http.MethodGet, "/customer/apikeys", nil)
	ctx := context.WithValue(req.Context(), middleware.ContextKeyCustomerID, custA)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	h.GetCustomerAPIKeys(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var keys []map[string]interface{}
	json.NewDecoder(w.Body).Decode(&keys)
	if len(keys) != 1 {
		t.Fatalf("want 1 key, got %d", len(keys))
	}
	if keys[0]["id"] != key1 {
		t.Errorf("want %s, got %v", key1, keys[0]["id"])
	}
}

func TestVerifyAPIKey_NoProviderID(t *testing.T) {
	h := handler.NewAPIKeyHandler(&mockAPIKeyService{}, nil)
	body := []byte(`{"api_key":"k","provided_api_id":"p"}`)
	req := httptest.NewRequest(http.MethodPost, "/provider/keys/verify", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.VerifyAPIKey(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func TestVerifyAPIKey_InvalidJSON(t *testing.T) {
	h := handler.NewAPIKeyHandler(&mockAPIKeyService{}, nil)
	req := httptest.NewRequest(http.MethodPost, "/provider/keys/verify", bytes.NewReader([]byte("bad")))
	ctx := context.WithValue(req.Context(), middleware.ContextKeyProviderID, "prov-1")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	h.VerifyAPIKey(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestVerifyAPIKey_MissingFields(t *testing.T) {
	h := handler.NewAPIKeyHandler(&mockAPIKeyService{}, nil)
	body := []byte(`{"api_key":"","provided_api_id":""}`)
	req := httptest.NewRequest(http.MethodPost, "/provider/keys/verify", bytes.NewReader(body))
	ctx := context.WithValue(req.Context(), middleware.ContextKeyProviderID, "prov-1")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	h.VerifyAPIKey(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestVerifyAPIKey_InvalidKey(t *testing.T) {
	mock := &mockAPIKeyService{
		verifyAPIKeyFunc: func(_ context.Context, _, _, _ string) (*repository.VerifyResult, error) {
			return nil, nil
		},
	}
	h := handler.NewAPIKeyHandler(mock, nil)
	body := []byte(`{"api_key":"bad-key","provided_api_id":"44444444-4444-4444-4444-444444444444"}`)
	req := httptest.NewRequest(http.MethodPost, "/provider/keys/verify", bytes.NewReader(body))
	ctx := context.WithValue(req.Context(), middleware.ContextKeyProviderID, "prov-1")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	h.VerifyAPIKey(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["valid"] != false {
		t.Errorf("want valid=false, got %v", resp["valid"])
	}
}

func TestVerifyAPIKey_ValidKey(t *testing.T) {
	mock := &mockAPIKeyService{
		verifyAPIKeyFunc: func(_ context.Context, _, _, _ string) (*repository.VerifyResult, error) {
			return &repository.VerifyResult{ID: "11111111-1111-1111-1111-111111111111", CustomerID: "cccccccc-cccc-cccc-cccc-cccccccccccc", UsageCount: 5}, nil
		},
	}
	h := handler.NewAPIKeyHandler(mock, nil)
	body := []byte(`{"api_key":"good-key","provided_api_id":"44444444-4444-4444-4444-444444444444"}`)
	req := httptest.NewRequest(http.MethodPost, "/provider/keys/verify", bytes.NewReader(body))
	ctx := context.WithValue(req.Context(), middleware.ContextKeyProviderID, "prov-1")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	h.VerifyAPIKey(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["valid"] != true {
		t.Errorf("want valid=true, got %v", resp["valid"])
	}
	if resp["customer_id"] != "cccccccc-cccc-cccc-cccc-cccccccccccc" {
		t.Errorf("want customer_id=cccccccc-cccc-cccc-cccc-cccccccccccc, got %v", resp["customer_id"])
	}
	if resp["usage_count"] != float64(5) {
		t.Errorf("want usage_count=5, got %v", resp["usage_count"])
	}
}

func TestVerifyAPIKey_RateLimited(t *testing.T) {
	mock := &mockAPIKeyService{
		verifyAPIKeyFunc: func(_ context.Context, _, _, _ string) (*repository.VerifyResult, error) {
			return nil, repository.ErrRateLimited
		},
	}
	h := handler.NewAPIKeyHandler(mock, nil)
	body := []byte(`{"api_key":"some-key","provided_api_id":"44444444-4444-4444-4444-444444444444"}`)
	req := httptest.NewRequest(http.MethodPost, "/provider/keys/verify", bytes.NewReader(body))
	ctx := context.WithValue(req.Context(), middleware.ContextKeyProviderID, "prov-1")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	h.VerifyAPIKey(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("want 429, got %d", w.Code)
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["error"] != "rate_limit_exceeded" {
		t.Errorf("want error=rate_limit_exceeded, got %v", resp["error"])
	}
}

func TestDeleteAPIKey_NoCustomerID_Returns401(t *testing.T) {
	h := handler.NewAPIKeyHandler(&mockAPIKeyService{}, nil)
	req := httptest.NewRequest(http.MethodDelete, "/apikeys/key-1", nil)
	req = withChiID(req, "key-1")
	w := httptest.NewRecorder()

	h.DeleteAPIKey(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func TestDeleteAPIKey_NotFound_Returns404(t *testing.T) {
	mock := &mockAPIKeyService{
		deleteAPIKeyFunc: func(_ context.Context, _, _ string) error {
			return sql.ErrNoRows
		},
	}
	h := handler.NewAPIKeyHandler(mock, nil)
	req := httptest.NewRequest(http.MethodDelete, "/apikeys/key-1", nil)
	req = withChiID(req, "key-1")
	ctx := context.WithValue(req.Context(), middleware.ContextKeyCustomerID, "customer-A")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	h.DeleteAPIKey(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", w.Code)
	}
}

func TestDeleteAPIKey_Success_Returns204(t *testing.T) {
	h := handler.NewAPIKeyHandler(&mockAPIKeyService{}, nil)
	req := httptest.NewRequest(http.MethodDelete, "/apikeys/key-1", nil)
	req = withChiID(req, "key-1")
	ctx := context.WithValue(req.Context(), middleware.ContextKeyCustomerID, "customer-A")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	h.DeleteAPIKey(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("want 204, got %d", w.Code)
	}
}

func TestAPIKeyHandler_IssueAPIKey_LimitExceeded(t *testing.T) {
	mock := &mockAPIKeyService{
		issueAPIKeyFunc: func(_ context.Context, _ string, _ *string) (string, *model.APIKey, error) {
			return "", nil, repository.ErrAPIKeyLimitExceeded
		},
	}
	h := handler.NewAPIKeyHandler(mock, nil)
	body := []byte(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/apikeys", bytes.NewReader(body))
	ctx := context.WithValue(req.Context(), middleware.ContextKeyCustomerID, "customer-xyz")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	h.IssueAPIKey(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", w.Code)
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["error"] != "api_key_limit_exceeded" {
		t.Errorf("want error=api_key_limit_exceeded, got %v", resp["error"])
	}
}

func TestGetCustomerAPIKeys_NoCustomerID_Returns401(t *testing.T) {
	db := setupCustomerAPIKeysTestDB(t)
	h := handler.NewAPIKeyHandler(nil, db)

	req := httptest.NewRequest(http.MethodGet, "/customer/apikeys", nil)
	w := httptest.NewRecorder()

	h.GetCustomerAPIKeys(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func TestGetCustomerAPIKeys_Empty(t *testing.T) {
	db := setupCustomerAPIKeysTestDB(t)
	db.Exec(`INSERT INTO customers (id, name, email) VALUES ('cust-empty', 'Empty Customer', 'empty@example.com')`)

	h := handler.NewAPIKeyHandler(nil, db)
	req := httptest.NewRequest(http.MethodGet, "/customer/apikeys", nil)
	ctx := context.WithValue(req.Context(), middleware.ContextKeyCustomerID, "cust-empty")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	h.GetCustomerAPIKeys(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var keys []map[string]interface{}
	json.NewDecoder(w.Body).Decode(&keys)
	if keys == nil {
		t.Error("want empty array [], got null")
	}
	if len(keys) != 0 {
		t.Errorf("want 0 keys, got %d", len(keys))
	}
}

func TestAPIKeyHandler_IssueAPIKey_InvalidJSON_WithCustomerID(t *testing.T) {
	h := handler.NewAPIKeyHandler(&mockAPIKeyService{}, nil)
	req := httptest.NewRequest(http.MethodPost, "/apikeys", bytes.NewReader([]byte("bad json")))
	ctx := context.WithValue(req.Context(), middleware.ContextKeyCustomerID, "customer-xyz")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	h.IssueAPIKey(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestDeleteAPIKey_NoKeyID_Returns400(t *testing.T) {
	h := handler.NewAPIKeyHandler(&mockAPIKeyService{}, nil)
	req := httptest.NewRequest(http.MethodDelete, "/apikeys/", nil)
	ctx := context.WithValue(req.Context(), middleware.ContextKeyCustomerID, "customer-A")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	h.DeleteAPIKey(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestVerifyAPIKey_InternalError(t *testing.T) {
	mock := &mockAPIKeyService{
		verifyAPIKeyFunc: func(_ context.Context, _, _, _ string) (*repository.VerifyResult, error) {
			return nil, errors.New("database connection lost")
		},
	}
	h := handler.NewAPIKeyHandler(mock, nil)
	body := []byte(`{"api_key":"some-key","provided_api_id":"44444444-4444-4444-4444-444444444444"}`)
	req := httptest.NewRequest(http.MethodPost, "/provider/keys/verify", bytes.NewReader(body))
	ctx := context.WithValue(req.Context(), middleware.ContextKeyProviderID, "prov-1")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	h.VerifyAPIKey(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", w.Code)
	}
}
