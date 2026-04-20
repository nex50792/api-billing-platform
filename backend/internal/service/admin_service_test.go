package service_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"api-billing-platform/internal/repository"
	"api-billing-platform/internal/service"

	"golang.org/x/crypto/bcrypt"
)

type fakeAdminRepo struct {
	admin                   *repository.Admin
	findByEmailErr          error
	createSessionErr        error
	findBySessionHashResult *repository.Admin
	findBySessionHashErr    error
	deleteSessionErr        error

	createSessionCalled bool
	deleteSessionCalled bool
}

func (f *fakeAdminRepo) FindByEmail(_ string) (*repository.Admin, error) {
	return f.admin, f.findByEmailErr
}

func (f *fakeAdminRepo) CreateSession(_ context.Context, _ int, _ string, _ time.Time) error {
	f.createSessionCalled = true
	return f.createSessionErr
}

func (f *fakeAdminRepo) FindBySessionHash(_ context.Context, _ string) (*repository.Admin, error) {
	return f.findBySessionHashResult, f.findBySessionHashErr
}

func (f *fakeAdminRepo) DeleteSession(_ context.Context, _ string) error {
	f.deleteSessionCalled = true
	return f.deleteSessionErr
}

func TestAdminService_Authenticate_Success(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("correct-pass"), bcrypt.MinCost)
	repo := &fakeAdminRepo{
		admin: &repository.Admin{ID: 1, Email: "admin@example.com", PasswordHash: string(hash), Name: "Admin"},
	}
	svc := service.NewAdminService(repo)

	admin, err := svc.Authenticate(context.Background(), "admin@example.com", "correct-pass")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if admin == nil {
		t.Fatal("expected admin, got nil")
	}
	if admin.ID != 1 {
		t.Errorf("want ID=1, got %d", admin.ID)
	}
}

func TestAdminService_Authenticate_WrongPassword(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("correct-pass"), bcrypt.MinCost)
	repo := &fakeAdminRepo{
		admin: &repository.Admin{ID: 1, Email: "admin@example.com", PasswordHash: string(hash), Name: "Admin"},
	}
	svc := service.NewAdminService(repo)

	admin, err := svc.Authenticate(context.Background(), "admin@example.com", "wrong-pass")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if admin != nil {
		t.Errorf("want nil for wrong password, got %+v", admin)
	}
}

func TestAdminService_Authenticate_EmailNotFound(t *testing.T) {
	repo := &fakeAdminRepo{
		findByEmailErr: sql.ErrNoRows,
	}
	svc := service.NewAdminService(repo)

	admin, err := svc.Authenticate(context.Background(), "nobody@example.com", "pass")
	if err == nil {
		t.Fatal("expected error for nonexistent email, got nil")
	}
	if admin != nil {
		t.Errorf("want nil admin for error case, got %+v", admin)
	}
}

func TestAdminService_CreateSession_Success(t *testing.T) {
	repo := &fakeAdminRepo{}
	svc := service.NewAdminService(repo)

	plainToken, err := svc.CreateSession(context.Background(), 1, 24*time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plainToken == "" {
		t.Error("expected non-empty plain token")
	}
	if !repo.createSessionCalled {
		t.Error("expected CreateSession to be called on repo")
	}
}

func TestAdminService_GetAdminBySession_Found(t *testing.T) {
	expected := &repository.Admin{ID: 42, Email: "admin@example.com", Name: "Admin"}
	repo := &fakeAdminRepo{findBySessionHashResult: expected}
	svc := service.NewAdminService(repo)

	admin, err := svc.GetAdminBySession(context.Background(), "some-plain-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if admin == nil {
		t.Fatal("expected admin, got nil")
	}
	if admin.ID != 42 {
		t.Errorf("want ID=42, got %d", admin.ID)
	}
}

func TestAdminService_GetAdminBySession_NotFound(t *testing.T) {
	repo := &fakeAdminRepo{findBySessionHashResult: nil}
	svc := service.NewAdminService(repo)

	admin, err := svc.GetAdminBySession(context.Background(), "nonexistent-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if admin != nil {
		t.Errorf("want nil for nonexistent session, got %+v", admin)
	}
}

func TestAdminService_DeleteSession(t *testing.T) {
	repo := &fakeAdminRepo{}
	svc := service.NewAdminService(repo)

	if err := svc.DeleteSession(context.Background(), "some-plain-token"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !repo.deleteSessionCalled {
		t.Error("expected DeleteSession to be called on repo")
	}
}
