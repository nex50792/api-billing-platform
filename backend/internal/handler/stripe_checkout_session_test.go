package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stripe/stripe-go/v85"
)

func insertCheckoutTestCustomerAndSession(t *testing.T, db *sql.DB, email, plan string) (customerID, plainToken string) {
	t.Helper()
	if err := db.QueryRow(
		`INSERT INTO customers (id, name, email, password_hash, is_active, plan) VALUES (gen_random_uuid(), 'Test User', $1, 'hash', true, $2) RETURNING id`,
		email, plan,
	).Scan(&customerID); err != nil {
		t.Fatalf("failed to insert customer: %v", err)
	}
	var err error
	plainToken, err = generateSessionToken()
	if err != nil {
		t.Fatalf("failed to generate session token: %v", err)
	}
	tokenHash := hashSessionToken(plainToken)
	if _, err := db.Exec(
		`INSERT INTO customer_sessions (customer_id, token_hash, expires_at) VALUES ($1, $2, $3)`,
		customerID, tokenHash, time.Now().Add(24*time.Hour),
	); err != nil {
		t.Fatalf("failed to insert session: %v", err)
	}
	return
}

func TestGetSubscription_NoSession(t *testing.T) {
	db := testDB()
	if err := db.Ping(); err != nil {
		t.Skipf("test DB not available: %v", err)
	}

	h := NewStripeCheckoutHandler(db)
	req := httptest.NewRequest(http.MethodGet, "/customer/subscription", nil)
	w := httptest.NewRecorder()

	h.GetSubscription(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401 when no session, got %d", w.Code)
	}
}

func TestCreateCheckoutSession_NoSession(t *testing.T) {
	db := testDB()
	if err := db.Ping(); err != nil {
		t.Skipf("test DB not available: %v", err)
	}

	h := NewStripeCheckoutHandler(db)
	req := httptest.NewRequest(http.MethodPost, "/customer/checkout", nil)
	w := httptest.NewRecorder()

	h.CreateCheckoutSession(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401 when no session, got %d", w.Code)
	}
}

func TestCreateBillingPortalSession_StripeNotConfigured(t *testing.T) {
	db := testDB()
	if err := db.Ping(); err != nil {
		t.Skipf("test DB not available: %v", err)
	}

	customerID, plainToken := insertCheckoutTestCustomerAndSession(t, db, "billing-portal-no-key@example.com", "pro")
	db.Exec(`UPDATE customers SET stripe_customer_id = 'cus_bp_test_nokey' WHERE id = $1`, customerID)
	t.Cleanup(func() {
		db.Exec(`DELETE FROM customer_sessions WHERE customer_id = $1`, customerID)
		db.Exec(`DELETE FROM customers WHERE id = $1`, customerID)
	})

	origKey := stripe.Key
	stripe.Key = ""
	defer func() { stripe.Key = origKey }()

	h := NewStripeCheckoutHandler(db)
	req := httptest.NewRequest(http.MethodPost, "/customer/billing-portal", nil)
	req.AddCookie(&http.Cookie{Name: "customer_session_token", Value: plainToken})
	w := httptest.NewRecorder()

	h.CreateBillingPortalSession(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500 when Stripe not configured, got %d", w.Code)
	}
}

func TestCreateCheckoutSession_AlreadyPro(t *testing.T) {
	db := testDB()
	if err := db.Ping(); err != nil {
		t.Skipf("test DB not available: %v", err)
	}

	customerID, plainToken := insertCheckoutTestCustomerAndSession(t, db, "checkout-already-pro@example.com", "pro")
	t.Cleanup(func() {
		db.Exec(`DELETE FROM customer_sessions WHERE customer_id = $1`, customerID)
		db.Exec(`DELETE FROM customers WHERE id = $1`, customerID)
	})

	h := NewStripeCheckoutHandler(db)
	req := httptest.NewRequest(http.MethodPost, "/customer/checkout", nil)
	req.AddCookie(&http.Cookie{Name: "customer_session_token", Value: plainToken})
	w := httptest.NewRecorder()

	h.CreateCheckoutSession(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("want 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateCheckoutSession_StripeNotConfigured(t *testing.T) {
	db := testDB()
	if err := db.Ping(); err != nil {
		t.Skipf("test DB not available: %v", err)
	}

	origKey := stripe.Key
	stripe.Key = ""
	defer func() { stripe.Key = origKey }()

	customerID, plainToken := insertCheckoutTestCustomerAndSession(t, db, "checkout-no-stripe@example.com", "free")
	t.Cleanup(func() {
		db.Exec(`DELETE FROM customer_sessions WHERE customer_id = $1`, customerID)
		db.Exec(`DELETE FROM customers WHERE id = $1`, customerID)
	})

	h := NewStripeCheckoutHandler(db)
	req := httptest.NewRequest(http.MethodPost, "/customer/checkout", nil)
	req.AddCookie(&http.Cookie{Name: "customer_session_token", Value: plainToken})
	w := httptest.NewRecorder()

	h.CreateCheckoutSession(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateBillingPortalSession_NoStripeCustomer(t *testing.T) {
	db := testDB()
	if err := db.Ping(); err != nil {
		t.Skipf("test DB not available: %v", err)
	}

	customerID, plainToken := insertCheckoutTestCustomerAndSession(t, db, "billing-portal-no-stripe@example.com", "pro")
	t.Cleanup(func() {
		db.Exec(`DELETE FROM customer_sessions WHERE customer_id = $1`, customerID)
		db.Exec(`DELETE FROM customers WHERE id = $1`, customerID)
	})

	h := NewStripeCheckoutHandler(db)
	req := httptest.NewRequest(http.MethodPost, "/customer/billing-portal", nil)
	req.AddCookie(&http.Cookie{Name: "customer_session_token", Value: plainToken})
	w := httptest.NewRecorder()

	h.CreateBillingPortalSession(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetSubscription_Success(t *testing.T) {
	db := testDB()
	if err := db.Ping(); err != nil {
		t.Skipf("test DB not available: %v", err)
	}

	customerID, plainToken := insertCheckoutTestCustomerAndSession(t, db, "get-subscription@example.com", "pro")
	db.Exec(`UPDATE customers SET subscription_status = 'active' WHERE id = $1`, customerID)
	t.Cleanup(func() {
		db.Exec(`DELETE FROM customer_sessions WHERE customer_id = $1`, customerID)
		db.Exec(`DELETE FROM customers WHERE id = $1`, customerID)
	})

	h := NewStripeCheckoutHandler(db)
	req := httptest.NewRequest(http.MethodGet, "/customer/subscription", nil)
	req.AddCookie(&http.Cookie{Name: "customer_session_token", Value: plainToken})
	w := httptest.NewRecorder()

	h.GetSubscription(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["plan"] != "pro" {
		t.Errorf("want plan=pro, got %v", resp["plan"])
	}
	if resp["subscription_status"] != "active" {
		t.Errorf("want subscription_status=active, got %v", resp["subscription_status"])
	}
}
