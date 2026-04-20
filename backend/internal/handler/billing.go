package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"api-billing-platform/internal/apispec"
	"api-billing-platform/internal/middleware"
	"api-billing-platform/internal/model"
	"api-billing-platform/internal/repository"

	"github.com/google/uuid"
)

type BillingHandler struct {
	apiKeyRepo repository.APIKeyRepositoryInterface
	db         *sql.DB
}

func NewBillingHandler(apiKeyRepo repository.APIKeyRepositoryInterface, db *sql.DB) *BillingHandler {
	return &BillingHandler{apiKeyRepo: apiKeyRepo, db: db}
}

func (h *BillingHandler) Check(w http.ResponseWriter, r *http.Request) {
	apiKeyVal := r.Context().Value(middleware.ContextKeyAPIKey)
	apiKey, ok := apiKeyVal.(*model.APIKey)
	if !ok || apiKey == nil {
		http.Error(w, "api key not found", http.StatusUnauthorized)
		return
	}

	cust, err := h.apiKeyRepo.GetCustomerByAPIKey(r.Context(), apiKey.ID)
	if err != nil || cust == nil {
		http.Error(w, "customer not found", http.StatusUnauthorized)
		return
	}

	custUUID, _ := uuid.Parse(cust.ID)
	keyUUID, _ := uuid.Parse(apiKey.ID)

	var resp apispec.BillingCheckResponse
	resp.Customer.Id = custUUID
	resp.Customer.Plan = cust.Plan
	resp.Customer.SubscriptionStatus = cust.SubscriptionStatus
	resp.ApiKey.Id = keyUUID
	resp.ApiKey.UsageCount = apiKey.UsageCount
	resp.ApiKey.UsageResetAt = apiKey.UsageResetAt

	var pid sql.NullString
	var name sql.NullString
	err = h.db.QueryRowContext(r.Context(),
		`SELECT p.id, p.name FROM provided_apis p JOIN api_keys k ON k.provided_api_id = p.id WHERE k.id = $1`,
		apiKey.ID,
	).Scan(&pid, &name)
	if err == nil && pid.Valid {
		papiUUID, _ := uuid.Parse(pid.String)
		resp.ProvidedApi = &struct {
			Id   uuid.UUID `json:"id"`
			Name string    `json:"name"`
		}{Id: papiUUID, Name: name.String}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
