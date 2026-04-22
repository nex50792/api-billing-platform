package service_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"api-billing-platform/internal/model"
	"api-billing-platform/internal/repository"
	"api-billing-platform/internal/service"
)

type mockAPIKeyRepository struct {
	createFunc            func(ctx context.Context, customerID, plainKey string, providedAPIID *string) (*model.APIKey, error)
	getByIDFunc           func(ctx context.Context, id string) (*model.APIKey, error)
	getByPlainKeyFunc     func(ctx context.Context, plainKey string) (*model.APIKey, error)
	incrementUsageFunc    func(ctx context.Context, id string) error
	resetUsageFunc        func(ctx context.Context, id string) error
	getCustomerByAPIKey   func(ctx context.Context, apiKeyID string) (*model.Customer, error)
	verifyForProviderFunc func(ctx context.Context, plainKey, providedAPIID, providerID string) (*repository.VerifyResult, error)
	deleteFunc            func(ctx context.Context, id, customerID string) error
}

func (m *mockAPIKeyRepository) Create(ctx context.Context, customerID, plainKey string, providedAPIID *string) (*model.APIKey, error) {
	if m.createFunc != nil {
		return m.createFunc(ctx, customerID, plainKey, providedAPIID)
	}
	return &model.APIKey{ID: "key-123", CustomerID: customerID, ProvidedAPIID: providedAPIID, UsageCount: 0}, nil
}

func (m *mockAPIKeyRepository) GetByID(ctx context.Context, id string) (*model.APIKey, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, id)
	}
	return &model.APIKey{ID: id, UsageCount: 0}, nil
}

func (m *mockAPIKeyRepository) GetByPlainKey(ctx context.Context, plainKey string) (*model.APIKey, error) {
	if m.getByPlainKeyFunc != nil {
		return m.getByPlainKeyFunc(ctx, plainKey)
	}
	return &model.APIKey{ID: "key-123", UsageCount: 0}, nil
}

func (m *mockAPIKeyRepository) IncrementUsage(ctx context.Context, id string) error {
	if m.incrementUsageFunc != nil {
		return m.incrementUsageFunc(ctx, id)
	}
	return nil
}

func (m *mockAPIKeyRepository) ResetUsage(ctx context.Context, id string, resetAt time.Time) error {
	if m.resetUsageFunc != nil {
		return m.resetUsageFunc(ctx, id)
	}
	return nil
}

func (m *mockAPIKeyRepository) GetCustomerByAPIKey(ctx context.Context, apiKeyID string) (*model.Customer, error) {
	if m.getCustomerByAPIKey != nil {
		return m.getCustomerByAPIKey(ctx, apiKeyID)
	}
	return &model.Customer{ID: "cust-123", Plan: "free"}, nil
}

func (m *mockAPIKeyRepository) VerifyForProvider(ctx context.Context, plainKey, providedAPIID, providerID string) (*repository.VerifyResult, error) {
	if m.verifyForProviderFunc != nil {
		return m.verifyForProviderFunc(ctx, plainKey, providedAPIID, providerID)
	}
	return nil, nil
}

func (m *mockAPIKeyRepository) Delete(ctx context.Context, id, customerID string) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, id, customerID)
	}
	return nil
}

func (m *mockAPIKeyRepository) ListByCustomer(_ context.Context, _ string) ([]*model.APIKey, error) {
	return nil, nil
}

func TestAPIKeyService_IssueAPIKey_GeneratesValidKey(t *testing.T) {
	svc := service.NewAPIKeyService(&mockAPIKeyRepository{})
	ctx := context.Background()

	plainKey, apiKey, err := svc.IssueAPIKey(ctx, "cust-123", nil)
	if err != nil {
		t.Fatalf("failed to issue key: %v", err)
	}
	if plainKey == "" {
		t.Fatal("plain key should not be empty")
	}
	if apiKey == nil {
		t.Fatal("api key should not be nil")
	}
	if len(plainKey) < 32 {
		t.Errorf("expected plain key length >= 32, got %d", len(plainKey))
	}
}

