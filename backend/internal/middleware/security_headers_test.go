package middleware_test

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"api-billing-platform/internal/middleware"
)

func newSecurityHeadersHandler() http.Handler {
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return middleware.SecurityHeaders(ok)
}

func TestSecurityHeaders_XContentTypeOptions(t *testing.T) {
	rr := httptest.NewRecorder()
	newSecurityHeadersHandler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if got := rr.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options: want nosniff, got %q", got)
	}
}

func TestSecurityHeaders_XFrameOptions(t *testing.T) {
	rr := httptest.NewRecorder()
	newSecurityHeadersHandler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if got := rr.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Errorf("X-Frame-Options: want DENY, got %q", got)
	}
}

func TestSecurityHeaders_ReferrerPolicy(t *testing.T) {
	rr := httptest.NewRecorder()
	newSecurityHeadersHandler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if got := rr.Header().Get("Referrer-Policy"); got != "strict-origin-when-cross-origin" {
		t.Errorf("Referrer-Policy: want strict-origin-when-cross-origin, got %q", got)
	}
}

func TestSecurityHeaders_PermissionsPolicy(t *testing.T) {
	rr := httptest.NewRecorder()
	newSecurityHeadersHandler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if got := rr.Header().Get("Permissions-Policy"); got == "" {
		t.Error("Permissions-Policy header should be set")
	}
}

func TestSecurityHeaders_HSTS_NotSetOverHTTP(t *testing.T) {
	rr := httptest.NewRecorder()
	// r.TLS == nil (plain HTTP) → HSTS should NOT be set
	newSecurityHeadersHandler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if got := rr.Header().Get("Strict-Transport-Security"); got != "" {
		t.Errorf("HSTS should not be set over HTTP, got %q", got)
	}
}

func TestSecurityHeaders_HSTS_SetOverHTTPS(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "https://example.com/", nil)
	req.TLS = &tls.ConnectionState{} // r.TLS != nil → HTTPS
	newSecurityHeadersHandler().ServeHTTP(rr, req)
	got := rr.Header().Get("Strict-Transport-Security")
	if got == "" {
		t.Error("HSTS should be set over HTTPS")
	}
	if !strings.Contains(got, "max-age=") {
		t.Errorf("HSTS value should contain max-age, got %q", got)
	}
}
