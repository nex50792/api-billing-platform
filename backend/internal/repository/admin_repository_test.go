package repository_test

import (
	"context"
	"testing"
	"time"

	"api-billing-platform/internal/repository"
)

func TestAdminRepository_FindByEmail_Found(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	repo := repository.NewAdminRepository(db)

	email := "admin-find-test@example.com"
	var adminID int
	err := db.QueryRow(
		`INSERT INTO admins (email, password_hash, name) VALUES ($1, $2, $3) RETURNING id`,
		email, "hashedpassword", "Find Test Admin",
	).Scan(&adminID)
	if err != nil {
		t.Fatalf("failed to insert admin: %v", err)
	}
	t.Cleanup(func() { db.Exec(`DELETE FROM admins WHERE id = $1`, adminID) })

	admin, err := repo.FindByEmail(email)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if admin == nil {
		t.Fatal("expected admin, got nil")
	}
	if admin.Email != email {
		t.Errorf("want email=%s, got %s", email, admin.Email)
	}
	if admin.Name != "Find Test Admin" {
		t.Errorf("want name=Find Test Admin, got %s", admin.Name)
	}
}

func TestAdminRepository_FindByEmail_NotFound(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	repo := repository.NewAdminRepository(db)

	admin, err := repo.FindByEmail("nonexistent-admin@example.com")
	if admin != nil {
		t.Errorf("expected nil for nonexistent email, got %+v", admin)
	}
	if err == nil {
		t.Error("expected error for nonexistent email, got nil")
	}
}

func TestAdminRepository_CreateSession_AndFindBySessionHash(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	repo := repository.NewAdminRepository(db)
	ctx := context.Background()

	email := "admin-session-test@example.com"
	var adminID int
	if err := db.QueryRow(
		`INSERT INTO admins (email, password_hash, name) VALUES ($1, $2, $3) RETURNING id`,
		email, "hashval", "Session Test Admin",
	).Scan(&adminID); err != nil {
		t.Fatalf("failed to insert admin: %v", err)
	}
	t.Cleanup(func() {
		db.Exec(`DELETE FROM admin_sessions WHERE admin_id = $1`, adminID)
		db.Exec(`DELETE FROM admins WHERE id = $1`, adminID)
	})

	tokenHash := "testhash-admin-session-" + email
	expiresAt := time.Now().Add(24 * time.Hour)

	if err := repo.CreateSession(ctx, adminID, tokenHash, expiresAt); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	admin, err := repo.FindBySessionHash(ctx, tokenHash)
	if err != nil {
		t.Fatalf("FindBySessionHash failed: %v", err)
	}
	if admin == nil {
		t.Fatal("expected admin, got nil")
	}
	if admin.ID != adminID {
		t.Errorf("want adminID=%d, got %d", adminID, admin.ID)
	}
}

func TestAdminRepository_FindBySessionHash_NotFound(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	repo := repository.NewAdminRepository(db)
	ctx := context.Background()

	admin, err := repo.FindBySessionHash(ctx, "nonexistent-hash-xyz")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if admin != nil {
		t.Errorf("expected nil for nonexistent hash, got %+v", admin)
	}
}

func TestAdminRepository_FindBySessionHash_Expired(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	repo := repository.NewAdminRepository(db)
	ctx := context.Background()

	email := "admin-expired-session@example.com"
	var adminID int
	if err := db.QueryRow(
		`INSERT INTO admins (email, password_hash, name) VALUES ($1, $2, $3) RETURNING id`,
		email, "hashval", "Expired Session Admin",
	).Scan(&adminID); err != nil {
		t.Fatalf("failed to insert admin: %v", err)
	}
	t.Cleanup(func() {
		db.Exec(`DELETE FROM admin_sessions WHERE admin_id = $1`, adminID)
		db.Exec(`DELETE FROM admins WHERE id = $1`, adminID)
	})

	expiredHash := "expired-hash-admin-" + email
	if _, err := db.Exec(
		`INSERT INTO admin_sessions (admin_id, token_hash, expires_at) VALUES ($1, $2, now() - interval '1 hour')`,
		adminID, expiredHash,
	); err != nil {
		t.Fatalf("failed to insert expired session: %v", err)
	}

	admin, err := repo.FindBySessionHash(ctx, expiredHash)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if admin != nil {
		t.Errorf("expected nil for expired session, got %+v", admin)
	}
}

func TestAdminRepository_DeleteSession(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	repo := repository.NewAdminRepository(db)
	ctx := context.Background()

	email := "admin-delete-session@example.com"
	var adminID int
	if err := db.QueryRow(
		`INSERT INTO admins (email, password_hash, name) VALUES ($1, $2, $3) RETURNING id`,
		email, "hashval", "Delete Session Admin",
	).Scan(&adminID); err != nil {
		t.Fatalf("failed to insert admin: %v", err)
	}
	t.Cleanup(func() {
		db.Exec(`DELETE FROM admin_sessions WHERE admin_id = $1`, adminID)
		db.Exec(`DELETE FROM admins WHERE id = $1`, adminID)
	})

	tokenHash := "delete-test-hash-" + email
	db.Exec(
		`INSERT INTO admin_sessions (admin_id, token_hash, expires_at) VALUES ($1, $2, now() + interval '1 hour')`,
		adminID, tokenHash,
	)

	if err := repo.DeleteSession(ctx, tokenHash); err != nil {
		t.Fatalf("DeleteSession failed: %v", err)
	}

	var count int
	db.QueryRow(`SELECT COUNT(*) FROM admin_sessions WHERE token_hash = $1`, tokenHash).Scan(&count)
	if count != 0 {
		t.Error("session should be deleted from DB")
	}
}
