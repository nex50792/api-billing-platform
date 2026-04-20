package handler_test

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"api-billing-platform/internal/handler"

	_ "github.com/glebarez/sqlite"
)

func generateWebhookSig(secret string, payload []byte) string {
	ts := time.Now().Unix()
	mac := hmac.New(sha256.New, []byte(secret))
	fmt.Fprintf(mac, "%d.", ts)
	mac.Write(payload)
	sig := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("t=%d,v1=%s", ts, sig)
}

func setupWebhookTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	db.Exec(`CREATE TABLE stripe_events (event_id TEXT PRIMARY KEY)`)
	db.Exec(`CREATE TABLE customers (id TEXT PRIMARY KEY, name TEXT, email TEXT, plan TEXT DEFAULT 'free', stripe_customer_id TEXT, subscription_status TEXT, created_at TEXT)`)
	return db
}

func TestStripeWebhook_CheckoutCompleted_UpgradesToPro(t *testing.T) {
	const secret = "test-webhook-secret"
	t.Setenv("STRIPE_WEBHOOK_SECRET", secret)

	db := setupWebhookTestDB(t)
	db.Exec(`INSERT INTO customers (id, name, email, plan, stripe_customer_id) VALUES ('cust-wh-1', 'Test', 'wh@example.com', 'free', '')`)

	payload, _ := json.Marshal(map[string]interface{}{
		"id":     "evt_checkout_1",
		"object": "event",
		"type":   "checkout.session.completed",
		"data": map[string]interface{}{
			"object": map[string]interface{}{
				"id":                  "cs_test_1",
				"object":              "checkout.session",
				"customer":            "cus_stripe_abc",
				"client_reference_id": "cust-wh-1",
				"subscription":        "sub_test_1",
			},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/stripe/webhook", bytes.NewReader(payload))
	req.Header.Set("Stripe-Signature", generateWebhookSig(secret, payload))
	w := httptest.NewRecorder()

	handler.NewStripeWebhookHandler(db).Handle(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var plan, stripeID string
	db.QueryRow(`SELECT plan, stripe_customer_id FROM customers WHERE id = 'cust-wh-1'`).Scan(&plan, &stripeID)
	if plan != "pro" {
		t.Errorf("want plan=pro, got %s", plan)
	}
	if stripeID != "cus_stripe_abc" {
		t.Errorf("want stripe_customer_id=cus_stripe_abc, got %s", stripeID)
	}
}

func TestStripeWebhook_SubscriptionDeleted_DowngradesToFree(t *testing.T) {
	const secret = "test-webhook-secret"
	t.Setenv("STRIPE_WEBHOOK_SECRET", secret)

	db := setupWebhookTestDB(t)
	db.Exec(`INSERT INTO customers (id, name, email, plan, stripe_customer_id, subscription_status) VALUES ('cust-wh-2', 'Pro User', 'pro@example.com', 'pro', 'cus_stripe_pro', 'active')`)

	payload, _ := json.Marshal(map[string]interface{}{
		"id":     "evt_sub_deleted_1",
		"object": "event",
		"type":   "customer.subscription.deleted",
		"data": map[string]interface{}{
			"object": map[string]interface{}{
				"id":       "sub_test_2",
				"object":   "subscription",
				"customer": "cus_stripe_pro",
				"status":   "canceled",
			},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/stripe/webhook", bytes.NewReader(payload))
	req.Header.Set("Stripe-Signature", generateWebhookSig(secret, payload))
	w := httptest.NewRecorder()

	handler.NewStripeWebhookHandler(db).Handle(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var plan, status string
	db.QueryRow(`SELECT plan, subscription_status FROM customers WHERE stripe_customer_id = 'cus_stripe_pro'`).Scan(&plan, &status)
	if plan != "free" {
		t.Errorf("want plan=free, got %s", plan)
	}
	if status != "canceled" {
		t.Errorf("want subscription_status=canceled, got %s", status)
	}
}

func TestStripeWebhook_Idempotent_SkipsDuplicateEvent(t *testing.T) {
	const secret = "test-webhook-secret"
	t.Setenv("STRIPE_WEBHOOK_SECRET", secret)

	db := setupWebhookTestDB(t)
	db.Exec(`INSERT INTO customers (id, name, email, plan, stripe_customer_id) VALUES ('cust-wh-3', 'Test', 'idem@example.com', 'free', '')`)
	db.Exec(`INSERT INTO stripe_events (event_id) VALUES ('evt_idempotent_1')`)

	payload, _ := json.Marshal(map[string]interface{}{
		"id":     "evt_idempotent_1",
		"object": "event",
		"type":   "checkout.session.completed",
		"data": map[string]interface{}{
			"object": map[string]interface{}{
				"customer":            "cus_stripe_xyz",
				"client_reference_id": "cust-wh-3",
				"subscription":        "sub_test_3",
			},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/stripe/webhook", bytes.NewReader(payload))
	req.Header.Set("Stripe-Signature", generateWebhookSig(secret, payload))
	w := httptest.NewRecorder()

	handler.NewStripeWebhookHandler(db).Handle(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var plan string
	db.QueryRow(`SELECT plan FROM customers WHERE id = 'cust-wh-3'`).Scan(&plan)
	if plan != "free" {
		t.Errorf("want plan=free (unchanged), got %s", plan)
	}
}

func TestStripeWebhookHandler_Handle_MissingSecret(t *testing.T) {
	t.Setenv("STRIPE_WEBHOOK_SECRET", "")
	h := handler.NewStripeWebhookHandler((*sql.DB)(nil))
	req := httptest.NewRequest(http.MethodPost, "/stripe/webhook", nil)
	w := httptest.NewRecorder()

	h.Handle(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", w.Code)
	}
}

// cancel_at_period_end=true は subscription_status を "canceling" に写像する。
func TestStripeWebhook_SubscriptionUpdated_CancelAtPeriodEnd(t *testing.T) {
	const secret = "test-webhook-secret"
	t.Setenv("STRIPE_WEBHOOK_SECRET", secret)

	db := setupWebhookTestDB(t)
	db.Exec(`INSERT INTO customers (id, name, email, plan, stripe_customer_id, subscription_status) VALUES ('cust-wh-4', 'Cancel User', 'cancel@example.com', 'pro', 'cus_cancel_123', 'active')`)

	payload, _ := json.Marshal(map[string]interface{}{
		"id":     "evt_sub_updated_1",
		"object": "event",
		"type":   "customer.subscription.updated",
		"data": map[string]interface{}{
			"object": map[string]interface{}{
				"id":                   "sub_cancel_1",
				"object":               "subscription",
				"customer":             "cus_cancel_123",
				"status":               "active",
				"cancel_at_period_end": true,
			},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/stripe/webhook", bytes.NewReader(payload))
	req.Header.Set("Stripe-Signature", generateWebhookSig(secret, payload))
	w := httptest.NewRecorder()

	handler.NewStripeWebhookHandler(db).Handle(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var status string
	db.QueryRow(`SELECT subscription_status FROM customers WHERE stripe_customer_id = 'cus_cancel_123'`).Scan(&status)
	if status != "canceling" {
		t.Errorf("want subscription_status=canceling, got %s", status)
	}
}

func TestStripeWebhook_InvoicePaymentFailed_SetsPastDue(t *testing.T) {
	const secret = "test-webhook-secret"
	t.Setenv("STRIPE_WEBHOOK_SECRET", secret)

	db := setupWebhookTestDB(t)
	db.Exec(`INSERT INTO customers (id, name, email, plan, stripe_customer_id, subscription_status) VALUES ('cust-wh-5', 'Late User', 'late@example.com', 'pro', 'cus_late_123', 'active')`)

	payload, _ := json.Marshal(map[string]interface{}{
		"id":     "evt_invoice_failed_1",
		"object": "event",
		"type":   "invoice.payment_failed",
		"data": map[string]interface{}{
			"object": map[string]interface{}{
				"id":       "in_test_1",
				"object":   "invoice",
				"customer": "cus_late_123",
			},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/stripe/webhook", bytes.NewReader(payload))
	req.Header.Set("Stripe-Signature", generateWebhookSig(secret, payload))
	w := httptest.NewRecorder()

	handler.NewStripeWebhookHandler(db).Handle(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var status string
	db.QueryRow(`SELECT subscription_status FROM customers WHERE stripe_customer_id = 'cus_late_123'`).Scan(&status)
	if status != "past_due" {
		t.Errorf("want subscription_status=past_due, got %s", status)
	}
}

func TestStripeWebhook_CheckoutCompleted_EmptyReferenceID(t *testing.T) {
	const secret = "test-webhook-secret"
	t.Setenv("STRIPE_WEBHOOK_SECRET", secret)

	db := setupWebhookTestDB(t)
	db.Exec(`INSERT INTO customers (id, name, email, plan, stripe_customer_id) VALUES ('cust-wh-ref', 'Ref User', 'ref@example.com', 'free', '')`)

	payload, _ := json.Marshal(map[string]interface{}{
		"id":     "evt_empty_ref_1",
		"object": "event",
		"type":   "checkout.session.completed",
		"data": map[string]interface{}{
			"object": map[string]interface{}{
				"id":                  "cs_empty_ref",
				"object":              "checkout.session",
				"customer":            "cus_some_stripe_id",
				"client_reference_id": "",
				"subscription":        "sub_empty_ref",
			},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/stripe/webhook", bytes.NewReader(payload))
	req.Header.Set("Stripe-Signature", generateWebhookSig(secret, payload))
	w := httptest.NewRecorder()

	handler.NewStripeWebhookHandler(db).Handle(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var plan string
	db.QueryRow(`SELECT plan FROM customers WHERE id = 'cust-wh-ref'`).Scan(&plan)
	if plan != "free" {
		t.Errorf("want plan=free (unchanged), got %s", plan)
	}
}

func TestStripeWebhook_SubscriptionCreated_UpdatesStatus(t *testing.T) {
	const secret = "test-webhook-secret"
	t.Setenv("STRIPE_WEBHOOK_SECRET", secret)

	db := setupWebhookTestDB(t)
	db.Exec(`INSERT INTO customers (id, name, email, plan, stripe_customer_id, subscription_status) VALUES ('cust-wh-sub-c', 'New Sub', 'newsub@example.com', 'pro', 'cus_sub_created', '')`)

	payload, _ := json.Marshal(map[string]interface{}{
		"id":     "evt_sub_created_1",
		"object": "event",
		"type":   "customer.subscription.created",
		"data": map[string]interface{}{
			"object": map[string]interface{}{
				"id":                   "sub_new_1",
				"object":               "subscription",
				"customer":             "cus_sub_created",
				"status":               "active",
				"cancel_at_period_end": false,
			},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/stripe/webhook", bytes.NewReader(payload))
	req.Header.Set("Stripe-Signature", generateWebhookSig(secret, payload))
	w := httptest.NewRecorder()

	handler.NewStripeWebhookHandler(db).Handle(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var status string
	db.QueryRow(`SELECT subscription_status FROM customers WHERE stripe_customer_id = 'cus_sub_created'`).Scan(&status)
	if status != "active" {
		t.Errorf("want subscription_status=active, got %s", status)
	}
}

func TestStripeWebhook_UnknownEvent_Returns200(t *testing.T) {
	const secret = "test-webhook-secret"
	t.Setenv("STRIPE_WEBHOOK_SECRET", secret)

	db := setupWebhookTestDB(t)

	payload, _ := json.Marshal(map[string]interface{}{
		"id":     "evt_unknown_1",
		"object": "event",
		"type":   "payment_intent.created",
		"data":   map[string]interface{}{"object": map[string]interface{}{}},
	})
	req := httptest.NewRequest(http.MethodPost, "/stripe/webhook", bytes.NewReader(payload))
	req.Header.Set("Stripe-Signature", generateWebhookSig(secret, payload))
	w := httptest.NewRecorder()

	handler.NewStripeWebhookHandler(db).Handle(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("want 200 for unknown event, got %d", w.Code)
	}
}

func TestStripeWebhookHandler_Handle_InvalidSignature(t *testing.T) {
	t.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_test_secret")
	h := handler.NewStripeWebhookHandler((*sql.DB)(nil))
	req := httptest.NewRequest(http.MethodPost, "/stripe/webhook", nil)
	req.Header.Set("Stripe-Signature", "invalid")
	w := httptest.NewRecorder()

	h.Handle(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}
