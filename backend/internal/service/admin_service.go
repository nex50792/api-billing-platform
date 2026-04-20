package service

import (
	"api-billing-platform/internal/repository"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type AdminService struct {
	Repo repository.AdminRepositoryInterface
}

func NewAdminService(repo repository.AdminRepositoryInterface) *AdminService {
	return &AdminService{Repo: repo}
}

func (s *AdminService) Authenticate(ctx context.Context, email, password string) (*repository.Admin, error) {
	admin, err := s.Repo.FindByEmail(email)
	if err != nil {
		return nil, err
	}
	if bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(password)) != nil {
		return nil, nil
	}
	return admin, nil
}

func (s *AdminService) CreateSession(ctx context.Context, adminID int, ttl time.Duration) (plainToken string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", err
	}
	plainToken = base64.RawURLEncoding.EncodeToString(b)
	tokenHash := hashAdminToken(plainToken)
	return plainToken, s.Repo.CreateSession(ctx, adminID, tokenHash, time.Now().Add(ttl))
}

func (s *AdminService) GetAdminBySession(ctx context.Context, plainToken string) (*repository.Admin, error) {
	return s.Repo.FindBySessionHash(ctx, hashAdminToken(plainToken))
}

func (s *AdminService) DeleteSession(ctx context.Context, plainToken string) error {
	return s.Repo.DeleteSession(ctx, hashAdminToken(plainToken))
}

func hashAdminToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
