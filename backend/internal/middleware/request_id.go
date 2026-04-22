package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
)

type requestIDKeyType string

const requestIDKey requestIDKeyType = "request_id"

// 上流 proxy (Cloudflare 等) が付けた X-Request-Id を優先。無ければ 16 バイトランダム。
// slog.Default を request id 付き logger に差し替えてハンドラに伝搬する。
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-Id")
		if id == "" {
			b := make([]byte, 16)
			if _, err := rand.Read(b); err == nil {
				id = hex.EncodeToString(b)
			}
		}
		w.Header().Set("X-Request-Id", id)
		ctx := context.WithValue(r.Context(), requestIDKey, id)
		ctx = context.WithValue(ctx, slogLoggerKey, slog.Default().With("request_id", id))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequestIDFromContext はデバッグや追加ログで id を取り出す用途。
func RequestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey).(string); ok {
		return v
	}
	return ""
}

type slogLoggerKeyType string

const slogLoggerKey slogLoggerKeyType = "slog_logger"

// LoggerFromContext は request id 付きの slog.Logger を返す。未設定時は default logger。
func LoggerFromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(slogLoggerKey).(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}
