package handler

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	_ "github.com/glebarez/sqlite"
	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"

	"api-billing-platform/internal/apispec"
	"api-billing-platform/internal/middleware"
)

type testDBCloser struct{ *sql.DB }

func (c testDBCloser) Close() error { return c.DB.Close() }

func setupTestDB(t *testing.T) testDBCloser {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`CREATE TABLE provided_apis (
		id TEXT PRIMARY KEY,
		provider_id TEXT,
		name TEXT,
		description TEXT,
		documentation TEXT,
		created_at TEXT
    )`)
	if err != nil {
		db.Close()
		t.Fatalf("failed to create table: %v", err)
	}
	return testDBCloser{db}
}

func TestProvidedAPIListHandler_Empty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	h := ProvidedAPIListHandler(db.DB)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/provided-apis", nil)

	h(w, r)
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var apis []apispec.ProvidedAPI
	if err := json.NewDecoder(resp.Body).Decode(&apis); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if len(apis) != 0 {
		t.Errorf("expected 0 apis, got %d", len(apis))
	}
}

func providerTestCtx(req *http.Request) *http.Request {
	const providerID = "00000000-0000-0000-0000-000000000002"
	return req.WithContext(context.WithValue(req.Context(), middleware.ContextKeyProviderID, providerID))
}

func insertTestProvidedAPI(t *testing.T, db *sql.DB, providerID, name string) string {
	t.Helper()
	var id string
	err := db.QueryRow(
		`INSERT INTO provided_apis (provider_id, name, description) VALUES ($1, $2, 'test desc') RETURNING id`,
		providerID, name,
	).Scan(&id)
	if err != nil {
		t.Fatalf("insertTestProvidedAPI: %v", err)
	}
	t.Cleanup(func() { db.Exec(`DELETE FROM provided_apis WHERE id = $1`, id) })
	return id
}

func TestProvidedAPICreateHandler_Success(t *testing.T) {
	db := testDB()
	if err := db.Ping(); err != nil {
		t.Skipf("test DB not available: %v", err)
	}

	const providerID = "00000000-0000-0000-0000-000000000002"
	h := ProvidedAPICreateHandler(db)

	body := bytes.NewBufferString(`{"name":"Test Create API","description":"A new API"}`)
	req := httptest.NewRequest(http.MethodPost, "/provided-apis", body)
	req.Header.Set("Content-Type", "application/json")
	req = providerTestCtx(req)
	w := httptest.NewRecorder()

	h(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, w.Body.String())
	}
	var api apispec.ProvidedAPI
	if err := json.NewDecoder(resp.Body).Decode(&api); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if api.Id == uuid.Nil {
		t.Error("expected non-empty ID")
	}
	if api.Name != "Test Create API" {
		t.Errorf("expected name 'Test Create API', got %q", api.Name)
	}
	if api.ProviderId == nil || *api.ProviderId != providerID {
		got := ""
		if api.ProviderId != nil {
			got = *api.ProviderId
		}
		t.Errorf("expected provider_id %s, got %s", providerID, got)
	}
	t.Cleanup(func() { db.Exec(`DELETE FROM provided_apis WHERE id = $1`, api.Id.String()) })
}

func TestProvidedAPICreateHandler_InvalidJSON(t *testing.T) {
	db := testDB()
	if err := db.Ping(); err != nil {
		t.Skipf("test DB not available: %v", err)
	}

	h := ProvidedAPICreateHandler(db)
	req := httptest.NewRequest(http.MethodPost, "/provided-apis", bytes.NewBufferString(`not-json`))
	req = providerTestCtx(req)
	w := httptest.NewRecorder()

	h(w, req)

	if w.Result().StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Result().StatusCode)
	}
}

