package repository

import (
	"context"
	"database/sql"
	"time"
)

type DBProviderSessionStore struct {
	DB *sql.DB
}

func NewDBProviderSessionStore(db *sql.DB) *DBProviderSessionStore {
	return &DBProviderSessionStore{DB: db}
}

func (s *DBProviderSessionStore) GetProviderByTokenHash(ctx context.Context, tokenHash string) (string, time.Time, error) {
	var providerID string
	var expiresAt time.Time
	err := s.DB.QueryRowContext(ctx,
		`SELECT provider_id, expires_at FROM provider_sessions WHERE token_hash = $1`,
		tokenHash,
	).Scan(&providerID, &expiresAt)
	return providerID, expiresAt, err
}

type DBCustomerSessionStore struct {
	DB *sql.DB
}

func NewDBCustomerSessionStore(db *sql.DB) *DBCustomerSessionStore {
	return &DBCustomerSessionStore{DB: db}
}

func (s *DBCustomerSessionStore) GetCustomerByTokenHash(ctx context.Context, tokenHash string) (string, time.Time, error) {
	var customerID string
	var expiresAt time.Time
	err := s.DB.QueryRowContext(ctx,
		`SELECT customer_id, expires_at FROM customer_sessions WHERE token_hash = $1`,
		tokenHash,
	).Scan(&customerID, &expiresAt)
	return customerID, expiresAt, err
}
