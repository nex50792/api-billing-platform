package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type CustomerRepository struct {
	DB *sql.DB
}

func NewCustomerRepository(db *sql.DB) *CustomerRepository {
	return &CustomerRepository{DB: db}
}

type IDAndActive struct {
	ID       string
	IsActive bool
}

func (r *CustomerRepository) GetByEmail(ctx context.Context, email string) (id, passwordHash string, err error) {
	err = r.DB.QueryRowContext(ctx,
		`SELECT id, password_hash FROM customers WHERE email = $1 AND is_active = true`,
		email,
	).Scan(&id, &passwordHash)
	if err != nil {
		return "", "", err
	}
	return id, passwordHash, nil
}

func (r *CustomerRepository) GetIDByEmailIncludingInactive(ctx context.Context, email string) (*IDAndActive, error) {
	var res IDAndActive
	err := r.DB.QueryRowContext(ctx,
		`SELECT id, is_active FROM customers WHERE email = $1`, email,
	).Scan(&res.ID, &res.IsActive)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("repository.GetIDByEmailIncludingInactive: %w", err)
	}
	return &res, nil
}

func (r *CustomerRepository) GetActiveIDByEmail(ctx context.Context, email string) (string, error) {
	var id string
	err := r.DB.QueryRowContext(ctx,
		`SELECT id FROM customers WHERE email = $1 AND is_active = true`, email,
	).Scan(&id)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (r *CustomerRepository) GetIDByEmailAny(ctx context.Context, email string) (string, error) {
	var id string
	err := r.DB.QueryRowContext(ctx,
		`SELECT id FROM customers WHERE email = $1`, email,
	).Scan(&id)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (r *CustomerRepository) GetPasswordHash(ctx context.Context, id string) (string, error) {
	var hash string
	err := r.DB.QueryRowContext(ctx,
		`SELECT password_hash FROM customers WHERE id = $1`, id,
	).Scan(&hash)
	if err != nil {
		return "", err
	}
	return hash, nil
}

type MeProfile struct {
	Name               string
	Email              string
	Plan               string
	SubscriptionStatus *string
}

func (r *CustomerRepository) GetMe(ctx context.Context, id string) (*MeProfile, error) {
	var p MeProfile
	var subStatus sql.NullString
	err := r.DB.QueryRowContext(ctx,
		`SELECT name, email, plan, subscription_status FROM customers WHERE id = $1 AND is_active = true`,
		id,
	).Scan(&p.Name, &p.Email, &p.Plan, &subStatus)
	if err != nil {
		return nil, err
	}
	if subStatus.Valid {
		p.SubscriptionStatus = &subStatus.String
	}
	return &p, nil
}

// 新規顧客は is_active=false で挿入する（メール認証で true になる）。
func (r *CustomerRepository) Create(ctx context.Context, id, name, email, passwordHash string) error {
	_, err := r.DB.ExecContext(ctx,
		`INSERT INTO customers (id, name, email, password_hash, created_at, is_active) VALUES ($1, $2, $3, $4, now(), false)`,
		id, name, email, passwordHash,
	)
	if err != nil {
		return fmt.Errorf("repository.CustomerCreate: %w", err)
	}
	return nil
}

// 非アクティブな既存顧客の再登録時に呼ばれ、name/password_hash/created_at を更新する。
func (r *CustomerRepository) UpdateForReissue(ctx context.Context, id, name, passwordHash string) error {
	_, err := r.DB.ExecContext(ctx,
		`UPDATE customers SET name=$1, password_hash=$2, created_at=now() WHERE id=$3`,
		name, passwordHash, id,
	)
	if err != nil {
		return fmt.Errorf("repository.CustomerUpdateForReissue: %w", err)
	}
	return nil
}

func (r *CustomerRepository) Activate(ctx context.Context, id string) error {
	_, err := r.DB.ExecContext(ctx, `UPDATE customers SET is_active = true WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("repository.CustomerActivate: %w", err)
	}
	return nil
}

func (r *CustomerRepository) UpdatePasswordHash(ctx context.Context, id, passwordHash string) error {
	_, err := r.DB.ExecContext(ctx,
		`UPDATE customers SET password_hash = $1 WHERE id = $2`,
		passwordHash, id,
	)
	if err != nil {
		return fmt.Errorf("repository.CustomerUpdatePasswordHash: %w", err)
	}
	return nil
}

type VerificationCode struct {
	Code      string
	ExpiresAt time.Time
}

func (r *CustomerRepository) GetLatestVerificationCode(ctx context.Context, customerID string) (*VerificationCode, error) {
	var v VerificationCode
	err := r.DB.QueryRowContext(ctx,
		`SELECT code, expires_at FROM customer_verification_codes WHERE customer_id = $1 AND is_used = false ORDER BY created_at DESC LIMIT 1`,
		customerID,
	).Scan(&v.Code, &v.ExpiresAt)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (r *CustomerRepository) MarkVerificationCodeUsed(ctx context.Context, customerID, code string) error {
	_, err := r.DB.ExecContext(ctx,
		`UPDATE customer_verification_codes SET is_used = true WHERE customer_id = $1 AND code = $2`,
		customerID, code,
	)
	if err != nil {
		return fmt.Errorf("repository.MarkVerificationCodeUsed: %w", err)
	}
	return nil
}

func (r *CustomerRepository) InvalidateUnusedVerificationCodes(ctx context.Context, customerID string) error {
	_, err := r.DB.ExecContext(ctx,
		`UPDATE customer_verification_codes SET is_used = true WHERE customer_id = $1 AND is_used = false`,
		customerID,
	)
	if err != nil {
		return fmt.Errorf("repository.InvalidateUnusedVerificationCodes: %w", err)
	}
	return nil
}

func (r *CustomerRepository) CreateVerificationCode(ctx context.Context, customerID, code string, expiresAt time.Time) error {
	_, err := r.DB.ExecContext(ctx,
		`INSERT INTO customer_verification_codes (customer_id, code, expires_at) VALUES ($1, $2, $3)`,
		customerID, code, expiresAt,
	)
	if err != nil {
		return fmt.Errorf("repository.CreateVerificationCode: %w", err)
	}
	return nil
}

func (r *CustomerRepository) CreatePasswordResetToken(ctx context.Context, id, customerID, tokenHash string, expiresAt time.Time) error {
	_, err := r.DB.ExecContext(ctx,
		`INSERT INTO customer_password_resets (id, customer_id, token_hash, expires_at) VALUES ($1, $2, $3, $4)`,
		id, customerID, tokenHash, expiresAt,
	)
	if err != nil {
		return fmt.Errorf("repository.CreatePasswordResetToken: %w", err)
	}
	return nil
}

type PasswordReset struct {
	ID         string
	CustomerID string
	ExpiresAt  time.Time
	UsedAt     sql.NullTime
}

func (r *CustomerRepository) GetPasswordResetByTokenHash(ctx context.Context, tokenHash string) (*PasswordReset, error) {
	var p PasswordReset
	err := r.DB.QueryRowContext(ctx,
		`SELECT id, customer_id, expires_at, used_at FROM customer_password_resets WHERE token_hash = $1`,
		tokenHash,
	).Scan(&p.ID, &p.CustomerID, &p.ExpiresAt, &p.UsedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// password_hash 更新と used_at マークを TX 内でアトミックに行う。
func (r *CustomerRepository) ApplyPasswordReset(ctx context.Context, customerID, resetID, passwordHash string) error {
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("repository.ApplyPasswordReset begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck
	if _, err := tx.ExecContext(ctx,
		`UPDATE customers SET password_hash = $1 WHERE id = $2`,
		passwordHash, customerID,
	); err != nil {
		return fmt.Errorf("repository.ApplyPasswordReset update customer: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE customer_password_resets SET used_at = now() WHERE id = $1`,
		resetID,
	); err != nil {
		return fmt.Errorf("repository.ApplyPasswordReset mark token: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("repository.ApplyPasswordReset commit: %w", err)
	}
	return nil
}

func (r *CustomerRepository) CreateSession(ctx context.Context, customerID, tokenHash string, expiresAt time.Time) error {
	_, err := r.DB.ExecContext(ctx,
		`INSERT INTO customer_sessions (customer_id, token_hash, expires_at) VALUES ($1, $2, $3)`,
		customerID, tokenHash, expiresAt,
	)
	if err != nil {
		return fmt.Errorf("repository.CustomerCreateSession: %w", err)
	}
	return nil
}

func (r *CustomerRepository) DeleteSessionByTokenHash(ctx context.Context, tokenHash string) error {
	_, err := r.DB.ExecContext(ctx, `DELETE FROM customer_sessions WHERE token_hash = $1`, tokenHash)
	if err != nil {
		return fmt.Errorf("repository.CustomerDeleteSessionByTokenHash: %w", err)
	}
	return nil
}

func (r *CustomerRepository) DeleteAllSessions(ctx context.Context, customerID string) error {
	_, err := r.DB.ExecContext(ctx, `DELETE FROM customer_sessions WHERE customer_id = $1`, customerID)
	if err != nil {
		return fmt.Errorf("repository.CustomerDeleteAllSessions: %w", err)
	}
	return nil
}

func (r *CustomerRepository) DeleteOtherSessions(ctx context.Context, customerID, exceptTokenHash string) error {
	_, err := r.DB.ExecContext(ctx,
		`DELETE FROM customer_sessions WHERE customer_id = $1 AND token_hash != $2`,
		customerID, exceptTokenHash,
	)
	if err != nil {
		return fmt.Errorf("repository.CustomerDeleteOtherSessions: %w", err)
	}
	return nil
}

// セッションが無い場合は sql.ErrNoRows を返す。
func (r *CustomerRepository) GetCustomerIDByTokenHash(ctx context.Context, tokenHash string) (string, error) {
	var customerID string
	err := r.DB.QueryRowContext(ctx,
		`SELECT customer_id FROM customer_sessions WHERE token_hash = $1 AND expires_at > now()`,
		tokenHash,
	).Scan(&customerID)
	return customerID, err
}

type StripeInfo struct {
	Name             string
	Email            string
	Plan             string
	StripeCustomerID sql.NullString
}

func (r *CustomerRepository) GetStripeInfo(ctx context.Context, id string) (*StripeInfo, error) {
	var info StripeInfo
	err := r.DB.QueryRowContext(ctx,
		`SELECT name, email, plan, stripe_customer_id FROM customers WHERE id = $1`,
		id,
	).Scan(&info.Name, &info.Email, &info.Plan, &info.StripeCustomerID)
	if err != nil {
		return nil, err
	}
	return &info, nil
}

func (r *CustomerRepository) GetStripeCustomerID(ctx context.Context, id string) (sql.NullString, error) {
	var s sql.NullString
	err := r.DB.QueryRowContext(ctx,
		`SELECT stripe_customer_id FROM customers WHERE id = $1`, id,
	).Scan(&s)
	return s, err
}

type SubscriptionInfo struct {
	Plan               string
	SubscriptionStatus sql.NullString
	StripeCustomerID   sql.NullString
}

func (r *CustomerRepository) GetSubscription(ctx context.Context, id string) (*SubscriptionInfo, error) {
	var s SubscriptionInfo
	err := r.DB.QueryRowContext(ctx,
		`SELECT plan, subscription_status, stripe_customer_id FROM customers WHERE id = $1`, id,
	).Scan(&s.Plan, &s.SubscriptionStatus, &s.StripeCustomerID)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *CustomerRepository) SetStripeCustomerID(ctx context.Context, id, stripeCustomerID string) error {
	_, err := r.DB.ExecContext(ctx,
		`UPDATE customers SET stripe_customer_id = $1 WHERE id = $2`,
		stripeCustomerID, id,
	)
	if err != nil {
		return fmt.Errorf("repository.SetStripeCustomerID: %w", err)
	}
	return nil
}

// checkout.session.completed を受けて呼ばれる。影響行数を返す（ログで Webhook の結果確認に使う）。
func (r *CustomerRepository) UpgradeToPro(ctx context.Context, customerID, stripeCustomerID string) (int64, error) {
	res, err := r.DB.ExecContext(ctx,
		`UPDATE customers SET plan = 'pro', subscription_status = 'active', stripe_customer_id = $1 WHERE id = $2`,
		stripeCustomerID, customerID,
	)
	if err != nil {
		return 0, fmt.Errorf("repository.UpgradeToPro: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

func (r *CustomerRepository) SetSubscriptionStatus(ctx context.Context, stripeCustomerID, status string) error {
	_, err := r.DB.ExecContext(ctx,
		`UPDATE customers SET subscription_status = $1 WHERE stripe_customer_id = $2`,
		status, stripeCustomerID,
	)
	if err != nil {
		return fmt.Errorf("repository.SetSubscriptionStatus: %w", err)
	}
	return nil
}

// customer.subscription.deleted を受けて呼ばれる。plan=free, status=canceled に戻す。
func (r *CustomerRepository) Downgrade(ctx context.Context, stripeCustomerID string) error {
	_, err := r.DB.ExecContext(ctx,
		`UPDATE customers SET plan = 'free', subscription_status = 'canceled' WHERE stripe_customer_id = $1`,
		stripeCustomerID,
	)
	if err != nil {
		return fmt.Errorf("repository.Downgrade: %w", err)
	}
	return nil
}
