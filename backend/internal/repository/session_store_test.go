package repository_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"api-billing-platform/internal/repository"
)

func TestDBProviderSessionStore_GetProviderByTokenHash_Found(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	store := repository.NewDBProviderSessionStore(db)
	ctx := context.Background()

	const providerID = "00000000-0000-0000-0000-000000000002"
	tokenHash := "provider-store-test-hash-found"
	expiresAt := time.Now().Add(time.Hour).UTC().Truncate(time.Second)

	_, err := db.Exec(
		`INSERT INTO provider_sessions (provider_id, token_hash, expires_at) VALUES ($1, $2, $3)`,
		providerID, tokenHash, expiresAt,
	)
	if err != nil {
		t.Fatalf("failed to insert session: %v", err)
	}
	t.Cleanup(func() {
		db.Exec(`DELETE FROM provider_sessions WHERE token_hash = $1`, tokenHash)
	})

	gotID, gotExpiry, err := store.GetProviderByTokenHash(ctx, tokenHash)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotID != providerID {
		t.Errorf("want providerID=%s, got %s", providerID, gotID)
	}
	if gotExpiry.IsZero() {
		t.Error("want non-zero expiresAt")
	}
}

func TestDBProviderSessionStore_GetProviderByTokenHash_NotFound(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	store := repository.NewDBProviderSessionStore(db)
	ctx := context.Background()

	_, _, err := store.GetProviderByTokenHash(ctx, "nonexistent-provider-hash-xyz")
	if err == nil {
		t.Fatal("expected error for nonexistent hash, got nil")
	}
	if err != sql.ErrNoRows {
		t.Errorf("want sql.ErrNoRows, got %v", err)
	}
}

func TestDBCustomerSessionStore_GetCustomerByTokenHash_Found(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	store := repository.NewDBCustomerSessionStore(db)
	ctx := context.Background()

	var customerID string
	if err := db.QueryRow(
		`INSERT INTO customers (name, email, password_hash) VALUES ($1, $2, $3) RETURNING id`,
		"Session Store Test Customer", "session-store-cust@example.com", "hashedpw",
	).Scan(&customerID); err != nil {
		t.Fatalf("failed to insert customer: %v", err)
	}
	t.Cleanup(func() {
		db.Exec(`DELETE FROM customer_sessions WHERE customer_id = $1`, customerID)
		db.Exec(`DELETE FROM customers WHERE id = $1`, customerID)
	})

	tokenHash := "customer-store-test-hash-found"
	expiresAt := time.Now().Add(time.Hour).UTC().Truncate(time.Second)

	if _, err := db.Exec(
		`INSERT INTO customer_sessions (customer_id, token_hash, expires_at) VALUES ($1, $2, $3)`,
		customerID, tokenHash, expiresAt,
	); err != nil {
		t.Fatalf("failed to insert customer session: %v", err)
	}

	gotID, gotExpiry, err := store.GetCustomerByTokenHash(ctx, tokenHash)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotID != customerID {
		t.Errorf("want customerID=%s, got %s", customerID, gotID)
	}
	if gotExpiry.IsZero() {
		t.Error("want non-zero expiresAt")
	}
}

func TestDBCustomerSessionStore_GetCustomerByTokenHash_NotFound(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	store := repository.NewDBCustomerSessionStore(db)
	ctx := context.Background()

	_, _, err := store.GetCustomerByTokenHash(ctx, "nonexistent-customer-hash-xyz")
	if err == nil {
		t.Fatal("expected error for nonexistent hash, got nil")
	}
	if err != sql.ErrNoRows {
		t.Errorf("want sql.ErrNoRows, got %v", err)
	}
}
