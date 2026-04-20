package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"api-billing-platform/internal/apispec"
	"api-billing-platform/internal/middleware"
)

func ProvidedAPICreateHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		providerID, _ := r.Context().Value(middleware.ContextKeyProviderID).(string)

		var input apispec.ProvidedAPIRequest
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			http.Error(w, "invalid input", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(input.Name) == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}
		id := uuid.New()
		now := time.Now()
		if _, err := db.ExecContext(r.Context(),
			`INSERT INTO provided_apis (id, provider_id, name, description, documentation, created_at) VALUES ($1, $2, $3, $4, $5, $6)`,
			id.String(), providerID, input.Name, input.Description, input.Documentation, now,
		); err != nil {
			http.Error(w, "failed to insert", http.StatusInternalServerError)
			return
		}
		api := apispec.ProvidedAPI{
			Id:            id,
			ProviderId:    &providerID,
			Name:          input.Name,
			Description:   input.Description,
			Documentation: input.Documentation,
			CreatedAt:     now,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(api)
	}
}

func ProvidedAPIUpdateHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		providerID, _ := r.Context().Value(middleware.ContextKeyProviderID).(string)

		var input apispec.ProvidedAPIRequest
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			http.Error(w, "invalid input", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(input.Name) == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}
		res, err := db.ExecContext(r.Context(),
			`UPDATE provided_apis SET name=$1, description=$2, documentation=$3 WHERE id=$4 AND provider_id=$5`,
			input.Name, input.Description, input.Documentation, id, providerID,
		)
		if err != nil {
			http.Error(w, "failed to update", http.StatusInternalServerError)
			return
		}
		if n, _ := res.RowsAffected(); n == 0 {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		apiID, _ := uuid.Parse(id)
		api := apispec.ProvidedAPI{
			Id:            apiID,
			ProviderId:    &providerID,
			Name:          input.Name,
			Description:   input.Description,
			Documentation: input.Documentation,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(api)
	}
}

func ProvidedAPIDeleteHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		providerID, _ := r.Context().Value(middleware.ContextKeyProviderID).(string)

		res, err := db.ExecContext(r.Context(),
			`DELETE FROM provided_apis WHERE id=$1 AND provider_id=$2`,
			id, providerID,
		)
		if err != nil {
			http.Error(w, "failed to delete", http.StatusInternalServerError)
			return
		}
		if n, _ := res.RowsAffected(); n == 0 {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func ProvidedAPIUsageHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		providerID, _ := r.Context().Value(middleware.ContextKeyProviderID).(string)

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

		rows, err := db.QueryContext(r.Context(),
			`SELECT pa.id, pa.name, c.id, c.name, c.email, ak.usage_count
			 FROM api_keys ak
			 JOIN provided_apis pa ON ak.provided_api_id = pa.id
			 JOIN customers c ON ak.customer_id = c.id
			 WHERE pa.provider_id = $1
			 ORDER BY pa.name, ak.usage_count DESC
			 LIMIT $2 OFFSET $3`,
			providerID, limit, offset,
		)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		usage := []apispec.UsageRow{}
		for rows.Next() {
			var apiID, apiName, custID, custName, custEmail string
			var usageCount int
			if err := rows.Scan(&apiID, &apiName, &custID, &custName, &custEmail, &usageCount); err != nil {
				continue
			}
			apiUUID, _ := uuid.Parse(apiID)
			custUUID, _ := uuid.Parse(custID)
			usage = append(usage, apispec.UsageRow{
				ApiId:         apiUUID,
				ApiName:       apiName,
				CustomerId:    custUUID,
				CustomerName:  custName,
				CustomerEmail: custEmail,
				UsageCount:    usageCount,
			})
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(usage)
	}
}

// provider_id は Customer に不要な情報のため含めない。
func ProvidedAPIListHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.QueryContext(r.Context(),
			`SELECT id, name, description, documentation, created_at FROM provided_apis ORDER BY created_at`,
		)
		if err != nil {
			http.Error(w, "failed to fetch provided apis", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		apis := []apispec.ProvidedAPIPublic{}
		for rows.Next() {
			var id, name, description string
			var createdAt scanTime
			var doc sql.NullString
			if err := rows.Scan(&id, &name, &description, &doc, &createdAt); err != nil {
				http.Error(w, "failed to scan provided api", http.StatusInternalServerError)
				return
			}
			apiID, _ := uuid.Parse(id)
			api := apispec.ProvidedAPIPublic{
				Id:          apiID,
				Name:        name,
				Description: description,
				CreatedAt:   createdAt.Time,
			}
			if doc.Valid {
				api.Documentation = &doc.String
			}
			apis = append(apis, api)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(apis)
	}
}

func ProvidedAPIProviderListHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		providerID, _ := r.Context().Value(middleware.ContextKeyProviderID).(string)

		rows, err := db.QueryContext(r.Context(),
			`SELECT pa.id, COALESCE(pa.provider_id, ''), pa.name, pa.description, pa.documentation, pa.created_at,
			        COUNT(ak.id),
			        COALESCE(SUM(ak.usage_count), 0)
			 FROM provided_apis pa
			 LEFT JOIN api_keys ak ON ak.provided_api_id = pa.id
			 WHERE pa.provider_id = $1
			 GROUP BY pa.id
			 ORDER BY pa.created_at`,
			providerID,
		)
		if err != nil {
			http.Error(w, "failed to fetch provided apis", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		apis := []apispec.ProvidedAPI{}
		for rows.Next() {
			var id, providerIDStr, name, description string
			var createdAt scanTime
			var doc sql.NullString
			var keyCount, totalUsage int
			if err := rows.Scan(&id, &providerIDStr, &name, &description, &doc, &createdAt, &keyCount, &totalUsage); err != nil {
				http.Error(w, "failed to scan provided api", http.StatusInternalServerError)
				return
			}
			apiID, _ := uuid.Parse(id)
			api := apispec.ProvidedAPI{
				Id:          apiID,
				ProviderId:  &providerIDStr,
				Name:        name,
				Description: description,
				CreatedAt:   createdAt.Time,
				KeyCount:    &keyCount,
				TotalUsage:  &totalUsage,
			}
			if doc.Valid {
				api.Documentation = &doc.String
			}
			apis = append(apis, api)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(apis)
	}
}
