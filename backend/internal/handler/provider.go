package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"api-billing-platform/internal/apispec"
	"api-billing-platform/internal/repository"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"golang.org/x/crypto/bcrypt"
)

type Mailer interface {
	SendPasswordSetupEmail(to, link string) error
	SendPasswordResetEmail(to, link string) error
	SendVerificationCodeEmail(to, code string) error
}

type ProviderHandler struct {
	db          *sql.DB
	repo        *repository.ProviderRepository
	Mailer      Mailer
	rateLimiter *RateLimiter
}

func NewProviderHandler(db *sql.DB, mailer Mailer) *ProviderHandler {
	var repo *repository.ProviderRepository
	if db != nil {
		repo = repository.NewProviderRepository(db)
	}
	return &ProviderHandler{db: db, repo: repo, Mailer: mailer, rateLimiter: NewRateLimiter()}
}

func (h *ProviderHandler) GetProviders(w http.ResponseWriter, r *http.Request) {
	limit := 100
	offset := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	items, err := h.repo.List(r.Context(), limit, offset)
	if err != nil {
		http.Error(w, "failed to fetch providers", http.StatusInternalServerError)
		return
	}
	providers := make([]apispec.Provider, 0, len(items))
	for _, p := range items {
		uid, _ := uuid.Parse(p.ID)
		providers = append(providers, apispec.Provider{
			Id:       uid,
			Name:     p.Name,
			Email:    openapi_types.Email(p.Email),
			IsActive: p.IsActive,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(providers)
}

func (h *ProviderHandler) CreateProvider(w http.ResponseWriter, r *http.Request) {
	var input apispec.CreateProviderRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid input", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(input.Name)
	email := strings.TrimSpace(string(input.Email))
	if name == "" || email == "" {
		http.Error(w, "name and email required", http.StatusBadRequest)
		return
	}
	if !isValidEmail(email) {
		http.Error(w, "invalid email address", http.StatusBadRequest)
		return
	}
	existingID, err := h.repo.ExistingByEmail(r.Context(), email)
	if err == nil && existingID != "" {
		plainToken, err := generateSessionToken()
		if err != nil {
			http.Error(w, "failed to generate token", http.StatusInternalServerError)
			return
		}
		hashedToken := hashSessionToken(plainToken)
		tokenExpiry := time.Now().Add(1 * time.Hour)
		if err := h.repo.UpdatePasswordToken(r.Context(), existingID, hashedToken, tokenExpiry); err != nil {
			http.Error(w, "failed to update token", http.StatusInternalServerError)
			return
		}
		link := frontendURL() + "/provider/set-password?token=" + plainToken
		if h.Mailer != nil {
			if err := h.Mailer.SendPasswordSetupEmail(email, link); err != nil {
				slog.Error("create provider failed to send re-invite email", "email", email, "error", err)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "password reset mail sent"})
		return
	}
	id := uuid.New().String()
	plainToken, err := generateSessionToken()
	if err != nil {
		http.Error(w, "failed to generate token", http.StatusInternalServerError)
		return
	}
	hashedToken := hashSessionToken(plainToken)
	tokenExpiry := time.Now().Add(1 * time.Hour)
	if err := h.repo.Create(r.Context(), id, name, email, hashedToken, tokenExpiry); err != nil {
		http.Error(w, "failed to create provider", http.StatusInternalServerError)
		return
	}
	link := frontendURL() + "/provider/set-password?token=" + plainToken
	if h.Mailer != nil {
		if err := h.Mailer.SendPasswordSetupEmail(email, link); err != nil {
			slog.Error("create provider failed to send setup email", "email", email, "error", err)
		}
	}
	pid, _ := uuid.Parse(id)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(apispec.Provider{
		Id:       pid,
		Name:     name,
		Email:    openapi_types.Email(email),
		IsActive: false,
	})
}

// メールアドレスが存在しない場合でも 200 を返す（ユーザー列挙攻撃の防止）。
func (h *ProviderHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var input apispec.ForgotPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil || strings.TrimSpace(string(input.Email)) == "" {
		http.Error(w, "email required", http.StatusBadRequest)
		return
	}
	email := strings.TrimSpace(string(input.Email))

	ip := clientIP(r)
	if !h.rateLimiter.Allow("provider_forgot:"+ip, rateLimitForgot, rateLimitWindow) {
		http.Error(w, "too many requests", http.StatusTooManyRequests)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	id, err := h.repo.GetActiveIDByEmail(r.Context(), email)
	if err != nil {
		// アドレスが存在しない or DB エラーでも同じ応答（ユーザー列挙防止）
		json.NewEncoder(w).Encode(map[string]string{"message": "リセットリンクをメールで送信しました（登録済みの場合）"})
		return
	}

	plainToken, genErr := generateSessionToken()
	if genErr != nil {
		http.Error(w, "failed to generate reset token", http.StatusInternalServerError)
		return
	}
	hashedToken := hashSessionToken(plainToken)
	tokenExpiry := time.Now().Add(1 * time.Hour)
	if err := h.repo.UpdatePasswordToken(r.Context(), id, hashedToken, tokenExpiry); err != nil {
		http.Error(w, "failed to generate reset token", http.StatusInternalServerError)
		return
	}

	link := frontendURL() + "/provider/set-password?token=" + plainToken
	if h.Mailer != nil {
		if err := h.Mailer.SendPasswordResetEmail(email, link); err != nil {
			slog.Error("provider forgot password failed to send reset email", "email", email, "error", err)
		}
	}
	json.NewEncoder(w).Encode(map[string]string{"message": "リセットリンクをメールで送信しました（登録済みの場合）"})
}

func (h *ProviderHandler) ValidatePasswordToken(w http.ResponseWriter, r *http.Request) {
	var input apispec.TokenOnlyRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.Token == "" {
		http.Error(w, "token required", http.StatusBadRequest)
		return
	}
	expiry, err := h.repo.GetPasswordTokenExpiry(r.Context(), hashSessionToken(input.Token))
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "invalid token", http.StatusBadRequest)
		return
	} else if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	if time.Now().After(expiry) {
		http.Error(w, "expired token", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":true}`))
}

func (h *ProviderHandler) SetPassword(w http.ResponseWriter, r *http.Request) {
	var input apispec.SetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid input", http.StatusBadRequest)
		return
	}
	if input.Token == "" || input.Password == "" {
		http.Error(w, "token and password required", http.StatusBadRequest)
		return
	}
	if len(input.Password) < 8 {
		http.Error(w, "password must be at least 8 characters", http.StatusBadRequest)
		return
	}
	tokenHash := hashSessionToken(input.Token)
	expiry, err := h.repo.GetPasswordTokenExpiry(r.Context(), tokenHash)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "invalid token", http.StatusBadRequest)
		return
	} else if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	if time.Now().After(expiry) {
		http.Error(w, "expired token", http.StatusBadRequest)
		return
	}
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "failed to hash password", http.StatusInternalServerError)
		return
	}
	providerID, err := h.repo.SetPassword(r.Context(), tokenHash, string(hashedBytes))
	if err != nil {
		http.Error(w, "failed to set password", http.StatusInternalServerError)
		return
	}
	if err := h.repo.DeleteAllSessions(r.Context(), providerID); err != nil {
		slog.Error("provider set password failed to invalidate sessions", "error", err)
	}
	w.WriteHeader(http.StatusOK)
}

