package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	"api-billing-platform/internal/model"
	"api-billing-platform/internal/repository"
)

type APIKeyService struct {
	repo repository.APIKeyRepositoryInterface
}

func NewAPIKeyService(repo repository.APIKeyRepositoryInterface) *APIKeyService {
	return &APIKeyService{repo: repo}
}

func (s *APIKeyService) IssueAPIKey(ctx context.Context, customerID string, providedAPIID *string) (plainKey string, apiKey *model.APIKey, err error) {
	plainKey, err = generateAPIKey()
	if err != nil {
		return "", nil, err
	}
	apiKey, err = s.repo.Create(ctx, customerID, plainKey, providedAPIID)
	if err != nil {
		return "", nil, err
	}
	return plainKey, apiKey, nil
}

func generateAPIKey() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return "ak_" + hex.EncodeToString(b), nil
}

func (s *APIKeyService) VerifyAPIKey(ctx context.Context, plainKey, providedAPIID, providerID string) (*repository.VerifyResult, error) {
	return s.repo.VerifyForProvider(ctx, plainKey, providedAPIID, providerID)
}

func (s *APIKeyService) DeleteAPIKey(ctx context.Context, id, customerID string) error {
	return s.repo.Delete(ctx, id, customerID)
}

func (s *APIKeyService) ListCustomerKeys(ctx context.Context, customerID string) ([]*model.APIKey, error) {
	return s.repo.ListByCustomer(ctx, customerID)
}

type APIKeyServiceInterface interface {
	IssueAPIKey(ctx context.Context, customerID string, providedAPIID *string) (plainKey string, apiKey *model.APIKey, err error)
	VerifyAPIKey(ctx context.Context, plainKey, providedAPIID, providerID string) (*repository.VerifyResult, error)
	DeleteAPIKey(ctx context.Context, id, customerID string) error
	ListCustomerKeys(ctx context.Context, customerID string) ([]*model.APIKey, error)
}
