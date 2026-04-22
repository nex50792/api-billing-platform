package handler

import (
	"api-billing-platform/internal/apispec"
	"api-billing-platform/internal/middleware"
	"api-billing-platform/internal/repository"
	"api-billing-platform/internal/service"
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"golang.org/x/crypto/bcrypt"
)

type AdminHandler struct {
	Svc         *service.AdminService
	db          *sql.DB
	repo        *repository.AdminRepository
	rateLimiter *RateLimiter
}

func NewAdminHandler(svc *service.AdminService) *AdminHandler {
	return &AdminHandler{Svc: svc, rateLimiter: NewRateLimiter()}
}

func NewAdminHandlerWithDB(svc *service.AdminService, db *sql.DB) *AdminHandler {
	var repo *repository.AdminRepository
	if db != nil {
		repo = repository.NewAdminRepository(db)
	}
	return &AdminHandler{Svc: svc, db: db, repo: repo, rateLimiter: NewRateLimiter()}
}

func (h *AdminHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req apispec.CredentialsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	admin, err := h.Svc.Authenticate(r.Context(), string(req.Email), req.Password)
	if err != nil || admin == nil {
		ip := clientIP(r)
		if !h.rateLimiter.Allow("admin_login:"+ip, rateLimitAdminLogin, rateLimitWindow) {
			http.Error(w, "too many failed login attempts", http.StatusTooManyRequests)
			return
		}
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	plainToken, err := h.Svc.CreateSession(r.Context(), admin.ID, 24*time.Hour)
	if err != nil {
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}
	setSessionCookie(w, "admin_session_token", plainToken, time.Now().Add(24*time.Hour))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "ok"})
}

func (h *AdminHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie("admin_session_token"); err == nil {
		_ = h.Svc.DeleteSession(r.Context(), c.Value)
	}
	clearSessionCookie(w, "admin_session_token")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "logged out"})
}

func (h *AdminHandler) GetCustomers(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		http.Error(w, "not configured", http.StatusInternalServerError)
		return
	}
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
	rows, err := h.repo.ListCustomers(r.Context(), limit, offset)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	customers := make([]apispec.AdminCustomer, 0, len(rows))
	for _, c := range rows {
		uid, _ := uuid.Parse(c.ID)
		customers = append(customers, apispec.AdminCustomer{
			Id:                 uid,
			Name:               c.Name,
			Email:              openapi_types.Email(c.Email),
			Plan:               c.Plan,
			SubscriptionStatus: c.SubscriptionStatus,
			IsActive:           c.IsActive,
			CreatedAt:          c.CreatedAt,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(customers)
}

func (h *AdminHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
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

	c, err := r.Cookie("admin_session_token")
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	admin, err := h.Svc.GetAdminBySession(r.Context(), c.Value)
	if err != nil || admin == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(input.CurrentPassword)); err != nil {
		http.Error(w, "current password is incorrect", http.StatusUnauthorized)
		return
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "failed to hash password", http.StatusInternalServerError)
		return
	}
	if h.repo == nil {
		http.Error(w, "not configured", http.StatusInternalServerError)
		return
	}
	if err := h.repo.UpdatePasswordHash(r.Context(), admin.ID, string(newHash)); err != nil {
		http.Error(w, "failed to update password", http.StatusInternalServerError)
		return
	}

	currentHash := hashSessionToken(c.Value)
	if err := h.repo.DeleteOtherSessions(r.Context(), admin.ID, currentHash); err != nil {
		middleware.LoggerFromContext(r.Context()).Error("admin change password failed to invalidate other sessions", "error", err)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "パスワードを変更しました"})
}

func (h *AdminHandler) GetEmailLogs(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		http.Error(w, "not configured", http.StatusInternalServerError)
		return
	}
	rows, err := h.repo.ListEmailLogs(r.Context())
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	logs := make([]apispec.EmailLog, 0, len(rows))
	for _, lr := range rows {
		uid, _ := uuid.Parse(lr.ID)
		log := apispec.EmailLog{
			Id:      uid,
			ToEmail: lr.ToEmail,
			Subject: lr.Subject,
			Status:  apispec.EmailLogStatus(lr.Status),
			SentAt:  lr.SentAt,
		}
		if lr.ErrorMessage != "" {
			msg := lr.ErrorMessage
			log.ErrorMessage = &msg
		}
		logs = append(logs, log)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(logs)
}

func (h *AdminHandler) Me(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("admin_session_token")
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	admin, err := h.Svc.GetAdminBySession(r.Context(), c.Value)
	if err != nil || admin == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(apispec.AdminMe{
		Id:    admin.ID,
		Email: openapi_types.Email(admin.Email),
		Name:  &admin.Name,
	})
}
