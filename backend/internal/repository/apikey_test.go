package repository_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"api-billing-platform/internal/repository"
)

func insertCustomer(t *testing.T, db *sql.DB, suffix string) string {
	t.Helper()
	var id string
	err := db.QueryRow(
		`INSERT INTO customers (id, name, email, password_hash) VALUES (gen_random_uuid(), $1, $2, 'hash') RETURNING id`,
		"Test Customer "+suffix,
		fmt.Sprintf("test-%s@example.com", suffix),
	).Scan(&id)
	if err != nil {
		t.Fatalf("failed to insert test customer: %v", err)
	}
	return id
}

func insertCustomerWithPlan(t *testing.T, db *sql.DB, suffix, plan string) string {
	t.Helper()
	var id string
	err := db.QueryRow(
		`INSERT INTO customers (id, name, email, password_hash, plan) VALUES (gen_random_uuid(), $1, $2, 'hash', $3) RETURNING id`,
		"Test Customer "+suffix,
		fmt.Sprintf("test-%s@example.com", suffix),
		plan,
	).Scan(&id)
	if err != nil {
		t.Fatalf("failed to insert test customer: %v", err)
	}
	return id
}

func insertProvidedAPI(t *testing.T, db *sql.DB, suffix string) (apiID, providerID string) {
	t.Helper()
	providerID = "00000000-0000-0000-0000-000000000002" // test_seed.sql のシード
	err := db.QueryRow(
		`INSERT INTO provided_apis (provider_id, name, description) VALUES ($1, $2, $3) RETURNING id`,
		providerID,
		"Test API "+suffix,
		"desc",
	).Scan(&apiID)
	if err != nil {
		t.Fatalf("failed to insert provided_api: %v", err)
	}
	return apiID, providerID
}

func setUsageCount(t *testing.T, db *sql.DB, keyID string, count int) {
	t.Helper()
	_, err := db.Exec(`UPDATE api_keys SET usage_count = $1 WHERE id = $2`, count, keyID)
	if err != nil {
		t.Fatalf("failed to set usage_count: %v", err)
	}
}

func TestAPIKeyRepository_Create(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	keyRepo := repository.NewAPIKeyRepository(db)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	customerID := insertCustomer(t, db, time.Now().Format("20060102150405.000"))

	key, err := keyRepo.Create(ctx, customerID, "testplainkey-create-"+time.Now().Format("20060102150405.000"), nil)
	if err != nil {
		t.Fatalf("failed to create key: %v", err)
	}

	if key == nil {
		t.Fatal("key should not be nil")
	}
	if key.CustomerID != customerID {
		t.Errorf("customer ID mismatch: %s vs %s", customerID, key.CustomerID)
	}
	if key.UsageCount != 0 {
		t.Errorf("expected usage 0, got %d", key.UsageCount)
	}
}

func TestAPIKeyRepository_GetByID(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	keyRepo := repository.NewAPIKeyRepository(db)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	customerID := insertCustomer(t, db, "getbyid-"+time.Now().Format("20060102150405.000"))
	created, err := keyRepo.Create(ctx, customerID, "testplainkey-getbyid-"+time.Now().Format("20060102150405.000"), nil)
	if err != nil {
		t.Fatalf("failed to create key: %v", err)
	}

	retrieved, err := keyRepo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("failed to get: %v", err)
	}

	if retrieved == nil {
		t.Fatal("retrieved key should not be nil")
	}
	if retrieved.ID != created.ID {
		t.Errorf("ID mismatch: %s vs %s", created.ID, retrieved.ID)
	}
}

func TestAPIKeyRepository_GetByPlainKey(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	keyRepo := repository.NewAPIKeyRepository(db)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	customerID := insertCustomer(t, db, "getbyplain-"+time.Now().Format("20060102150405.000"))
	plainKey := "my-plain-api-key-" + time.Now().Format("20060102150405.000")
	created, err := keyRepo.Create(ctx, customerID, plainKey, nil)
	if err != nil {
		t.Fatalf("failed to create key: %v", err)
	}

	retrieved, err := keyRepo.GetByPlainKey(ctx, plainKey)
	if err != nil {
		t.Fatalf("failed to get by plain key: %v", err)
	}

	if retrieved == nil {
		t.Fatal("retrieved key should not be nil")
	}
	if retrieved.ID != created.ID {
		t.Errorf("ID mismatch: %s vs %s", created.ID, retrieved.ID)
	}
}