func TestAPIKeyService_IssueAPIKey_Success(t *testing.T) {
	mock := &mockAPIKeyRepository{
		createFunc: func(ctx context.Context, customerID, plainKey string, providedAPIID *string) (*model.APIKey, error) {
			return &model.APIKey{ID: "key-456", CustomerID: customerID, ProvidedAPIID: providedAPIID, UsageCount: 0}, nil
		},
	}

	pid := "api-service-1"
	svc := service.NewAPIKeyService(mock)
	plainKey, apiKey, err := svc.IssueAPIKey(context.Background(), "cust-123", &pid)

	if err != nil {
		t.Fatalf("should not error: %v", err)
	}
	if plainKey == "" {
		t.Fatal("plain key should not be empty")
	}
	if apiKey == nil || apiKey.ID != "key-456" {
		t.Fatal("api key should be returned from repository")
	}
	if apiKey.CustomerID != "cust-123" {
		t.Errorf("customer ID mismatch: %s", apiKey.CustomerID)
	}
	if apiKey.ProvidedAPIID == nil || *apiKey.ProvidedAPIID != pid {
		t.Errorf("provided_api_id mismatch: %v", apiKey.ProvidedAPIID)
	}
}

func TestAPIKeyService_IssueAPIKey_Initialization(t *testing.T) {
	svc := service.NewAPIKeyService(nil)
	if svc == nil {
		t.Fatal("service should not be nil")
	}
}

func TestAPIKeyService_VerifyAPIKey_Valid(t *testing.T) {
	mock := &mockAPIKeyRepository{
		verifyForProviderFunc: func(_ context.Context, plainKey, providedAPIID, providerID string) (*repository.VerifyResult, error) {
			if plainKey == "good-key" && providedAPIID == "api-1" && providerID == "prov-1" {
				return &repository.VerifyResult{ID: "key-1", CustomerID: "cust-1", UsageCount: 3}, nil
			}
			return nil, nil
		},
	}
	svc := service.NewAPIKeyService(mock)
	result, err := svc.VerifyAPIKey(context.Background(), "good-key", "api-1", "prov-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.CustomerID != "cust-1" {
		t.Errorf("want cust-1, got %s", result.CustomerID)
	}
	if result.UsageCount != 3 {
		t.Errorf("want usage 3, got %d", result.UsageCount)
	}
}

func TestAPIKeyService_VerifyAPIKey_Invalid(t *testing.T) {
	mock := &mockAPIKeyRepository{}
	svc := service.NewAPIKeyService(mock)
	result, err := svc.VerifyAPIKey(context.Background(), "bad-key", "api-1", "prov-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil for invalid key, got %+v", result)
	}
}

func TestAPIKeyService_DeleteAPIKey_Success(t *testing.T) {
	mock := &mockAPIKeyRepository{}
	svc := service.NewAPIKeyService(mock)
	if err := svc.DeleteAPIKey(context.Background(), "key-1", "cust-1"); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestAPIKeyService_DeleteAPIKey_NotFound(t *testing.T) {
	mock := &mockAPIKeyRepository{
		deleteFunc: func(_ context.Context, _, _ string) error {
			return sql.ErrNoRows
		},
	}
	svc := service.NewAPIKeyService(mock)
	err := svc.DeleteAPIKey(context.Background(), "nonexistent", "cust-1")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("expected sql.ErrNoRows, got: %v", err)
	}
}

func TestAPIKeyService_IssueAPIKey_RepositoryError(t *testing.T) {
	repoErr := errors.New("db connection failed")
	mock := &mockAPIKeyRepository{
		createFunc: func(_ context.Context, _, _ string, _ *string) (*model.APIKey, error) {
			return nil, repoErr
		},
	}
	svc := service.NewAPIKeyService(mock)
	plainKey, apiKey, err := svc.IssueAPIKey(context.Background(), "cust-1", nil)
	if !errors.Is(err, repoErr) {
		t.Errorf("expected repo error, got: %v", err)
	}
	if plainKey != "" {
		t.Error("expected empty plain key on error")
	}
	if apiKey != nil {
		t.Error("expected nil api key on error")
	}
}

func TestAPIKeyService_VerifyAPIKey_RateLimited(t *testing.T) {
	mock := &mockAPIKeyRepository{
		verifyForProviderFunc: func(_ context.Context, _, _, _ string) (*repository.VerifyResult, error) {
			return nil, repository.ErrRateLimited
		},
	}
	svc := service.NewAPIKeyService(mock)
	result, err := svc.VerifyAPIKey(context.Background(), "some-key", "api-1", "prov-1")
	if !errors.Is(err, repository.ErrRateLimited) {
		t.Errorf("expected ErrRateLimited, got: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result on rate limit, got: %+v", result)
	}
}
