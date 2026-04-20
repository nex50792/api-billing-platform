package handler

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimiter_Allow_UnderLimit(t *testing.T) {
	rl := NewRateLimiter()
	for i := 0; i < 5; i++ {
		if !rl.Allow("key1", 5, time.Minute) {
			t.Errorf("attempt %d: expected true, got false", i+1)
		}
	}
}

func TestRateLimiter_Allow_OverLimit(t *testing.T) {
	rl := NewRateLimiter()
	for i := 0; i < 3; i++ {
		rl.Allow("key2", 3, time.Minute)
	}
	if rl.Allow("key2", 3, time.Minute) {
		t.Error("expected false after limit exceeded, got true")
	}
}

func TestRateLimiter_Allow_DifferentKeys(t *testing.T) {
	rl := NewRateLimiter()
	for i := 0; i < 3; i++ {
		rl.Allow("keyA", 3, time.Minute)
	}
	if !rl.Allow("keyB", 3, time.Minute) {
		t.Error("different key should not be rate limited")
	}
}

func TestRateLimiter_Allow_WindowExpiry(t *testing.T) {
	rl := NewRateLimiter()
	// 極めて短いウィンドウ（1ナノ秒）でリミットを超える
	for i := 0; i < 2; i++ {
		rl.Allow("key3", 2, time.Nanosecond)
	}
	time.Sleep(time.Millisecond)
	if !rl.Allow("key3", 2, time.Nanosecond) {
		t.Error("expected true after window expiry, got false")
	}
}

func TestClientIP_XForwardedFor(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-For", "1.2.3.4, 10.0.0.1, 10.0.0.2")
	got := clientIP(req)
	if got != "1.2.3.4" {
		t.Errorf("want 1.2.3.4, got %s", got)
	}
}

func TestClientIP_XRealIP(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Real-IP", "5.6.7.8")
	got := clientIP(req)
	if got != "5.6.7.8" {
		t.Errorf("want 5.6.7.8, got %s", got)
	}
}

func TestClientIP_RemoteAddr(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "9.10.11.12:54321"
	got := clientIP(req)
	if got != "9.10.11.12" {
		t.Errorf("want 9.10.11.12, got %s", got)
	}
}

func TestClientIP_XForwardedFor_Priority(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-For", "1.1.1.1")
	req.Header.Set("X-Real-IP", "2.2.2.2")
	got := clientIP(req)
	if got != "1.1.1.1" {
		t.Errorf("want X-Forwarded-For (1.1.1.1) to take priority, got %s", got)
	}
}