func TestAPIKeyRepository_IncrementUsage(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	keyRepo := repository.NewAPIKeyRepository(db)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	customerID := insertCustomer(t, db, "usage-"+time.Now().Format("20060102150405.000"))
	created, err := keyRepo.Create(ctx, customerID, "testplainkey-"+time.Now().Format("20060102150405.000"), nil)
	if err != nil {
		t.Fatalf("failed to create key: %v", err)
	}

	for i := 0; i < 3; i++ {
		err := keyRepo.IncrementUsage(ctx, created.ID)
		if err != nil {
			t.Fatalf("failed to increment: %v", err)
		}
	}

	updated, err := keyRepo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("failed to get by id: %v", err)
	}
	if updated.UsageCount != 3 {
		t.Errorf("expected usage 3, got %d", updated.UsageCount)
	}
}

func TestAPIKeyRepository_ResetUsage(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	keyRepo := repository.NewAPIKeyRepository(db)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	customerID := insertCustomer(t, db, "reset-"+time.Now().Format("20060102150405.000"))
	created, err := keyRepo.Create(ctx, customerID, "testplainkey-reset-"+time.Now().Format("20060102150405.000"), nil)
	if err != nil {
		t.Fatalf("failed to create key: %v", err)
	}
	keyRepo.IncrementUsage(ctx, created.ID)
	keyRepo.IncrementUsage(ctx, created.ID)

	resetTime := time.Now()
	err = keyRepo.ResetUsage(ctx, created.ID, resetTime)
	if err != nil {
		t.Fatalf("failed to reset: %v", err)
	}

	updated, err := keyRepo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("failed to get by id: %v", err)
	}
	if updated.UsageCount != 0 {
		t.Errorf("expected usage 0 after reset, got %d", updated.UsageCount)
	}
}

func TestAPIKeyRepository_GetCustomerByAPIKey(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	keyRepo := repository.NewAPIKeyRepository(db)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	customerID := insertCustomer(t, db, "getcust-"+time.Now().Format("20060102150405.000"))
	key, err := keyRepo.Create(ctx, customerID, "testplainkey-getcust-"+time.Now().Format("20060102150405.000"), nil)
	if err != nil {
		t.Fatalf("failed to create key: %v", err)
	}

	retrieved, err := keyRepo.GetCustomerByAPIKey(ctx, key.ID)
	if err != nil {
		t.Fatalf("failed to get customer: %v", err)
	}

	if retrieved == nil {
		t.Fatal("retrieved customer should not be nil")
	}
	if retrieved.ID != customerID {
		t.Errorf("ID mismatch: %s vs %s", customerID, retrieved.ID)
	}
}

func TestVerifyForProvider_FreePlan_AtLimit(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	ctx := context.Background()
	keyRepo := repository.NewAPIKeyRepository(db)
	suffix := "free-at-" + time.Now().Format("20060102150405.000")

	customerID := insertCustomerWithPlan(t, db, suffix, "free")
	apiID, providerID := insertProvidedAPI(t, db, suffix)
	key, _ := keyRepo.Create(ctx, customerID, "plain-"+suffix, &apiID)

	setUsageCount(t, db, key.ID, 99)

	result, err := keyRepo.VerifyForProvider(ctx, "plain-"+suffix, apiID, providerID)
	if err != nil {
		t.Fatalf("100回目(上限ぴったり)でエラーが発生: %v", err)
	}
	if result == nil {
		t.Fatal("100回目(上限ぴったり)でnil: 成功すべき")
	}
	if result.UsageCount != 100 {
		t.Errorf("usage_count=100 を期待, got %d", result.UsageCount)
	}
}

func TestVerifyForProvider_FreePlan_OverLimit(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	ctx := context.Background()
	keyRepo := repository.NewAPIKeyRepository(db)
	suffix := "free-over-" + time.Now().Format("20060102150405.000")

	customerID := insertCustomerWithPlan(t, db, suffix, "free")
	apiID, providerID := insertProvidedAPI(t, db, suffix)
	key, _ := keyRepo.Create(ctx, customerID, "plain-"+suffix, &apiID)

	setUsageCount(t, db, key.ID, 100)

	result, err := keyRepo.VerifyForProvider(ctx, "plain-"+suffix, apiID, providerID)
	if !errors.Is(err, repository.ErrRateLimited) {
		t.Errorf("101回目(上限超過): ErrRateLimited を期待, got err=%v result=%v", err, result)
	}
	if result != nil {
		t.Errorf("上限超過時は result=nil を期待, got %+v", result)
	}
}