func TestProvidedAPIUpdateHandler_Success(t *testing.T) {
	db := testDB()
	if err := db.Ping(); err != nil {
		t.Skipf("test DB not available: %v", err)
	}

	const providerID = "00000000-0000-0000-0000-000000000002"
	id := insertTestProvidedAPI(t, db, providerID, "Before Update")

	h := ProvidedAPIUpdateHandler(db)
	body := bytes.NewBufferString(`{"name":"After Update","description":"updated desc"}`)
	req := httptest.NewRequest(http.MethodPut, "/provided-apis/"+id, body)
	req.Header.Set("Content-Type", "application/json")
	req = withChiParam(providerTestCtx(req), "id", id)
	w := httptest.NewRecorder()

	h(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Result().StatusCode, w.Body.String())
	}
	var api apispec.ProvidedAPI
	if err := json.NewDecoder(w.Result().Body).Decode(&api); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if api.Name != "After Update" {
		t.Errorf("expected 'After Update', got %q", api.Name)
	}
}

func TestProvidedAPIUpdateHandler_NotFound(t *testing.T) {
	db := testDB()
	if err := db.Ping(); err != nil {
		t.Skipf("test DB not available: %v", err)
	}

	h := ProvidedAPIUpdateHandler(db)
	nonExistentID := uuid.New().String()
	body := bytes.NewBufferString(`{"name":"X","description":"Y"}`)
	req := httptest.NewRequest(http.MethodPut, "/provided-apis/"+nonExistentID, body)
	req.Header.Set("Content-Type", "application/json")
	req = withChiParam(providerTestCtx(req), "id", nonExistentID)
	w := httptest.NewRecorder()

	h(w, req)

	if w.Result().StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Result().StatusCode)
	}
}

func TestProvidedAPIDeleteHandler_Success(t *testing.T) {
	db := testDB()
	if err := db.Ping(); err != nil {
		t.Skipf("test DB not available: %v", err)
	}

	const providerID = "00000000-0000-0000-0000-000000000002"
	id := insertTestProvidedAPI(t, db, providerID, "To Delete")

	h := ProvidedAPIDeleteHandler(db)
	req := httptest.NewRequest(http.MethodDelete, "/provided-apis/"+id, nil)
	req = withChiParam(providerTestCtx(req), "id", id)
	w := httptest.NewRecorder()

	h(w, req)

	if w.Result().StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Result().StatusCode, w.Body.String())
	}
}

func TestProvidedAPIDeleteHandler_NotFound(t *testing.T) {
	db := testDB()
	if err := db.Ping(); err != nil {
		t.Skipf("test DB not available: %v", err)
	}

	h := ProvidedAPIDeleteHandler(db)
	nonExistentID := uuid.New().String()
	req := httptest.NewRequest(http.MethodDelete, "/provided-apis/"+nonExistentID, nil)
	req = withChiParam(providerTestCtx(req), "id", nonExistentID)
	w := httptest.NewRecorder()

	h(w, req)

	if w.Result().StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Result().StatusCode)
	}
}

func TestProvidedAPIProviderListHandler_ReturnsOwnAPIs(t *testing.T) {
	db := testDB()
	if err := db.Ping(); err != nil {
		t.Skipf("test DB not available: %v", err)
	}

	const providerID = "00000000-0000-0000-0000-000000000002"
	insertTestProvidedAPI(t, db, providerID, "Provider List Test API")

	h := ProvidedAPIProviderListHandler(db)
	req := httptest.NewRequest(http.MethodGet, "/provider/apis", nil)
	req = providerTestCtx(req)
	w := httptest.NewRecorder()

	h(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, w.Body.String())
	}
	var apis []apispec.ProvidedAPI
	if err := json.NewDecoder(resp.Body).Decode(&apis); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	var found bool
	for _, a := range apis {
		if a.Name == "Provider List Test API" {
			found = true
			if a.KeyCount == nil {
				t.Error("key_count should be non-nil")
			}
			if a.TotalUsage == nil {
				t.Error("total_usage should be non-nil")
			}
			break
		}
	}
	if !found {
		t.Error("inserted API not found in provider list response")
	}
}

