package handler

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"strings"
	"time"

	"api-billing-platform/internal/apispec"
	"api-billing-platform/internal/repository"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"golang.org/x/crypto/bcrypt"
)

type CustomerHandler struct {
	repo        *repository.CustomerRepository
	Mailer      Mailer
	rateLimiter *RateLimiter
}

func NewCustomerHandler(db *sql.DB) *CustomerHandler {
	var repo *repository.CustomerRepository
	if db != nil {
		repo = repository.NewCustomerRepository(db)
	}
	return &CustomerHandler{repo: repo, rateLimiter: NewRateLimiter()}
}

func generateVerificationCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(900000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()+100000), nil
}

func (h *CustomerHandler) VerifyCode(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if !h.rateLimiter.Allow("verify:"+ip, rateLimitVerify, rateLimitWindow) {
		http.Error(w, "too many requests", http.StatusTooManyRequests)
		return
	}
	var input apispec.VerifyCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid input", http.StatusBadRequest)
		return
	}
	customerID, err := h.repo.GetIDByEmailAny(r.Context(), string(input.Email))
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "customer not found", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	vc, err := h.repo.GetLatestVerificationCode(r.Context(), customerID)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "no verification code found", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	if time.Now().After(vc.ExpiresAt) {
		http.Error(w, "verification code expired", http.StatusBadRequest)
		return
	}
	if input.Code != vc.Code {
		http.Error(w, "invalid code", http.StatusBadRequest)
		return
	}
	if err := h.repo.MarkVerificationCodeUsed(r.Context(), customerID, input.Code); err != nil {
		http.Error(w, "failed to update verification code", http.StatusInternalServerError)
		return
	}
	if err := h.repo.Activate(r.Context(), customerID); err != nil {
		http.Error(w, "failed to activate customer", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "verified"})
}

func (h *CustomerHandler) Login(w http.ResponseWriter, r *http.Request) {
	var input apispec.CredentialsRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	id, hash, err := h.repo.GetByEmail(r.Context(), string(input.Email))
	if errors.Is(err, sql.ErrNoRows) {
		ip := clientIP(r)
		if !h.rateLimiter.Allow("login:"+ip, rateLimitLogin, rateLimitWindow) {
			http.Error(w, "too many failed login attempts", http.StatusTooManyRequests)
			return
		}
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	} else if err != nil {
		slog.Error("customer login db error", "error", err)
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(input.Password)); err != nil {
		ip := clientIP(r)
		if !h.rateLimiter.Allow("login:"+ip, rateLimitLogin, rateLimitWindow) {
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
		slog.Error("customer login failed to create session", "error", err)
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}
	setSessionCookie(w, "customer_session_token", plainToken, expiresAt)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "ok"})
}

func (h *CustomerHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie("customer_session_token"); err == nil {
		tokenHash := hashSessionToken(c.Value)
		if err := h.repo.DeleteSessionByTokenHash(r.Context(), tokenHash); err != nil {
			slog.Error("customer logout failed to delete session", "error", err)
		}
	}
	clearSessionCookie(w, "customer_session_token")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "logged out"})
}

func (h *CustomerHandler) Me(w http.ResponseWriter, r *http.Request) {
	customerID, err := h.customerIDFromSession(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	me, err := h.repo.GetMe(r.Context(), customerID)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	id, _ := uuid.Parse(customerID)
	resp := apispec.CustomerMe{
		Id:    id,
		Name:  me.Name,
		Email: openapi_types.Email(me.Email),
		Plan:  apispec.CustomerMePlan(me.Plan),
	}
	if me.SubscriptionStatus != nil {
		resp.SubscriptionStatus = me.SubscriptionStatus
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *CustomerHandler) Signup(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if !h.rateLimiter.Allow("signup:"+ip, rateLimitSignup, rateLimitWindow) {
		http.Error(w, "too many requests", http.StatusTooManyRequests)
		return
	}
	var input apispec.SignupRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid input", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(input.Name)
	email := strings.TrimSpace(string(input.Email))
	if name == "" || email == "" || input.Password == "" {
		http.Error(w, "all fields required", http.StatusBadRequest)
		return
	}
	if !isValidEmail(email) {
		http.Error(w, "invalid email address", http.StatusBadRequest)
		return
	}
	if len(input.Password) < 8 {
		http.Error(w, "password must be at least 8 characters", http.StatusBadRequest)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "failed to hash password", http.StatusInternalServerError)
		return
	}
	code, err := generateVerificationCode()
	if err != nil {
		http.Error(w, "failed to generate code", http.StatusInternalServerError)
		return
	}
	expires := time.Now().Add(15 * time.Minute)

	existing, err := h.repo.GetIDByEmailIncludingInactive(r.Context(), email)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	if existing != nil {
		if existing.IsActive {
			http.Error(w, "email already exists", http.StatusConflict)
			return
		}
		if err := h.repo.UpdateForReissue(r.Context(), existing.ID, name, string(hash)); err != nil {
			http.Error(w, "failed to update customer", http.StatusInternalServerError)
			return
		}
		if err := h.repo.InvalidateUnusedVerificationCodes(r.Context(), existing.ID); err != nil {
			slog.Error("customer signup failed to invalidate old codes", "error", err)
		}
		if err := h.repo.CreateVerificationCode(r.Context(), existing.ID, code, expires); err != nil {
			http.Error(w, "failed to save verification code", http.StatusInternalServerError)
			return
		}
		h.sendVerificationCode(email, code)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"message": "signup ok (reissued)", "verification": "sent"})
		return
	}

	id := uuid.New().String()
	if err := h.repo.Create(r.Context(), id, name, email, string(hash)); err != nil {
		http.Error(w, "failed to create customer", http.StatusInternalServerError)
		return
	}
	if err := h.repo.CreateVerificationCode(r.Context(), id, code, expires); err != nil {
		http.Error(w, "failed to save verification code", http.StatusInternalServerError)
		return
	}
	h.sendVerificationCode(email, code)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"message": "signup ok", "verification": "sent"})
}