func TestVerifyForProvider_ProPlan_AtLimit(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	ctx := context.Background()
	keyRepo := repository.NewAPIKeyRepository(db)
	suffix := "pro-at-" + time.Now().Format("20060102150405.000")

	customerID := insertCustomerWithPlan(t, db, suffix, "pro")
	apiID, providerID := insertProvidedAPI(t, db, suffix)
	key, _ := keyRepo.Create(ctx, customerID, "plain-"+suffix, &apiID)

	setUsageCount(t, db, key.ID, 9999)

	result, err := keyRepo.VerifyForProvider(ctx, "plain-"+suffix, apiID, providerID)
	if err != nil {
		t.Fatalf("10000回目(上限ぴったり)でエラーが発生: %v", err)
	}
	if result == nil {
		t.Fatal("10000回目(上限ぴったり)でnil: 成功すべき")
	}
	if result.UsageCount != 10000 {
		t.Errorf("usage_count=10000 を期待, got %d", result.UsageCount)
	}
}

func TestVerifyForProvider_ProPlan_OverLimit(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	ctx := context.Background()
	keyRepo := repository.NewAPIKeyRepository(db)
	suffix := "pro-over-" + time.Now().Format("20060102150405.000")

	customerID := insertCustomerWithPlan(t, db, suffix, "pro")
	apiID, providerID := insertProvidedAPI(t, db, suffix)
	key, _ := keyRepo.Create(ctx, customerID, "plain-"+suffix, &apiID)

	setUsageCount(t, db, key.ID, 10000)

	result, err := keyRepo.VerifyForProvider(ctx, "plain-"+suffix, apiID, providerID)
	if !errors.Is(err, repository.ErrRateLimited) {
		t.Errorf("10001回目(上限超過): ErrRateLimited を期待, got err=%v result=%v", err, result)
	}
	if result != nil {
		t.Errorf("上限超過時は result=nil を期待, got %+v", result)
	}
}

func TestVerifyForProvider_MultipleKeys_UsageSummedPerCustomer(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	ctx := context.Background()
	keyRepo := repository.NewAPIKeyRepository(db)
	suffix := "multi-" + time.Now().Format("20060102150405.000")

	customerID := insertCustomerWithPlan(t, db, suffix, "free")
	apiID, providerID := insertProvidedAPI(t, db, suffix)

	key1, _ := keyRepo.Create(ctx, customerID, "plain-1-"+suffix, &apiID)
	setUsageCount(t, db, key1.ID, 60)

	key2, _ := keyRepo.Create(ctx, customerID, "plain-2-"+suffix, &apiID)
	setUsageCount(t, db, key2.ID, 39)

	result, err := keyRepo.VerifyForProvider(ctx, "plain-2-"+suffix, apiID, providerID)
	if err != nil {
		t.Fatalf("合計100回目で失敗: %v", err)
	}
	if result == nil {
		t.Fatal("合計100回目はnilでなく成功すべき")
	}

	result, err = keyRepo.VerifyForProvider(ctx, "plain-2-"+suffix, apiID, providerID)
	if !errors.Is(err, repository.ErrRateLimited) {
		t.Errorf("合計101回目: ErrRateLimited を期待, got err=%v result=%v", err, result)
	}
}

func TestAPIKeyRepository_Delete_Success(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	keyRepo := repository.NewAPIKeyRepository(db)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	customerID := insertCustomer(t, db, "del-ok-"+time.Now().Format("20060102150405.000"))
	created, err := keyRepo.Create(ctx, customerID, "plainkey-del-"+time.Now().Format("20060102150405.000"), nil)
	if err != nil {
		t.Fatalf("failed to create key: %v", err)
	}

	if err := keyRepo.Delete(ctx, created.ID, customerID); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	got, err := keyRepo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID after delete failed: %v", err)
	}
	if got != nil {
		t.Error("key should not exist after delete")
	}
}

func TestAPIKeyRepository_Delete_NotFound(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	keyRepo := repository.NewAPIKeyRepository(db)
	ctx := context.Background()

	err := keyRepo.Delete(ctx, "00000000-0000-0000-0000-000000000000", "00000000-0000-0000-0000-000000000001")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("expected sql.ErrNoRows, got: %v", err)
	}
}

func TestAPIKeyRepository_Delete_WrongCustomer(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	keyRepo := repository.NewAPIKeyRepository(db)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ownerID := insertCustomer(t, db, "del-owner-"+time.Now().Format("20060102150405.000"))
	attackerID := insertCustomer(t, db, "del-attacker-"+time.Now().Format("20060102150405.000"))
	created, err := keyRepo.Create(ctx, ownerID, "plainkey-del-wrong-"+time.Now().Format("20060102150405.000"), nil)
	if err != nil {
		t.Fatalf("failed to create key: %v", err)
	}

	err = keyRepo.Delete(ctx, created.ID, attackerID)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("expected sql.ErrNoRows for wrong customer, got: %v", err)
	}
}

