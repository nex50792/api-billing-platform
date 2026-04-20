package middleware

import (
	"context"
	"database/sql"
	"net/http"
	"strings"

	"api-billing-platform/internal/repository"
)

type contextKey string

const (
	ContextKeyAPIKey = contextKey("api_key")
)

type APIKeyAuthMiddleware struct {
	repo *repository.APIKeyRepository
}

func NewAPIKeyAuthMiddleware(repo *repository.APIKeyRepository) *APIKeyAuthMiddleware {
	return &APIKeyAuthMiddleware{repo: repo}
}

func (m *APIKeyAuthMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			http.Error(w, "missing or invalid Authorization header", http.StatusUnauthorized)
			return
		}
		plainKey := strings.TrimPrefix(header, "Bearer ")
		apiKey, err := m.repo.GetByPlainKey(r.Context(), plainKey)
		if err != nil {
			if err == sql.ErrNoRows {
				http.Error(w, "invalid api key", http.StatusUnauthorized)
				return
			}
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		if apiKey == nil {
			http.Error(w, "invalid api key", http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), ContextKeyAPIKey, apiKey)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