// ON DELETE SET NULL 制約により、Provided API 削除後も顧客の api_keys は残り provided_api_id のみ NULL になる。
func TestProvidedAPIDelete_SetsAPIKeyProvidedAPIIDToNull(t *testing.T) {
	db := testDB()
	if err := db.Ping(); err != nil {
		t.Skipf("test DB not available: %v", err)
	}

	const providerID = "00000000-0000-0000-0000-000000000002"
	apiID := insertTestProvidedAPI(t, db, providerID, "ON DELETE SET NULL Test")

	var customerID string
	err := db.QueryRow(
		`INSERT INTO customers (id, name, email, password_hash, is_active) VALUES (gen_random_uuid(), 'Set Null Test', $1, 'hash', true) RETURNING id`,
		"setnull-test@example.com",
	).Scan(&customerID)
	if err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	t.Cleanup(func() { db.Exec(`DELETE FROM customers WHERE id = $1`, customerID) })

	var keyID string
	err = db.QueryRow(
		`INSERT INTO api_keys (customer_id, key_hash, provided_api_id, usage_reset_at) VALUES ($1, 'testhash-setnull', $2, now()) RETURNING id`,
		customerID, apiID,
	).Scan(&keyID)
	if err != nil {
		t.Fatalf("insert api_key: %v", err)
	}
	t.Cleanup(func() { db.Exec(`DELETE FROM api_keys WHERE id = $1`, keyID) })

	h := ProvidedAPIDeleteHandler(db)
	req := httptest.NewRequest(http.MethodDelete, "/provided-apis/"+apiID, nil)
	req = withChiParam(providerTestCtx(req), "id", apiID)
	w := httptest.NewRecorder()
	h(w, req)
	if w.Result().StatusCode != http.StatusNoContent {
		t.Fatalf("delete: expected 204, got %d", w.Result().StatusCode)
	}

	var providedAPIID sql.NullString
	err = db.QueryRow(`SELECT provided_api_id FROM api_keys WHERE id = $1`, keyID).Scan(&providedAPIID)
	if err != nil {
		t.Fatalf("select api_key: %v", err)
	}
	if providedAPIID.Valid {
		t.Errorf("expected provided_api_id to be NULL after delete, got %q", providedAPIID.String)
	}
}

func TestProvidedAPIUsageHandler_Empty(t *testing.T) {
	db := testDB()
	if err := db.Ping(); err != nil {
		t.Skipf("test DB not available: %v", err)
	}

	h := ProvidedAPIUsageHandler(db)
	req := httptest.NewRequest(http.MethodGet, "/provider/usage", nil)
	req = providerTestCtx(req)
	w := httptest.NewRecorder()
	h(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Result().StatusCode)
	}
	var result []map[string]interface{}
	json.NewDecoder(w.Body).Decode(&result)
	if len(result) != 0 {
		t.Errorf("want empty usage, got %d items", len(result))
	}
}

func TestProvidedAPIUsageHandler_ReturnsUsage(t *testing.T) {
	db := testDB()
	if err := db.Ping(); err != nil {
		t.Skipf("test DB not available: %v", err)
	}

	const providerID = "00000000-0000-0000-0000-000000000002"
	apiID := insertTestProvidedAPI(t, db, providerID, "Usage Test API")

	var customerID string
	err := db.QueryRow(
		`INSERT INTO customers (id, name, email, password_hash, is_active) VALUES (gen_random_uuid(), 'Usage Customer', $1, 'hash', true) RETURNING id`,
		"usage-customer@example.com",
	).Scan(&customerID)
	if err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	t.Cleanup(func() { db.Exec(`DELETE FROM customers WHERE id = $1`, customerID) })

	var keyID string
	err = db.QueryRow(
		`INSERT INTO api_keys (customer_id, key_hash, provided_api_id, usage_count, usage_reset_at) VALUES ($1, 'testhash-usage', $2, 42, now()) RETURNING id`,
		customerID, apiID,
	).Scan(&keyID)
	if err != nil {
		t.Fatalf("insert api_key: %v", err)
	}
	t.Cleanup(func() { db.Exec(`DELETE FROM api_keys WHERE id = $1`, keyID) })

	h := ProvidedAPIUsageHandler(db)
	req := httptest.NewRequest(http.MethodGet, "/provider/usage", nil)
	req = providerTestCtx(req)
	w := httptest.NewRecorder()
	h(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Result().StatusCode, w.Body.String())
	}
	var result []map[string]interface{}
	json.NewDecoder(w.Body).Decode(&result)
	if len(result) != 1 {
		t.Fatalf("want 1 usage row, got %d", len(result))
	}
	if result[0]["api_name"] != "Usage Test API" {
		t.Errorf("want api_name='Usage Test API', got %v", result[0]["api_name"])
	}
	if result[0]["customer_email"] != "usage-customer@example.com" {
		t.Errorf("want customer_email='usage-customer@example.com', got %v", result[0]["customer_email"])
	}
	if int(result[0]["usage_count"].(float64)) != 42 {
		t.Errorf("want usage_count=42, got %v", result[0]["usage_count"])
	}
}

