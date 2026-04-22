package handler

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"time"

	"api-billing-platform/internal/middleware"
	"api-billing-platform/internal/repository"

	"github.com/stripe/stripe-go/v85/webhook"
)

type StripeWebhookHandler struct {
	customerRepo *repository.CustomerRepository
	eventsRepo   *repository.StripeEventsRepository
}

func NewStripeWebhookHandler(db *sql.DB) *StripeWebhookHandler {
	var cr *repository.CustomerRepository
	var er *repository.StripeEventsRepository
	if db != nil {
		cr = repository.NewCustomerRepository(db)
		er = repository.NewStripeEventsRepository(db)
	}
	return &StripeWebhookHandler{customerRepo: cr, eventsRepo: er}
}

func (h *StripeWebhookHandler) Handle(w http.ResponseWriter, r *http.Request) {
	log := middleware.LoggerFromContext(r.Context())
	webhookSecret := os.Getenv("STRIPE_WEBHOOK_SECRET")
	if webhookSecret == "" {
		log.Warn("STRIPE_WEBHOOK_SECRET not set")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	sigHeader := r.Header.Get("Stripe-Signature")
	event, err := webhook.ConstructEventWithOptions(payload, sigHeader, webhookSecret, webhook.ConstructEventOptions{
		IgnoreAPIVersionMismatch: true,
	})
	if err != nil {
		log.Error("stripe webhook signature verification failed", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// 冪等性: 同じ event_id はスキップ
	processed, err := h.eventsRepo.IsEventProcessed(r.Context(), event.ID)
	if err != nil {
		log.Error("failed to check stripe event processed", "event_id", event.ID, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if processed {
		log.Info("stripe event already processed, skipping", "event_id", event.ID)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		return
	}
	if err := h.eventsRepo.RecordEvent(r.Context(), event.ID); err != nil {
		log.Error("failed to record stripe event", "event_id", event.ID, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	switch event.Type {
	case "checkout.session.completed":
		log.Info("received stripe event", "type", event.Type)
		var sess struct {
			Customer          string `json:"customer"`
			ClientReferenceID string `json:"client_reference_id"`
			Subscription      string `json:"subscription"`
		}
		if err := json.Unmarshal(event.Data.Raw, &sess); err != nil {
			log.Error("failed to unmarshal checkout session", "error", err)
		} else {
			log.Info("stripe checkout session completed", "customer", sess.Customer, "client_reference_id", sess.ClientReferenceID, "subscription", sess.Subscription)
			customerID := sess.ClientReferenceID
			if customerID != "" {
				n, err := h.customerRepo.UpgradeToPro(r.Context(), customerID, sess.Customer)
				if err != nil {
					log.Error("failed to update customer plan", "error", err)
				} else {
					log.Info("customer upgraded to pro", "customer_id", customerID, "rows_affected", n)
				}
			} else {
				log.Warn("client_reference_id is empty, cannot update customer")
			}
		}

	case "customer.subscription.updated", "customer.subscription.created":
		log.Info("received stripe event", "type", event.Type)
		var sub struct {
			Customer          string `json:"customer"`
			Status            string `json:"status"`
			CancelAtPeriodEnd bool   `json:"cancel_at_period_end"`
			CancelAt          *int64 `json:"cancel_at"`
			CurrentPeriodEnd  int64  `json:"current_period_end"`
		}
		if err := json.Unmarshal(event.Data.Raw, &sub); err == nil {
			status := sub.Status
			var cancelAt *time.Time
			// cancel_at_period_end=true（旧パターン）または cancel_at に日時が入る（新パターン）の両方を「canceling」扱い。
			scheduledCancel := sub.CancelAtPeriodEnd || (sub.CancelAt != nil && *sub.CancelAt > 0)
			if scheduledCancel {
				status = "canceling"
				ts := sub.CurrentPeriodEnd
				if sub.CancelAt != nil && *sub.CancelAt > 0 {
					ts = *sub.CancelAt
				}
				if ts > 0 {
					t := time.Unix(ts, 0).UTC()
					cancelAt = &t
				}
			}
			log.Info("stripe subscription event", "customer", sub.Customer, "status", status, "cancel_at_period_end", sub.CancelAtPeriodEnd, "cancel_at", sub.CancelAt)
			if sub.Customer != "" && status != "" {
				if err := h.customerRepo.SetSubscriptionStatus(r.Context(), sub.Customer, status, cancelAt); err != nil {
					log.Error("failed to update subscription status", "error", err)
				}
			}
		}

	case "customer.subscription.deleted":
		log.Info("received stripe event", "type", event.Type)
		var sub struct {
			Customer string `json:"customer"`
		}
		if err := json.Unmarshal(event.Data.Raw, &sub); err == nil {
			if sub.Customer != "" {
				if err := h.customerRepo.Downgrade(r.Context(), sub.Customer); err != nil {
					log.Error("failed to downgrade customer", "error", err)
				}
			}
		}

	case "invoice.payment_failed":
		log.Info("received stripe event", "type", event.Type)
		var inv struct {
			Customer string `json:"customer"`
		}
		if err := json.Unmarshal(event.Data.Raw, &inv); err == nil {
			if inv.Customer != "" {
				if err := h.customerRepo.SetSubscriptionStatus(r.Context(), inv.Customer, "past_due", nil); err != nil {
					log.Error("failed to update subscription status", "error", err)
				}
			}
		}

	default:
		log.Warn("unhandled stripe event type", "type", event.Type)
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
