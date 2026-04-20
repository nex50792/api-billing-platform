package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type ProviderRepository struct {
	DB *sql.DB
}

func NewProviderRepository(db *sql.DB) *ProviderRepository {
	return &ProviderRepository{DB: db}
}

type ProviderListItem struct {
	ID       string
	Name     string
	Email    string
	IsActive bool
}

func (r *ProviderRepository) List(ctx context.Context, limit, offset int) ([]ProviderListItem, error) {
	rows, err := r.DB.QueryContext(ctx,
		`SELECT id, name, email, is_active FROM providers ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
		limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("repository.ProviderList: %w", err)
	}
	defer rows.Close()
	out := []ProviderListItem{}
	for rows.Next() {
		var p ProviderListItem
		if err := rows.Scan(&p.ID, &p.Name, &p.Email, &p.IsActive); err != nil {
			continue
		}
		out = append(out, p)
	}
	return out, nil
}

func (r *ProviderRepository) ExistingByEmail(ctx context.Context, email string) (string, error) {
	var id string
	err := r.DB.QueryRowContext(ctx,
		`SELECT id FROM providers WHERE email = $1`,
		email,
	).Scan(&id)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (r *ProviderRepository) UpdatePasswordToken(ctx context.Context, id, tokenHash string, expiry time.Time) error {
	_, err := r.DB.ExecContext(ctx,
		`UPDATE providers SET password_token_hash = $1, password_token_expiry = $2 WHERE id = $3`,
		tokenHash, expiry, id,
	)
	if err != nil {
		return fmt.Errorf("repository.UpdatePasswordToken: %w", err)
	}
	return nil
}

// 新規プロバイダーは is_active=false で挿入する（パスワード設定完了で true になる）。
func (r *ProviderRepository) Create(ctx context.Context, id, name, email, tokenHash string, expiry time.Time) error {
	_, err := r.DB.ExecContext(ctx,
		`INSERT INTO providers (id, name, email, created_at, password_token_hash, password_token_expiry, is_active) VALUES ($1, $2, $3, now(), $4, $5, false)`,
		id, name, email, tokenHash, expiry,
	)
	if err != nil {
		return fmt.Errorf("repository.ProviderCreate: %w", err)
	}
	return nil
}

func (r *ProviderRepository) GetActiveIDByEmail(ctx context.Context, email string) (string, error) {
	var id string
	err := r.DB.QueryRowContext(ctx,
		`SELECT id FROM providers WHERE email = $1 AND is_active = true`, email,
	).Scan(&id)
	if err != nil {
		return "", err
	}
	return id, nil
}

// トークンが無い場合は sql.ErrNoRows を返す。
func (r *ProviderRepository) GetPasswordTokenExpiry(ctx context.Context, tokenHash string) (time.Time, error) {
	var expiry time.Time
	err := r.DB.QueryRowContext(ctx,
		`SELECT password_token_expiry FROM providers WHERE password_token_hash = $1`, tokenHash,
	).Scan(&expiry)
	return expiry, err
}

func (r *ProviderRepository) SetPassword(ctx context.Context, tokenHash, passwordHash string) (string, error) {
	var providerID string
	err := r.DB.QueryRowContext(ctx,
		`UPDATE providers SET password_hash=$1, is_active=true, password_token_hash=NULL, password_token_expiry=NULL WHERE password_token_hash=$2 RETURNING id`,
		passwordHash, tokenHash,
	).Scan(&providerID)
	if err != nil {
		return "", err
	}
	return providerID, nil
}

func (r *ProviderRepository) GetCredentials(ctx context.Context, email string) (id, passwordHash string, err error) {
	err = r.DB.QueryRowContext(ctx,
		`SELECT id, password_hash FROM providers WHERE email = $1 AND is_active = true`, email,
	).Scan(&id, &passwordHash)
	if err != nil {
		return "", "", err
	}
	return id, passwordHash, nil
}

func (r *ProviderRepository) GetPasswordHash(ctx context.Context, id string) (string, error) {
	var hash string
	err := r.DB.QueryRowContext(ctx,
		`SELECT password_hash FROM providers WHERE id = $1`, id,
	).Scan(&hash)
	if err != nil {
		return "", err
	}
	return hash, nil
}

func (r *ProviderRepository) UpdatePasswordHash(ctx context.Context, id, passwordHash string) error {
	_, err := r.DB.ExecContext(ctx,
		`UPDATE providers SET password_hash = $1 WHERE id = $2`,
		passwordHash, id,
	)
	if err != nil {
		return fmt.Errorf("repository.ProviderUpdatePasswordHash: %w", err)
	}
	return nil
}

type ProviderMe struct {
	ID    string
	Name  string
	Email string
}

func (r *ProviderRepository) GetMe(ctx context.Context, id string) (*ProviderMe, error) {
	var p ProviderMe
	err := r.DB.QueryRowContext(ctx,
		`SELECT id, name, email FROM providers WHERE id = $1`, id,
	).Scan(&p.ID, &p.Name, &p.Email)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *ProviderRepository) Delete(ctx context.Context, id string) (int64, error) {
	res, err := r.DB.ExecContext(ctx, `DELETE FROM providers WHERE id = $1`, id)
	if err != nil {
		return 0, fmt.Errorf("repository.ProviderDelete: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("repository.ProviderDelete rowsAffected: %w", err)
	}
	return n, nil
}

func (r *ProviderRepository) CreateSession(ctx context.Context, providerID, tokenHash string, expiresAt time.Time) error {
	_, err := r.DB.ExecContext(ctx,
		`INSERT INTO provider_sessions (provider_id, token_hash, expires_at) VALUES ($1, $2, $3)`,
		providerID, tokenHash, expiresAt,
	)
	if err != nil {
		return fmt.Errorf("repository.ProviderCreateSession: %w", err)
	}
	return nil
}

func (r *ProviderRepository) DeleteSessionByTokenHash(ctx context.Context, tokenHash string) error {
	_, err := r.DB.ExecContext(ctx, `DELETE FROM provider_sessions WHERE token_hash = $1`, tokenHash)
	if err != nil {
		return fmt.Errorf("repository.ProviderDeleteSessionByTokenHash: %w", err)
	}
	return nil
}

func (r *ProviderRepository) DeleteAllSessions(ctx context.Context, providerID string) error {
	_, err := r.DB.ExecContext(ctx, `DELETE FROM provider_sessions WHERE provider_id = $1`, providerID)
	if err != nil {
		return fmt.Errorf("repository.ProviderDeleteAllSessions: %w", err)
	}
	return nil
}

func (r *ProviderRepository) DeleteOtherSessions(ctx context.Context, providerID, exceptTokenHash string) error {
	_, err := r.DB.ExecContext(ctx,
		`DELETE FROM provider_sessions WHERE provider_id = $1 AND token_hash != $2`,
		providerID, exceptTokenHash,
	)
	if err != nil {
		return fmt.Errorf("repository.ProviderDeleteOtherSessions: %w", err)
	}
	return nil
}

func (r *ProviderRepository) GetProviderIDByTokenHash(ctx context.Context, tokenHash string) (string, error) {
	var providerID string
	err := r.DB.QueryRowContext(ctx,
		`SELECT provider_id FROM provider_sessions WHERE token_hash = $1 AND expires_at > now()`,
		tokenHash,
	).Scan(&providerID)
	return providerID, err
}
