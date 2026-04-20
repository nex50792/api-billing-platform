package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"time"
)

const ContextKeyProviderID = contextKey("provider_id")

type ProviderSessionStore interface {
	GetProviderByTokenHash(ctx context.Context, tokenHash string) (providerID string, expiresAt time.Time, err error)
}

type ProviderAuthMiddleware struct {
	store ProviderSessionStore
}

func NewProviderAuthMiddleware(store ProviderSessionStore) *ProviderAuthMiddleware {
	return &ProviderAuthMiddleware{store: store}
}

func (m *ProviderAuthMiddleware) Require() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c, err := r.Cookie("provider_session_token")
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			h := sha256.Sum256([]byte(c.Value))
			tokenHash := hex.EncodeToString(h[:])
			providerID, expiresAt, err := m.store.GetProviderByTokenHash(r.Context(), tokenHash)
			if err != nil || providerID == "" || time.Now().After(expiresAt) {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), ContextKeyProviderID, providerID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
