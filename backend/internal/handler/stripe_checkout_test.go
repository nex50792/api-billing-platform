package handler_test

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"api-billing-platform/internal/handler"
)

func TestStripeCheckoutHandler_CreateCheckoutSession_Unauthorized(t *testing.T) {
	h := handler.NewStripeCheckoutHandler((*sql.DB)(nil))
	body := strings.NewReader(`{"team_id":"test-team-id"}`)
	req := httptest.NewRequest(http.MethodPost, "/customer/checkout", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateCheckoutSession(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func TestStripeCheckoutHandler_GetSubscription_Unauthorized(t *testing.T) {
	h := handler.NewStripeCheckoutHandler((*sql.DB)(nil))
	req := httptest.NewRequest(http.MethodGet, "/customer/subscription?team_id=test", nil)
	w := httptest.NewRecorder()

	h.GetSubscription(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func TestStripeCheckoutHandler_CreateBillingPortalSession_Unauthorized(t *testing.T) {
	h := handler.NewStripeCheckoutHandler((*sql.DB)(nil))
	req := httptest.NewRequest(http.MethodPost, "/customer/billing-portal", nil)
	w := httptest.NewRecorder()

	h.CreateBillingPortalSession(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}
