package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"time"
)

const ContextKeyCustomerID = contextKey("customer_id")

type CustomerSessionStore interface {
	GetCustomerByTokenHash(ctx context.Context, tokenHash string) (customerID string, expiresAt time.Time, err error)
}

type CustomerAuthMiddleware struct {
	store CustomerSessionStore
}

func NewCustomerAuthMiddleware(store CustomerSessionStore) *CustomerAuthMiddleware {
	return &CustomerAuthMiddleware{store: store}
}

func (m *CustomerAuthMiddleware) Require() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c, err := r.Cookie("customer_session_token")
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			h := sha256.Sum256([]byte(c.Value))
			tokenHash := hex.EncodeToString(h[:])
			customerID, expiresAt, err := m.store.GetCustomerByTokenHash(r.Context(), tokenHash)
			if err != nil || customerID == "" || time.Now().After(expiresAt) {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), ContextKeyCustomerID, customerID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
