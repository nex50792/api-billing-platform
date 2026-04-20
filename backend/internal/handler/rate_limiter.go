package handler

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For は "client, proxy1, proxy2" の形式
		if ip := strings.TrimSpace(strings.SplitN(xff, ",", 2)[0]); ip != "" {
			return ip
		}
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	// RemoteAddr は "host:port" 形式
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// IP 単位のレート制限しきい値。ブルートフォース対策の強度を一覧できるよう一箇所に集約。
const (
	rateLimitWindow        = 15 * time.Minute
	rateLimitLogin         = 10
	rateLimitSignup        = 5
	rateLimitVerify        = 5
	rateLimitForgot        = 3
	rateLimitAdminLogin    = 10
	rateLimitProviderLogin = 10
)

type RateLimiter struct {
	mu          sync.Mutex
	records     map[string][]time.Time
	lastCleanup time.Time
}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{records: make(map[string][]time.Time), lastCleanup: time.Now()}
}

func (rl *RateLimiter) Allow(key string, limit int, window time.Duration) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-window)

	prev := rl.records[key]
	filtered := prev[:0]
	for _, t := range prev {
		if t.After(cutoff) {
			filtered = append(filtered, t)
		}
	}

	if len(filtered) >= limit {
		rl.records[key] = filtered
		return false
	}
	rl.records[key] = append(filtered, now)

	// 1時間ごとにアクセスのないキーを全削除してメモリリークを防ぐ
	if now.Sub(rl.lastCleanup) > time.Hour {
		rl.cleanup(cutoff)
		rl.lastCleanup = now
	}

	return true
}

func (rl *RateLimiter) cleanup(cutoff time.Time) {
	for k, times := range rl.records {
		active := times[:0]
		for _, t := range times {
			if t.After(cutoff) {
				active = append(active, t)
			}
		}
		if len(active) == 0 {
			delete(rl.records, k)
		} else {
			rl.records[k] = active
		}
	}
}
