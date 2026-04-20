package middleware

import (
	"net/http"
	"strings"
)

// CSRFProtect guards state-changing requests against cross-site request forgery.
// It uses two complementary checks:
//
//  1. For POST/PUT/DELETE, the Origin header (if present) must be in allowedOrigins.
//  2. The request body must use Content-Type: application/json — HTML forms cannot
//     set this content type, so a plain cross-site form submission is rejected.
//
// SameSite=Lax is already set on session cookies, so this middleware provides
// additional defence-in-depth without requiring a separate token round-trip.
func CSRFProtect(allowedOrigins []string) func(http.Handler) http.Handler {
	originSet := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		originSet[strings.TrimRight(o, "/")] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodPost, http.MethodPut, http.MethodPatch:
				origin := r.Header.Get("Origin")
				if origin != "" && !originSet[strings.TrimRight(origin, "/")] {
					http.Error(w, "forbidden: invalid origin", http.StatusForbidden)
					return
				}
				// Content-Type: application/json はクロスサイトHTMLフォームでは送信できないため CSRF 防御になる（Stripe Webhook は署名検証のため除外）
				ct := r.Header.Get("Content-Type")
				if !strings.HasPrefix(ct, "application/json") &&
					r.URL.Path != "/stripe/webhook" {
					http.Error(w, "forbidden: content-type must be application/json", http.StatusForbidden)
					return
				}
			case http.MethodDelete:
				// DELETE requests have no body, so Content-Type check is skipped.
				// HTML forms cannot trigger DELETE anyway; Origin check is sufficient.
				origin := r.Header.Get("Origin")
				if origin != "" && !originSet[strings.TrimRight(origin, "/")] {
					http.Error(w, "forbidden: invalid origin", http.StatusForbidden)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