func (h *CustomerHandler) sendVerificationCode(email, code string) {
	if h.Mailer != nil {
		if err := h.Mailer.SendVerificationCodeEmail(email, code); err != nil {
			slog.Error("customer signup mail send failed", "email", email, "error", err)
		}
	} else {
		slog.Info("customer signup verification code", "email", email, "code", code)
	}
}

func (h *CustomerHandler) sendPasswordResetEmail(email, link string) {
	if h.Mailer != nil {
		if err := h.Mailer.SendPasswordResetEmail(email, link); err != nil {
			slog.Error("customer forgot password mail send failed", "email", email, "error", err)
		}
	} else {
		slog.Info("customer forgot password reset link", "email", email, "link", link)
	}
}

func (h *CustomerHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
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

	customerID, err := h.customerIDFromSession(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	hash, err := h.repo.GetPasswordHash(r.Context(), customerID)
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
	if err := h.repo.UpdatePasswordHash(r.Context(), customerID, string(newHash)); err != nil {
		http.Error(w, "failed to update password", http.StatusInternalServerError)
		return
	}

	if c, err := r.Cookie("customer_session_token"); err == nil {
		currentHash := hashSessionToken(c.Value)
		if err := h.repo.DeleteOtherSessions(r.Context(), customerID, currentHash); err != nil {
			slog.Error("customer change password failed to invalidate other sessions", "error", err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "パスワードを変更しました"})
}

// メールアドレスが存在しない場合でも 200 を返す（ユーザー列挙攻撃の防止）。
func (h *CustomerHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var input apispec.ForgotPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil || strings.TrimSpace(string(input.Email)) == "" {
		http.Error(w, "email required", http.StatusBadRequest)
		return
	}

	ip := clientIP(r)
	if !h.rateLimiter.Allow("forgot:"+ip, rateLimitForgot, rateLimitWindow) {
		http.Error(w, "too many requests", http.StatusTooManyRequests)
		return
	}

	email := strings.TrimSpace(string(input.Email))
	customerID, err := h.repo.GetActiveIDByEmail(r.Context(), email)

	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		// アドレスが存在しない or DB エラーでも同じ応答（ユーザー列挙防止）
		json.NewEncoder(w).Encode(map[string]string{"message": "リセットリンクをメールで送信しました（登録済みの場合）"})
		return
	}

	plainToken, err := generateSessionToken()
	if err != nil {
		http.Error(w, "failed to generate token", http.StatusInternalServerError)
		return
	}
	tokenHash := hashSessionToken(plainToken)
	id := uuid.New().String()
	expiresAt := time.Now().Add(30 * time.Minute)
	if err := h.repo.CreatePasswordResetToken(r.Context(), id, customerID, tokenHash, expiresAt); err != nil {
		http.Error(w, "failed to save reset token", http.StatusInternalServerError)
		return
	}

	link := frontendURL() + "/reset-password?token=" + plainToken
	h.sendPasswordResetEmail(email, link)
	json.NewEncoder(w).Encode(map[string]string{"message": "リセットリンクをメールで送信しました（登録済みの場合）"})
}

func (h *CustomerHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var input apispec.ResetPasswordRequest
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
	reset, err := h.repo.GetPasswordResetByTokenHash(r.Context(), tokenHash)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "invalid token", http.StatusBadRequest)
		return
	} else if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	if reset.UsedAt.Valid {
		http.Error(w, "token already used", http.StatusBadRequest)
		return
	}
	if time.Now().After(reset.ExpiresAt) {
		http.Error(w, "token expired", http.StatusBadRequest)
		return
	}

	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "failed to hash password", http.StatusInternalServerError)
		return
	}

	if err := h.repo.ApplyPasswordReset(r.Context(), reset.CustomerID, reset.ID, string(hashedBytes)); err != nil {
		http.Error(w, "failed to reset password", http.StatusInternalServerError)
		return
	}

	if err := h.repo.DeleteAllSessions(r.Context(), reset.CustomerID); err != nil {
		slog.Error("customer reset password failed to invalidate sessions", "error", err)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "パスワードをリセットしました"})
}

func (h *CustomerHandler) customerIDFromSession(r *http.Request) (string, error) {
	c, err := r.Cookie("customer_session_token")
	if err != nil {
		return "", err
	}
	if h.repo == nil {
		return "", sql.ErrNoRows
	}
	return h.repo.GetCustomerIDByTokenHash(r.Context(), hashSessionToken(c.Value))
}