func TestProvidedAPICreateHandler_EmptyName(t *testing.T) {
	db := testDB()
	if err := db.Ping(); err != nil {
		t.Skipf("test DB not available: %v", err)
	}

	h := ProvidedAPICreateHandler(db)
	body := bytes.NewBufferString(`{"name":"   ","description":"some desc"}`)
	req := httptest.NewRequest(http.MethodPost, "/provided-apis", body)
	req.Header.Set("Content-Type", "application/json")
	req = providerTestCtx(req)
	w := httptest.NewRecorder()

	h(w, req)

	if w.Result().StatusCode != http.StatusBadRequest {
		t.Errorf("want 400 for blank name, got %d", w.Result().StatusCode)
	}
}

func TestProvidedAPIUpdateHandler_EmptyName(t *testing.T) {
	db := testDB()
	if err := db.Ping(); err != nil {
		t.Skipf("test DB not available: %v", err)
	}

	const providerID = "00000000-0000-0000-0000-000000000002"
	id := insertTestProvidedAPI(t, db, providerID, "Original Name")

	h := ProvidedAPIUpdateHandler(db)
	body := bytes.NewBufferString(`{"name":"","description":"updated desc"}`)
	req := httptest.NewRequest(http.MethodPut, "/provided-apis/"+id, body)
	req.Header.Set("Content-Type", "application/json")
	req = withChiParam(providerTestCtx(req), "id", id)
	w := httptest.NewRecorder()

	h(w, req)

	if w.Result().StatusCode != http.StatusBadRequest {
		t.Errorf("want 400 for empty name, got %d", w.Result().StatusCode)
	}
}

func TestProvidedAPIListHandler_WithData(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	_, err := db.Exec(
		`INSERT INTO provided_apis (id, provider_id, name, description, created_at) VALUES (?, ?, ?, ?, ?)`,
		"id1", "provider-1", "API1", "desc1", "2026-02-28T00:00:00Z",
	)
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	h := ProvidedAPIListHandler(db.DB)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/provided-apis", nil)

	h(w, r)
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var apis []apispec.ProvidedAPI
	if err := json.NewDecoder(resp.Body).Decode(&apis); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if len(apis) != 1 {
		t.Fatalf("expected 1 api, got %d", len(apis))
	}
	if apis[0].Name != "API1" {
		t.Errorf("expected name API1, got %s", apis[0].Name)
	}
}

func TestProvidedAPIUpdateHandler_InvalidJSON(t *testing.T) {
	db := testDB()
	if err := db.Ping(); err != nil {
		t.Skipf("test DB not available: %v", err)
	}

	const providerID = "00000000-0000-0000-0000-000000000002"
	id := insertTestProvidedAPI(t, db, providerID, "Update InvalidJSON Test")

	h := ProvidedAPIUpdateHandler(db)
	req := httptest.NewRequest(http.MethodPut, "/provided-apis/"+id, bytes.NewBufferString("not-json"))
	req = withChiParam(providerTestCtx(req), "id", id)
	w := httptest.NewRecorder()

	h(w, req)

	if w.Result().StatusCode != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Result().StatusCode)
	}
}

