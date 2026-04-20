package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"os"
	"strconv"
	"strings"
	"time"

	"api-billing-platform/internal/model"
)

var ErrRateLimited = errors.New("rate limit exceeded")
var ErrAPIKeyLimitExceeded = errors.New("api key limit exceeded")

// PLAN_FREE_MONTHLY_LIMIT / PLAN_PRO_MONTHLY_LIMIT で上書き可能。
var planMonthlyLimit = map[string]int{
	"free": 100,
	"pro":  10000,
}

// PLAN_FREE_KEY_LIMIT / PLAN_PRO_KEY_LIMIT で上書き可能。
var planAPIKeyLimit = map[string]int{
	"free": 3,
	"pro":  20,
}

func init() {
	if v, err := strconv.Atoi(os.Getenv("PLAN_FREE_MONTHLY_LIMIT")); err == nil && v > 0 {
		planMonthlyLimit["free"] = v
	}
	if v, err := strconv.Atoi(os.Getenv("PLAN_PRO_MONTHLY_LIMIT")); err == nil && v > 0 {
		planMonthlyLimit["pro"] = v
	}
	if v, err := strconv.Atoi(os.Getenv("PLAN_FREE_KEY_LIMIT")); err == nil && v > 0 {
		planAPIKeyLimit["free"] = v
	}
	if v, err := strconv.Atoi(os.Getenv("PLAN_PRO_KEY_LIMIT")); err == nil && v > 0 {
		planAPIKeyLimit["pro"] = v
	}
}

type APIKeyRepository struct {
	db *sql.DB
}

func NewAPIKeyRepository(db *sql.DB) *APIKeyRepository {
	return &APIKeyRepository{db: db}
}

func hashKey(plainKey string) string {
	sum := sha256.Sum256([]byte(plainKey))
	return hex.EncodeToString(sum[:])
}

func (r *APIKeyRepository) Create(ctx context.Context, customerID, plainKey string, providedAPIID *string) (*model.APIKey, error) {
	var plan string
	var keyCount int
	err := r.db.QueryRowContext(ctx,
		`SELECT c.plan, COUNT(ak.id) FROM customers c LEFT JOIN api_keys ak ON ak.customer_id = c.id WHERE c.id = $1 GROUP BY c.plan`,
		customerID,
	).Scan(&plan, &keyCount)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	limit, ok := planAPIKeyLimit[plan]
	if !ok {
		limit = planAPIKeyLimit["free"]
	}
	if keyCount >= limit {
		return nil, ErrAPIKeyLimitExceeded
	}

	keyHash := hashKey(plainKey)
	row := r.db.QueryRowContext(ctx,
		`INSERT INTO api_keys (customer_id, key_hash, provided_api_id, usage_reset_at) VALUES ($1, $2, $3, $4) RETURNING id, customer_id, provided_api_id, usage_count, usage_reset_at, created_at`,
		customerID, keyHash, providedAPIID, time.Now())
	var k model.APIKey
	var pid sql.NullString
	if err := row.Scan(&k.ID, &k.CustomerID, &pid, &k.UsageCount, &k.UsageResetAt, &k.CreatedAt); err != nil {
		return nil, err
	}
	if pid.Valid {
		k.ProvidedAPIID = &pid.String
	}
	return &k, nil
}

func (r *APIKeyRepository) GetByID(ctx context.Context, id string) (*model.APIKey, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, customer_id, provided_api_id, usage_count, usage_reset_at, created_at FROM api_keys WHERE id = $1`, id)
	var k model.APIKey
	var pid sql.NullString
	if err := row.Scan(&k.ID, &k.CustomerID, &pid, &k.UsageCount, &k.UsageResetAt, &k.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if pid.Valid {
		k.ProvidedAPIID = &pid.String
	}
	return &k, nil
}

func (r *APIKeyRepository) GetByPlainKey(ctx context.Context, plainKey string) (*model.APIKey, error) {
	keyHash := hashKey(plainKey)
	row := r.db.QueryRowContext(ctx,
		`SELECT id, customer_id, provided_api_id, usage_count, usage_reset_at, created_at FROM api_keys WHERE key_hash = $1`, keyHash)
	var k model.APIKey
	var pid sql.NullString
	if err := row.Scan(&k.ID, &k.CustomerID, &pid, &k.UsageCount, &k.UsageResetAt, &k.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if pid.Valid {
		k.ProvidedAPIID = &pid.String
	}
	return &k, nil
}

func (r *APIKeyRepository) Delete(ctx context.Context, id, customerID string) error {
	result, err := r.db.ExecContext(ctx,
		`DELETE FROM api_keys WHERE id = $1 AND customer_id = $2`,
		id, customerID,
	)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *APIKeyRepository) IncrementUsage(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE api_keys SET usage_count = usage_count + 1 WHERE id = $1`, id)
	return err
}

