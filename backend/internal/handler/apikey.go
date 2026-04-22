package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"api-billing-platform/internal/apispec"
	"api-billing-platform/internal/middleware"
	"api-billing-platform/internal/repository"
	"api-billing-platform/internal/service"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (h *APIKeyHandler) VerifyAPIKey(w http.ResponseWriter, r *http.Request) {
	providerID, ok := r.Context().Value(middleware.ContextKeyProviderID).(string)
	if !ok || providerID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req apispec.VerifyAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	providedAPIID := req.ProvidedApiId.String()
	if strings.TrimSpace(req.ApiKey) == "" || strings.TrimSpace(providedAPIID) == "" {
		http.Error(w, "api_key and provided_api_id are required", http.StatusBadRequest)
		return
	}

	result, err := h.service.VerifyAPIKey(r.Context(), req.ApiKey, providedAPIID, providerID)
	if err != nil {
		if errors.Is(err, repository.ErrRateLimited) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(apispec.RateLimitError{
				Valid:   false,
				Error:   "rate_limit_exceeded",
				Message: "月間リクエスト上限に達しました。Proプランへのアップグレードをご検討ください。",
			})
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if result == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(apispec.VerifyAPIKeyInvalidResponse{Valid: false})
		return
	}

	customerUUID, _ := uuid.Parse(result.CustomerID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(apispec.VerifyAPIKeyResponse{
		Valid:      true,
		CustomerId: customerUUID,
		UsageCount: result.UsageCount,
	})
}

type APIKeyHandler struct {
	service service.APIKeyServiceInterface
	db      *sql.DB
}

func NewAPIKeyHandler(s service.APIKeyServiceInterface, db *sql.DB) *APIKeyHandler {
	return &APIKeyHandler{service: s, db: db}
}

func (h *APIKeyHandler) GetCustomerAPIKeys(w http.ResponseWriter, r *http.Request) {
	customerID, ok := r.Context().Value(middleware.ContextKeyCustomerID).(string)
	if !ok || customerID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	keys, err := h.service.ListCustomerKeys(r.Context(), customerID)
	if err != nil {
		http.Error(w, "failed to fetch api keys", http.StatusInternalServerError)
		return
	}
	result := make([]apispec.APIKey, 0, len(keys))
	for _, k := range keys {
		keyID, _ := uuid.Parse(k.ID)
		custUUID, _ := uuid.Parse(k.CustomerID)
		item := apispec.APIKey{
			Id:           keyID,
			CustomerId:   custUUID,
			UsageCount:   k.UsageCount,
			UsageResetAt: k.UsageResetAt,
			CreatedAt:    k.CreatedAt,
		}
		if k.ProvidedAPIID != nil {
			if p, err := uuid.Parse(*k.ProvidedAPIID); err == nil {
				item.ProvidedApiId = &p
			}
		}
		result = append(result, item)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (h *APIKeyHandler) DeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	customerID, ok := r.Context().Value(middleware.ContextKeyCustomerID).(string)
	if !ok || customerID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	keyID := chi.URLParam(r, "id")
	if keyID == "" {
		http.Error(w, "missing key id", http.StatusBadRequest)
		return
	}
	err := h.service.DeleteAPIKey(r.Context(), keyID, customerID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *APIKeyHandler) IssueAPIKey(w http.ResponseWriter, r *http.Request) {
	customerID, ok := r.Context().Value(middleware.ContextKeyCustomerID).(string)
	if !ok || customerID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req apispec.IssueAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	var providedAPIID *string
	if req.ProvidedApiId != nil {
		s := req.ProvidedApiId.String()
		providedAPIID = &s
	}
	plainKey, apiKey, err := h.service.IssueAPIKey(r.Context(), customerID, providedAPIID)
	if err != nil {
		if errors.Is(err, repository.ErrAPIKeyLimitExceeded) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(apispec.APIKeyLimitError{
				Error:   "api_key_limit_exceeded",
				Message: "プランの API キー発行上限に達しました。Proプランへのアップグレードをご検討ください。",
			})
			return
		}
		http.Error(w, "failed to issue api key", http.StatusInternalServerError)
		return
	}
	id, _ := uuid.Parse(apiKey.ID)
	resp := apispec.IssueAPIKeyResponse{ApiKey: plainKey, Id: id}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
