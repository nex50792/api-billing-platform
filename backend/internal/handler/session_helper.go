package handler

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"net/mail"
	"os"
	"time"
)

func setSessionCookie(w http.ResponseWriter, name, value string, expires time.Time) {
	// 本番 (HTTPS + クロスサイト) では SameSite=None が必須。ローカル開発 (same-origin) は Lax で十分。
	sameSite := http.SameSiteLaxMode
	if isSecureCookie() {
		sameSite = http.SameSiteNoneMode
	}
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   isSecureCookie(),
		Expires:  expires,
		SameSite: sameSite,
	})
}

func clearSessionCookie(w http.ResponseWriter, name string) {
	setSessionCookie(w, name, "", time.Unix(0, 0))
}

func generateSessionToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func hashSessionToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

func isValidEmail(email string) bool {
	_, err := mail.ParseAddress(email)
	return err == nil
}

func isSecureCookie() bool {
	return os.Getenv("APP_ENV") == "production"
}

func frontendURL() string {
	if v := os.Getenv("FRONTEND_URL"); v != "" {
		return v
	}
	return "http://localhost:5173"
}