func TestProvidedAPIUpdateHandler_WrongProvider(t *testing.T) {
	db := testDB()
	if err := db.Ping(); err != nil {
		t.Skipf("test DB not available: %v", err)
	}

	const ownerProviderID = "00000000-0000-0000-0000-000000000002"
	id := insertTestProvidedAPI(t, db, ownerProviderID, "Wrong Provider Update Test")

	h := ProvidedAPIUpdateHandler(db)
	body := bytes.NewBufferString(`{"name":"Hacked","description":"by attacker"}`)
	req := httptest.NewRequest(http.MethodPut, "/provided-apis/"+id, body)
	req.Header.Set("Content-Type", "application/json")
	req = withChiParam(req.WithContext(
		context.WithValue(req.Context(), middleware.ContextKeyProviderID, "00000000-0000-0000-0000-000000000999"),
	), "id", id)
	w := httptest.NewRecorder()

	h(w, req)

	if w.Result().StatusCode != http.StatusNotFound {
		t.Errorf("want 404 for wrong provider, got %d", w.Result().StatusCode)
	}
}

func TestProvidedAPIUsageHandler_Pagination(t *testing.T) {
	db := testDB()
	if err := db.Ping(); err != nil {
		t.Skipf("test DB not available: %v", err)
	}

	const providerID = "00000000-0000-0000-0000-000000000002"
	apiID := insertTestProvidedAPI(t, db, providerID, "Pagination Test API")

	insertCustomerWithKey := func(email, keyHash string) {
		var customerID string
		if err := db.QueryRow(
			`INSERT INTO customers (id, name, email, password_hash, is_active) VALUES (gen_random_uuid(), 'Pagination User', $1, 'hash', true) RETURNING id`,
			email,
		).Scan(&customerID); err != nil {
			t.Fatalf("insert customer: %v", err)
		}
		t.Cleanup(func() { db.Exec(`DELETE FROM customers WHERE id = $1`, customerID) })
		var keyID string
		if err := db.QueryRow(
			`INSERT INTO api_keys (customer_id, key_hash, provided_api_id, usage_reset_at) VALUES ($1, $2, $3, now()) RETURNING id`,
			customerID, keyHash, apiID,
		).Scan(&keyID); err != nil {
			t.Fatalf("insert api_key: %v", err)
		}
		t.Cleanup(func() { db.Exec(`DELETE FROM api_keys WHERE id = $1`, keyID) })
	}
	insertCustomerWithKey("pagination-a@example.com", "testhash-pg-a")
	insertCustomerWithKey("pagination-b@example.com", "testhash-pg-b")

	h := ProvidedAPIUsageHandler(db)

	req := httptest.NewRequest(http.MethodGet, "/provider/usage?limit=1&offset=0", nil)
	req = providerTestCtx(req)
	w := httptest.NewRecorder()
	h(w, req)
	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Result().StatusCode)
	}
	var result []map[string]interface{}
	json.NewDecoder(w.Body).Decode(&result)
	if len(result) != 1 {
		t.Errorf("want 1 row with limit=1, got %d", len(result))
	}

	req2 := httptest.NewRequest(http.MethodGet, "/provider/usage?limit=100&offset=2", nil)
	req2 = providerTestCtx(req2)
	w2 := httptest.NewRecorder()
	h(w2, req2)
	var result2 []map[string]interface{}
	json.NewDecoder(w2.Body).Decode(&result2)
	if len(result2) != 0 {
		t.Errorf("want 0 rows with offset=2, got %d", len(result2))
	}
}

func TestProvidedAPIDeleteHandler_WrongProvider(t *testing.T) {
	db := testDB()
	if err := db.Ping(); err != nil {
		t.Skipf("test DB not available: %v", err)
	}

	const ownerProviderID = "00000000-0000-0000-0000-000000000002"
	id := insertTestProvidedAPI(t, db, ownerProviderID, "Wrong Provider Delete Test")

	h := ProvidedAPIDeleteHandler(db)
	req := httptest.NewRequest(http.MethodDelete, "/provided-apis/"+id, nil)
	req = withChiParam(req.WithContext(
		context.WithValue(req.Context(), middleware.ContextKeyProviderID, "00000000-0000-0000-0000-000000000999"),
	), "id", id)
	w := httptest.NewRecorder()

	h(w, req)

	if w.Result().StatusCode != http.StatusNotFound {
		t.Errorf("want 404 for wrong provider, got %d", w.Result().StatusCode)
	}
}