func (h *ProviderHandler) Login(w http.ResponseWriter, r *http.Request) {
	var input apispec.CredentialsRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	id, hash, err := h.repo.GetCredentials(r.Context(), string(input.Email))
	if errors.Is(err, sql.ErrNoRows) {
		ip := clientIP(r)
		if !h.rateLimiter.Allow("login:"+ip, rateLimitProviderLogin, rateLimitWindow) {
			http.Error(w, "too many failed login attempts", http.StatusTooManyRequests)
			return
		}
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	} else if err != nil {
		slog.Error("provider login db error", "error", err)
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(input.Password)); err != nil {
		ip := clientIP(r)
		if !h.rateLimiter.Allow("login:"+ip, rateLimitProviderLogin, rateLimitWindow) {
			http.Error(w, "too many failed login attempts", http.StatusTooManyRequests)
			return
		}
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	plainToken, err := generateSessionToken()
	if err != nil {
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}
	tokenHash := hashSessionToken(plainToken)
	expiresAt := time.Now().Add(24 * time.Hour)
	if err := h.repo.CreateSession(r.Context(), id, tokenHash, expiresAt); err != nil {
		slog.Error("provider login failed to create session", "error", err)
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}
	setSessionCookie(w, "provider_session_token", plainToken, expiresAt)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "ok"})
}

func (h *ProviderHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie("provider_session_token"); err == nil {
		tokenHash := hashSessionToken(c.Value)
		if err := h.repo.DeleteSessionByTokenHash(r.Context(), tokenHash); err != nil {
			slog.Error("provider logout failed to delete session", "error", err)
		}
	}
	clearSessionCookie(w, "provider_session_token")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "logged out"})
}

func (h *ProviderHandler) DeleteProvider(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}
	rows, err := h.repo.Delete(r.Context(), id)
	if err != nil {
		http.Error(w, "failed to delete provider", http.StatusInternalServerError)
		return
	}
	if rows == 0 {
		http.Error(w, "provider not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ProviderHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	var input apispec.PasswordChangeRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid input", http.StatusBadRequest)
		return
	}
	if input.CurrentPassword == "" || input.NewPassword == "" {
		http.Error(w, "current_password and new_password required", http.StatusBadRequest)
		return
	}
	if len(input.NewPassword) < 8 {
		http.Error(w, "password must be at least 8 characters", http.StatusBadRequest)
		return
	}

	providerID, err := h.providerIDFromSession(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	hash, err := h.repo.GetPasswordHash(r.Context(), providerID)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(input.CurrentPassword)); err != nil {
		http.Error(w, "current password is incorrect", http.StatusUnauthorized)
		return
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "failed to hash password", http.StatusInternalServerError)
		return
	}
	if err := h.repo.UpdatePasswordHash(r.Context(), providerID, string(newHash)); err != nil {
		http.Error(w, "failed to update password", http.StatusInternalServerError)
		return
	}

	if c, err := r.Cookie("provider_session_token"); err == nil {
		currentHash := hashSessionToken(c.Value)
		if err := h.repo.DeleteOtherSessions(r.Context(), providerID, currentHash); err != nil {
			slog.Error("provider change password failed to invalidate other sessions", "error", err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "パスワードを変更しました"})
}

func (h *ProviderHandler) Me(w http.ResponseWriter, r *http.Request) {
	providerID, err := h.providerIDFromSession(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	me, err := h.repo.GetMe(r.Context(), providerID)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	pid, _ := uuid.Parse(me.ID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(apispec.ProviderMe{
		Id:    pid,
		Name:  me.Name,
		Email: openapi_types.Email(me.Email),
	})
}

func (h *ProviderHandler) providerIDFromSession(r *http.Request) (string, error) {
	c, err := r.Cookie("provider_session_token")
	if err != nil {
		return "", err
	}
	if h.repo == nil {
		return "", sql.ErrNoRows
	}
	return h.repo.GetProviderIDByTokenHash(r.Context(), hashSessionToken(c.Value))
}
