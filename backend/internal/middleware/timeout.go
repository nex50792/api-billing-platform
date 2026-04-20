package middleware

import (
	"context"
	"net/http"
	"time"
)

// DefaultRequestTimeout は、ハンドラが外部 API や DB 呼び出しで詰まった場合のデッドライン。
// Stripe Webhook や SMTP など長めに必要なエンドポイントは個別に context を再設定すること。
const DefaultRequestTimeout = 30 * time.Second

func Timeout(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
