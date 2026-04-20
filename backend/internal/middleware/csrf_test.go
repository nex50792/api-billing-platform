package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"api-billing-platform/internal/middleware"
)

func newCSRFHandler(allowedOrigins []string) http.Handler {
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return middleware.CSRFProtect(allowedOrigins)(ok)
}

func TestCSRF_GET_AlwaysAllowed(t *testing.T) {
	h := newCSRFHandler([]string{"http://localhost:5173"})
	req := httptest.NewRequest(http.MethodGet, "/any", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("GET should always pass, got %d", rr.Code)
	}
}

func TestCSRF_POST_NoOrigin_AllowedIfJSON(t *testing.T) {
	h := newCSRFHandler([]string{"http://localhost:5173"})
	req := httptest.NewRequest(http.MethodPost, "/api", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("POST with no Origin and JSON content-type should pass, got %d", rr.Code)
	}
}

func TestCSRF_POST_ValidOrigin_AllowedIfJSON(t *testing.T) {
	h := newCSRFHandler([]string{"http://localhost:5173"})
	req := httptest.NewRequest(http.MethodPost, "/api", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:5173")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("POST from allowed origin should pass, got %d", rr.Code)
	}
}

func TestCSRF_POST_InvalidOrigin_Rejected(t *testing.T) {
	h := newCSRFHandler([]string{"http://localhost:5173"})
	req := httptest.NewRequest(http.MethodPost, "/api", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://evil.example.com")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("POST from disallowed origin should be 403, got %d", rr.Code)
	}
}

func TestCSRF_POST_FormContentType_Rejected(t *testing.T) {
	h := newCSRFHandler([]string{"http://localhost:5173"})
	req := httptest.NewRequest(http.MethodPost, "/api", strings.NewReader("field=value"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("POST with form content-type should be 403, got %d", rr.Code)
	}
}

func TestCSRF_POST_StripeWebhook_ExemptFromContentTypeCheck(t *testing.T) {
	h := newCSRFHandler([]string{"http://localhost:5173"})
	req := httptest.NewRequest(http.MethodPost, "/stripe/webhook", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json; charset=utf-8") // Stripe sends this
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("Stripe webhook should pass, got %d", rr.Code)
	}
}

func TestCSRF_POST_NoContentType_Rejected(t *testing.T) {
	h := newCSRFHandler([]string{"http://localhost:5173"})
	req := httptest.NewRequest(http.MethodPost, "/api", strings.NewReader("data"))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("POST with no Content-Type should be 403, got %d", rr.Code)
	}
}

func TestCSRF_DELETE_NoOrigin_Allowed(t *testing.T) {
	h := newCSRFHandler([]string{"http://localhost:5173"})
	req := httptest.NewRequest(http.MethodDelete, "/api/123", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("DELETE with no Origin should pass, got %d", rr.Code)
	}
}

func TestCSRF_DELETE_ValidOrigin_Allowed(t *testing.T) {
	h := newCSRFHandler([]string{"http://localhost:5173"})
	req := httptest.NewRequest(http.MethodDelete, "/api/123", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("DELETE from allowed origin should pass, got %d", rr.Code)
	}
}

func TestCSRF_DELETE_InvalidOrigin_Rejected(t *testing.T) {
	h := newCSRFHandler([]string{"http://localhost:5173"})
	req := httptest.NewRequest(http.MethodDelete, "/api/123", nil)
	req.Header.Set("Origin", "http://evil.example.com")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("DELETE from disallowed origin should be 403, got %d", rr.Code)
	}
}

func TestCSRF_PUT_InvalidOrigin_Rejected(t *testing.T) {
	h := newCSRFHandler([]string{"http://localhost:5173"})
	req := httptest.NewRequest(http.MethodPut, "/api/123", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://evil.example.com")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("PUT from disallowed origin should be 403, got %d", rr.Code)
	}
}

func TestCSRF_PUT_FormContentType_Rejected(t *testing.T) {
	h := newCSRFHandler([]string{"http://localhost:5173"})
	req := httptest.NewRequest(http.MethodPut, "/api/123", strings.NewReader("field=value"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("PUT with form content-type should be 403, got %d", rr.Code)
	}
}

func TestCSRF_PATCH_InvalidOrigin_Rejected(t *testing.T) {
	h := newCSRFHandler([]string{"http://localhost:5173"})
	req := httptest.NewRequest(http.MethodPatch, "/api/123", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://evil.example.com")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("PATCH from disallowed origin should be 403, got %d", rr.Code)
	}
}

func TestCSRF_PATCH_FormContentType_Rejected(t *testing.T) {
	h := newCSRFHandler([]string{"http://localhost:5173"})
	req := httptest.NewRequest(http.MethodPatch, "/api/123", strings.NewReader("field=value"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("PATCH with form content-type should be 403, got %d", rr.Code)
	}
}
