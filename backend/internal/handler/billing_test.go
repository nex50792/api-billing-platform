package handler_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"api-billing-platform/internal/handler"
	"api-billing-platform/internal/middleware"
	"api-billing-platform/internal/model"
	"api-billing-platform/internal/repository"
)

type billingMockRepo struct {
	getCustomerByAPIKeyFunc func(ctx context.Context, apiKeyID string) (*model.Customer, error)
}

func (m *billingMockRepo) Create(_ context.Context, _, _ string, _ *string) (*model.APIKey, error) {
	return nil, nil
}
func (m *billingMockRepo) GetByID(_ context.Context, _ string) (*model.APIKey, error) {
	return nil, nil
}
func (m *billingMockRepo) GetByPlainKey(_ context.Context, _ string) (*model.APIKey, error) {
	return nil, nil
}
func (m *billingMockRepo) Delete(_ context.Context, _, _ string) error               { return nil }
func (m *billingMockRepo) IncrementUsage(_ context.Context, _ string) error          { return nil }
func (m *billingMockRepo) ResetUsage(_ context.Context, _ string, _ time.Time) error { return nil }
func (m *billingMockRepo) GetCustomerByAPIKey(ctx context.Context, apiKeyID string) (*model.Customer, error) {
	if m.getCustomerByAPIKeyFunc != nil {
		return m.getCustomerByAPIKeyFunc(ctx, apiKeyID)
	}
	return &model.Customer{ID: "11111111-1111-1111-1111-111111111111", Plan: "free"}, nil
}
func (m *billingMockRepo) VerifyForProvider(_ context.Context, _, _, _ string) (*repository.VerifyResult, error) {
	return nil, nil
}
func (m *billingMockRepo) ListByCustomer(_ context.Context, _ string) ([]*model.APIKey, error) {
	return nil, nil
}

func setupBillingTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	return db
}

func setupBillingTestDBWithProvidedAPI(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	db.Exec(`CREATE TABLE provided_apis (id TEXT PRIMARY KEY, name TEXT)`)
	db.Exec(`CREATE TABLE api_keys (id TEXT PRIMARY KEY, customer_id TEXT, provided_api_id TEXT)`)
	db.Exec(`INSERT INTO provided_apis VALUES ('22222222-2222-2222-2222-222222222222', 'My Linked API')`)
	db.Exec(`INSERT INTO api_keys (id, customer_id, provided_api_id) VALUES ('33333333-3333-3333-3333-333333333333', '11111111-1111-1111-1111-111111111111', '22222222-2222-2222-2222-222222222222')`)
	return db
}

func testAPIKey() *model.APIKey {
	return &model.APIKey{
		ID:           "33333333-3333-3333-3333-333333333333",
		CustomerID:   "11111111-1111-1111-1111-111111111111",
		UsageCount:   5,
		UsageResetAt: time.Now(),
		CreatedAt:    time.Now(),
	}
}

func TestBillingCheck_NoAPIKeyInContext(t *testing.T) {
	h := handler.NewBillingHandler(&billingMockRepo{}, setupBillingTestDB(t))
	req := httptest.NewRequest(http.MethodPost, "/billing/check", nil)
	w := httptest.NewRecorder()

	h.Check(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func TestBillingCheck_ValidKey_ReturnsCustomerInfo(t *testing.T) {
	repo := &billingMockRepo{
		getCustomerByAPIKeyFunc: func(_ context.Context, _ string) (*model.Customer, error) {
			return &model.Customer{ID: "11111111-1111-1111-1111-111111111111", Plan: "pro"}, nil
		},
	}
	h := handler.NewBillingHandler(repo, setupBillingTestDB(t))

	req := httptest.NewRequest(http.MethodPost, "/billing/check", nil)
	ctx := context.WithValue(req.Context(), middleware.ContextKeyAPIKey, testAPIKey())
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	h.Check(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	cust, ok := resp["customer"].(map[string]interface{})
	if !ok {
		t.Fatal("customer field missing")
	}
	if cust["id"] != "11111111-1111-1111-1111-111111111111" {
		t.Errorf("want id=11111111-1111-1111-1111-111111111111, got %v", cust["id"])
	}
	if cust["plan"] != "pro" {
		t.Errorf("want plan=pro, got %v", cust["plan"])
	}
	if resp["api_key"] == nil {
		t.Error("api_key field missing")
	}
}

func TestBillingCheck_WithProvidedAPI(t *testing.T) {
	repo := &billingMockRepo{
		getCustomerByAPIKeyFunc: func(_ context.Context, _ string) (*model.Customer, error) {
			return &model.Customer{ID: "11111111-1111-1111-1111-111111111111", Plan: "pro"}, nil
		},
	}
	h := handler.NewBillingHandler(repo, setupBillingTestDBWithProvidedAPI(t))

	req := httptest.NewRequest(http.MethodPost, "/billing/check", nil)
	ctx := context.WithValue(req.Context(), middleware.ContextKeyAPIKey, testAPIKey())
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	h.Check(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	pa, ok := resp["provided_api"].(map[string]interface{})
	if !ok || pa == nil {
		t.Fatal("provided_api field missing or nil")
	}
	if pa["id"] != "22222222-2222-2222-2222-222222222222" {
		t.Errorf("want provided_api.id=22222222-2222-2222-2222-222222222222, got %v", pa["id"])
	}
	if pa["name"] != "My Linked API" {
		t.Errorf("want provided_api.name=My Linked API, got %v", pa["name"])
	}
}

func TestBillingCheck_CustomerNotFound(t *testing.T) {
	repo := &billingMockRepo{
		getCustomerByAPIKeyFunc: func(_ context.Context, _ string) (*model.Customer, error) {
			return nil, nil
		},
	}
	h := handler.NewBillingHandler(repo, setupBillingTestDB(t))

	req := httptest.NewRequest(http.MethodPost, "/billing/check", nil)
	ctx := context.WithValue(req.Context(), middleware.ContextKeyAPIKey, testAPIKey())
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	h.Check(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}
