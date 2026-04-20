package handler_test

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"api-billing-platform/internal/handler"

	_ "github.com/glebarez/sqlite"
)

func setupAdminTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	db.Exec(`CREATE TABLE customers (id TEXT PRIMARY KEY, name TEXT, email TEXT, plan TEXT DEFAULT 'free', stripe_customer_id TEXT, subscription_status TEXT, is_active INTEGER DEFAULT 1, created_at TEXT)`)
	db.Exec(`CREATE TABLE email_logs (id TEXT PRIMARY KEY, to_email TEXT, subject TEXT, body TEXT, status TEXT DEFAULT 'success', error_message TEXT, sent_at TEXT DEFAULT CURRENT_TIMESTAMP)`)
	return db
}

func TestGetCustomers_EmptyDB(t *testing.T) {
	db := setupAdminTestDB(t)
	h := handler.NewAdminHandlerWithDB(nil, db)

	req := httptest.NewRequest(http.MethodGet, "/admin/customers", nil)
	w := httptest.NewRecorder()
	h.GetCustomers(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var result []map[string]interface{}
	json.NewDecoder(w.Body).Decode(&result)
	if len(result) != 0 {
		t.Errorf("want empty list, got %d items", len(result))
	}
}

func TestGetCustomers_ReturnsAllCustomers(t *testing.T) {
	db := setupAdminTestDB(t)
	db.Exec(`INSERT INTO customers (id, name, email, plan, is_active, created_at) VALUES
		('cust-1', 'Alice', 'alice@example.com', 'pro', 1, '2024-01-01T00:00:00Z'),
		('cust-2', 'Bob', 'bob@example.com', 'free', 1, '2024-01-02T00:00:00Z')`)

	h := handler.NewAdminHandlerWithDB(nil, db)
	req := httptest.NewRequest(http.MethodGet, "/admin/customers", nil)
	w := httptest.NewRecorder()
	h.GetCustomers(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var result []map[string]interface{}
	json.NewDecoder(w.Body).Decode(&result)
	if len(result) != 2 {
		t.Fatalf("want 2 customers, got %d", len(result))
	}
	if result[0]["email"] != "bob@example.com" {
		t.Errorf("want bob first (newer), got %v", result[0]["email"])
	}
	if result[1]["plan"] != "pro" {
		t.Errorf("want alice plan=pro, got %v", result[1]["plan"])
	}
}

func TestGetEmailLogs_Empty(t *testing.T) {
	db := setupAdminTestDB(t)
	h := handler.NewAdminHandlerWithDB(nil, db)
	req := httptest.NewRequest(http.MethodGet, "/admin/email-logs", nil)
	w := httptest.NewRecorder()
	h.GetEmailLogs(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var result []map[string]interface{}
	json.NewDecoder(w.Body).Decode(&result)
	if len(result) != 0 {
		t.Errorf("want empty, got %d items", len(result))
	}
}

func TestGetEmailLogs_ReturnsLogs(t *testing.T) {
	db := setupAdminTestDB(t)
	db.Exec(`INSERT INTO email_logs (id, to_email, subject, body, status, sent_at) VALUES
		('log-1', 'a@example.com', 'Subject A', 'body', 'success', '2026-01-02T00:00:00Z'),
		('log-2', 'b@example.com', 'Subject B', 'body', 'error',   '2026-01-01T00:00:00Z')`)

	h := handler.NewAdminHandlerWithDB(nil, db)
	req := httptest.NewRequest(http.MethodGet, "/admin/email-logs", nil)
	w := httptest.NewRecorder()
	h.GetEmailLogs(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var result []map[string]interface{}
	json.NewDecoder(w.Body).Decode(&result)
	if len(result) != 2 {
		t.Fatalf("want 2 logs, got %d", len(result))
	}
	if result[0]["to_email"] != "a@example.com" {
		t.Errorf("want newest log first, got to_email=%v", result[0]["to_email"])
	}
	if result[1]["status"] != "error" {
		t.Errorf("want status=error for second log, got %v", result[1]["status"])
	}
}

func TestGetCustomers_Pagination(t *testing.T) {
	db := setupAdminTestDB(t)
	db.Exec(`INSERT INTO customers (id, name, email, plan, is_active, created_at) VALUES
		('cust-p1', 'A', 'a@pag.com', 'free', 1, '2024-01-01'),
		('cust-p2', 'B', 'b@pag.com', 'free', 1, '2024-01-02'),
		('cust-p3', 'C', 'c@pag.com', 'pro',  1, '2024-01-03')`)

	h := handler.NewAdminHandlerWithDB(nil, db)

	req := httptest.NewRequest(http.MethodGet, "/admin/customers?limit=1&offset=0", nil)
	w := httptest.NewRecorder()
	h.GetCustomers(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var r1 []map[string]interface{}
	json.NewDecoder(w.Body).Decode(&r1)
	if len(r1) != 1 {
		t.Fatalf("want 1 customer with limit=1, got %d", len(r1))
	}
	if r1[0]["email"] != "c@pag.com" {
		t.Errorf("want newest first, got %v", r1[0]["email"])
	}

	req2 := httptest.NewRequest(http.MethodGet, "/admin/customers?limit=100&offset=2", nil)
	w2 := httptest.NewRecorder()
	h.GetCustomers(w2, req2)
	var r2 []map[string]interface{}
	json.NewDecoder(w2.Body).Decode(&r2)
	if len(r2) != 1 {
		t.Fatalf("want 1 customer with offset=2, got %d", len(r2))
	}
	if r2[0]["email"] != "a@pag.com" {
		t.Errorf("want oldest at offset=2, got %v", r2[0]["email"])
	}
}

func TestGetCustomers_NoDB(t *testing.T) {
	h := handler.NewAdminHandlerWithDB(nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/admin/customers", nil)
	w := httptest.NewRecorder()
	h.GetCustomers(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", w.Code)
	}
}

func TestGetEmailLogs_NoDB(t *testing.T) {
	h := handler.NewAdminHandlerWithDB(nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/admin/email-logs", nil)
	w := httptest.NewRecorder()
	h.GetEmailLogs(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", w.Code)
	}
}

func TestGetCustomers_PlanBadge(t *testing.T) {
	db := setupAdminTestDB(t)
	db.Exec(`INSERT INTO customers (id, name, email, plan, is_active, created_at) VALUES ('cust-1', 'Alice', 'alice@example.com', 'pro', 1, '2024-01-01')`)

	h := handler.NewAdminHandlerWithDB(nil, db)
	req := httptest.NewRequest(http.MethodGet, "/admin/customers", nil)
	w := httptest.NewRecorder()
	h.GetCustomers(w, req)

	var result []map[string]interface{}
	json.NewDecoder(w.Body).Decode(&result)
	if len(result) != 1 {
		t.Fatalf("want 1 customer, got %d", len(result))
	}
	if result[0]["plan"] != "pro" {
		t.Errorf("want plan=pro, got %v", result[0]["plan"])
	}
	if result[0]["name"] != "Alice" {
		t.Errorf("want name=Alice, got %v", result[0]["name"])
	}
}
