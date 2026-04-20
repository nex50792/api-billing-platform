package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type Admin struct {
	ID           int
	Email        string
	PasswordHash string
	Name         string
	IsActive     bool
}

type AdminRepositoryInterface interface {
	FindByEmail(email string) (*Admin, error)
	CreateSession(ctx context.Context, adminID int, tokenHash string, expiresAt time.Time) error
	FindBySessionHash(ctx context.Context, tokenHash string) (*Admin, error)
	DeleteSession(ctx context.Context, tokenHash string) error
}

type AdminRepository struct {
	DB *sql.DB
}

func NewAdminRepository(db *sql.DB) *AdminRepository {
	return &AdminRepository{DB: db}
}

func (r *AdminRepository) FindByEmail(email string) (*Admin, error) {
	row := r.DB.QueryRow(
		`SELECT id, email, password_hash, name, is_active FROM admins WHERE email = $1 AND is_active = TRUE`,
		email,
	)
	var a Admin
	if err := row.Scan(&a.ID, &a.Email, &a.PasswordHash, &a.Name, &a.IsActive); err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *AdminRepository) CreateSession(ctx context.Context, adminID int, tokenHash string, expiresAt time.Time) error {
	_, err := r.DB.ExecContext(ctx,
		`INSERT INTO admin_sessions (admin_id, token_hash, expires_at) VALUES ($1, $2, $3)`,
		adminID, tokenHash, expiresAt,
	)
	return err
}

func (r *AdminRepository) FindBySessionHash(ctx context.Context, tokenHash string) (*Admin, error) {
	row := r.DB.QueryRowContext(ctx, `
		SELECT a.id, a.email, a.password_hash, a.name, a.is_active
		FROM admins a
		JOIN admin_sessions s ON s.admin_id = a.id
		WHERE s.token_hash = $1 AND s.expires_at > now()
	`, tokenHash)
	var a Admin
	if err := row.Scan(&a.ID, &a.Email, &a.PasswordHash, &a.Name, &a.IsActive); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &a, nil
}

func (r *AdminRepository) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := r.DB.ExecContext(ctx, `DELETE FROM admin_sessions WHERE token_hash = $1`, tokenHash)
	return err
}

type AdminCustomerRow struct {
	ID                 string
	Name               string
	Email              string
	Plan               string
	SubscriptionStatus string
	IsActive           bool
	CreatedAt          time.Time
}

// created_at は PG の timestamp / sqlite の TEXT のどちらでも扱えるよう flexTime で受ける。
func (r *AdminRepository) ListCustomers(ctx context.Context, limit, offset int) ([]AdminCustomerRow, error) {
	rows, err := r.DB.QueryContext(ctx,
		`SELECT id, name, email, plan, COALESCE(subscription_status, ''), is_active, created_at FROM customers ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
		limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("repository.ListCustomers: %w", err)
	}
	defer rows.Close()
	out := []AdminCustomerRow{}
	for rows.Next() {
		var c AdminCustomerRow
		var createdAt flexTime
		if err := rows.Scan(&c.ID, &c.Name, &c.Email, &c.Plan, &c.SubscriptionStatus, &c.IsActive, &createdAt); err != nil {
			continue
		}
		c.CreatedAt = createdAt.Time
		out = append(out, c)
	}
	return out, nil
}

type AdminEmailLogRow struct {
	ID           string
	ToEmail      string
	Subject      string
	Status       string
	ErrorMessage string
	SentAt       time.Time
}

func (r *AdminRepository) ListEmailLogs(ctx context.Context) ([]AdminEmailLogRow, error) {
	rows, err := r.DB.QueryContext(ctx,
		`SELECT id, to_email, subject, status, COALESCE(error_message, ''), sent_at FROM email_logs ORDER BY sent_at DESC LIMIT 200`,
	)
	if err != nil {
		return nil, fmt.Errorf("repository.ListEmailLogs: %w", err)
	}
	defer rows.Close()
	out := []AdminEmailLogRow{}
	for rows.Next() {
		var log AdminEmailLogRow
		var sentAt flexTime
		if err := rows.Scan(&log.ID, &log.ToEmail, &log.Subject, &log.Status, &log.ErrorMessage, &sentAt); err != nil {
			continue
		}
		log.SentAt = sentAt.Time
		out = append(out, log)
	}
	return out, nil
}

func (r *AdminRepository) UpdatePasswordHash(ctx context.Context, id int, passwordHash string) error {
	_, err := r.DB.ExecContext(ctx,
		`UPDATE admins SET password_hash = $1 WHERE id = $2`,
		passwordHash, id,
	)
	if err != nil {
		return fmt.Errorf("repository.AdminUpdatePasswordHash: %w", err)
	}
	return nil
}

func (r *AdminRepository) DeleteOtherSessions(ctx context.Context, adminID int, exceptTokenHash string) error {
	_, err := r.DB.ExecContext(ctx,
		`DELETE FROM admin_sessions WHERE admin_id = $1 AND token_hash != $2`,
		adminID, exceptTokenHash,
	)
	if err != nil {
		return fmt.Errorf("repository.AdminDeleteOtherSessions: %w", err)
	}
	return nil
}
