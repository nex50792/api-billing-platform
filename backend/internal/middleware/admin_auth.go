package middleware

import (
	"context"
	"net/http"

	"api-billing-platform/internal/service"
)

type AdminAuthMiddleware struct {
	svc *service.AdminService
}

func NewAdminAuthMiddleware(svc *service.AdminService) *AdminAuthMiddleware {
	return &AdminAuthMiddleware{svc: svc}
}

func (m *AdminAuthMiddleware) Require() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c, err := r.Cookie("admin_session_token")
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			admin, err := m.svc.GetAdminBySession(r.Context(), c.Value)
			if err != nil || admin == nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), contextKey("admin"), admin)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