func (r *APIKeyRepository) ResetUsage(ctx context.Context, id string, resetAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE api_keys SET usage_count = 0, usage_reset_at = $2 WHERE id = $1`, id, resetAt)
	return err
}

func (r *APIKeyRepository) GetCustomerByAPIKey(ctx context.Context, apiKeyID string) (*model.Customer, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT c.id, c.name, c.email, c.plan, c.stripe_customer_id, c.subscription_status, c.created_at
		 FROM customers c JOIN api_keys k ON c.id = k.customer_id WHERE k.id = $1`, apiKeyID)
	var c model.Customer
	var stripeCustomerID, subscriptionStatus sql.NullString
	if err := row.Scan(&c.ID, &c.Name, &c.Email, &c.Plan, &stripeCustomerID, &subscriptionStatus, &c.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if stripeCustomerID.Valid {
		c.StripeCustomerID = &stripeCustomerID.String
	}
	if subscriptionStatus.Valid {
		c.SubscriptionStatus = &subscriptionStatus.String
	}
	return &c, nil
}

type VerifyResult struct {
	ID         string
	CustomerID string
	UsageCount int
}

func (r *APIKeyRepository) resetExpiredKeys(ctx context.Context, customerID string) error {
	startOfMonth := time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 0, -time.Now().UTC().Day()+1)
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx,
		`UPDATE api_keys SET usage_count = 0, usage_reset_at = $1
		 WHERE customer_id = $2 AND usage_reset_at < $3`,
		now, customerID, startOfMonth,
	)
	return err
}

// キーが存在しない・権限外は (nil, nil)、月間合計がプラン上限超は ErrRateLimited を返す。
func (r *APIKeyRepository) VerifyForProvider(ctx context.Context, plainKey, providedAPIID, providerID string) (*VerifyResult, error) {
	keyHash := hashKey(plainKey)
	var res VerifyResult
	var customerPlan string
	err := r.db.QueryRowContext(ctx,
		`SELECT ak.id, ak.customer_id, ak.usage_count, c.plan
		 FROM api_keys ak
		 JOIN provided_apis pa ON ak.provided_api_id = pa.id
		 JOIN customers c ON ak.customer_id = c.id
		 WHERE ak.key_hash = $1
		   AND ak.provided_api_id = $2
		   AND pa.provider_id = $3`,
		keyHash, providedAPIID, providerID,
	).Scan(&res.ID, &res.CustomerID, &res.UsageCount, &customerPlan)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		if strings.Contains(err.Error(), "invalid input syntax") {
			return nil, nil
		}
		return nil, err
	}

	if err := r.resetExpiredKeys(ctx, res.CustomerID); err != nil {
		return nil, err
	}

	limit, ok := planMonthlyLimit[customerPlan]
	if !ok {
		limit = planMonthlyLimit["free"]
	}

	// 月間合計チェックとインクリメントをアトミックに行う（TOCTOU競合を防止）
	// CTEで現在の月間合計を取得し、上限未満のときのみ usage_count を +1 する。
	err = r.db.QueryRowContext(ctx,
		`WITH monthly_total AS (
		   SELECT COALESCE(SUM(usage_count), 0) AS total
		   FROM api_keys
		   WHERE customer_id = $1
		 )
		 UPDATE api_keys
		 SET usage_count = usage_count + 1
		 WHERE id = $2
		   AND (SELECT total FROM monthly_total) < $3
		 RETURNING usage_count`,
		res.CustomerID, res.ID, limit,
	).Scan(&res.UsageCount)
	if errors.Is(err, sql.ErrNoRows) {
		// UPDATE が条件を満たさず 0 行 → 月間上限に達している
		return nil, ErrRateLimited
	}
	if err != nil {
		return nil, err
	}
	return &res, nil
}

type APIKeyRepositoryInterface interface {
	Create(ctx context.Context, customerID, plainKey string, providedAPIID *string) (*model.APIKey, error)
	GetByID(ctx context.Context, id string) (*model.APIKey, error)
	GetByPlainKey(ctx context.Context, plainKey string) (*model.APIKey, error)
	Delete(ctx context.Context, id, customerID string) error
	IncrementUsage(ctx context.Context, id string) error
	ResetUsage(ctx context.Context, id string, resetAt time.Time) error
	GetCustomerByAPIKey(ctx context.Context, apiKeyID string) (*model.Customer, error)
	VerifyForProvider(ctx context.Context, plainKey, providedAPIID, providerID string) (*VerifyResult, error)
}