func TestAPIKeyRepository_GetByID_NotFound(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	keyRepo := repository.NewAPIKeyRepository(db)
	ctx := context.Background()

	got, err := keyRepo.GetByID(ctx, "00000000-0000-0000-0000-000000000000")
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil, got: %+v", got)
	}
}

func TestAPIKeyRepository_GetByPlainKey_NotFound(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	keyRepo := repository.NewAPIKeyRepository(db)
	ctx := context.Background()

	got, err := keyRepo.GetByPlainKey(ctx, "nonexistent-key-"+time.Now().Format("20060102150405.000"))
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil, got: %+v", got)
	}
}

func TestVerifyForProvider_WrongPlainKey_ReturnsNil(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	ctx := context.Background()
	keyRepo := repository.NewAPIKeyRepository(db)
	suffix := "wrong-key-" + time.Now().Format("20060102150405.000")

	customerID := insertCustomerWithPlan(t, db, suffix, "free")
	apiID, providerID := insertProvidedAPI(t, db, suffix)
	keyRepo.Create(ctx, customerID, "correct-plain-"+suffix, &apiID)

	result, err := keyRepo.VerifyForProvider(ctx, "wrong-plain-key", apiID, providerID)
	if err != nil {
		t.Fatalf("expected nil error for wrong key, got: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil for wrong key, got: %+v", result)
	}
}

func TestVerifyForProvider_WrongProviderID_ReturnsNil(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	ctx := context.Background()
	keyRepo := repository.NewAPIKeyRepository(db)
	suffix := "wrong-provider-" + time.Now().Format("20060102150405.000")

	customerID := insertCustomerWithPlan(t, db, suffix, "free")
	apiID, _ := insertProvidedAPI(t, db, suffix)
	plainKey := "plain-wp-" + suffix
	keyRepo.Create(ctx, customerID, plainKey, &apiID)

	result, err := keyRepo.VerifyForProvider(ctx, plainKey, apiID, "00000000-0000-0000-0000-000000000999")
	if err != nil {
		t.Fatalf("expected nil error for wrong provider, got: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil for wrong provider, got: %+v", result)
	}
}

func TestVerifyForProvider_MonthlyReset(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	ctx := context.Background()
	keyRepo := repository.NewAPIKeyRepository(db)
	suffix := "monthly-" + time.Now().Format("20060102150405.000")

	customerID := insertCustomerWithPlan(t, db, suffix, "free")
	apiID, providerID := insertProvidedAPI(t, db, suffix)
	key, _ := keyRepo.Create(ctx, customerID, "plain-"+suffix, &apiID)

	setUsageCount(t, db, key.ID, 100)
	_, err := db.Exec(`UPDATE api_keys SET usage_reset_at = now() - INTERVAL '2 months' WHERE id = $1`, key.ID)
	if err != nil {
		t.Fatalf("failed to set old usage_reset_at: %v", err)
	}

	result, err := keyRepo.VerifyForProvider(ctx, "plain-"+suffix, apiID, providerID)
	if err != nil {
		t.Fatalf("月次リセット後は成功すべき: %v", err)
	}
	if result == nil {
		t.Fatal("月次リセット後はnilでなく成功すべき")
	}
	if result.UsageCount != 1 {
		t.Errorf("リセット後は usage_count=1 を期待, got %d", result.UsageCount)
	}
}

func TestAPIKeyRepository_Create_FreePlanLimit(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	ctx := context.Background()
	keyRepo := repository.NewAPIKeyRepository(db)
	suffix := "keylimit-" + time.Now().Format("20060102150405.000")

	customerID := insertCustomerWithPlan(t, db, suffix, "free")

	for i := 0; i < 3; i++ {
		_, err := keyRepo.Create(ctx, customerID, fmt.Sprintf("plain-%s-%d", suffix, i), nil)
		if err != nil {
			t.Fatalf("key %d: unexpected error: %v", i, err)
		}
	}

	_, err := keyRepo.Create(ctx, customerID, "plain-"+suffix+"-extra", nil)
	if !errors.Is(err, repository.ErrAPIKeyLimitExceeded) {
		t.Errorf("want ErrAPIKeyLimitExceeded, got %v", err)
	}
}

func TestAPIKeyRepository_Create_ProPlanAllowsMore(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	ctx := context.Background()
	keyRepo := repository.NewAPIKeyRepository(db)
	suffix := "proplanlimit-" + time.Now().Format("20060102150405.000")

	customerID := insertCustomerWithPlan(t, db, suffix, "pro")

	for i := 0; i < 5; i++ {
		_, err := keyRepo.Create(ctx, customerID, fmt.Sprintf("plain-%s-%d", suffix, i), nil)
		if err != nil {
			t.Fatalf("pro plan key %d: unexpected error: %v", i, err)
		}
	}
}
