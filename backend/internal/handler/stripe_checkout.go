package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"time"

	"api-billing-platform/internal/apispec"
	"api-billing-platform/internal/middleware"
	"api-billing-platform/internal/repository"

	"github.com/stripe/stripe-go/v85"
	billingportalsession "github.com/stripe/stripe-go/v85/billingportal/session"
	checkoutsession "github.com/stripe/stripe-go/v85/checkout/session"
	"github.com/stripe/stripe-go/v85/customer"
)

type StripeCheckoutHandler struct {
	repo *repository.CustomerRepository
}

func NewStripeCheckoutHandler(db *sql.DB) *StripeCheckoutHandler {
	var repo *repository.CustomerRepository
	if db != nil {
		repo = repository.NewCustomerRepository(db)
	}
	return &StripeCheckoutHandler{repo: repo}
}

func (h *StripeCheckoutHandler) customerIDFromSession(r *http.Request) (string, error) {
	c, err := r.Cookie("customer_session_token")
	if err != nil {
		return "", err
	}
	if h.repo == nil {
		return "", sql.ErrNoRows
	}
	return h.repo.GetCustomerIDByTokenHash(r.Context(), hashSessionToken(c.Value))
}

func (h *StripeCheckoutHandler) CreateCheckoutSession(w http.ResponseWriter, r *http.Request) {
	customerID, err := h.customerIDFromSession(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	info, err := h.repo.GetStripeInfo(r.Context(), customerID)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "customer not found", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	if info.Plan == "pro" {
		http.Error(w, "already on pro plan", http.StatusConflict)
		return
	}

	if stripe.Key == "" {
		http.Error(w, "Stripe not configured", http.StatusInternalServerError)
		return
	}

	var stripeCustomer string
	if info.StripeCustomerID.Valid && info.StripeCustomerID.String != "" {
		stripeCustomer = info.StripeCustomerID.String
	} else {
		params := &stripe.CustomerParams{
			Name:  stripe.String(info.Name),
			Email: stripe.String(info.Email),
		}
		params.AddMetadata("customer_id", customerID)
		c, err := customer.New(params)
		if err != nil {
			middleware.LoggerFromContext(r.Context()).Error("stripe checkout failed to create customer", "error", err)
			http.Error(w, "failed to create stripe customer", http.StatusInternalServerError)
			return
		}
		stripeCustomer = c.ID
		if err := h.repo.SetStripeCustomerID(r.Context(), customerID, stripeCustomer); err != nil {
			middleware.LoggerFromContext(r.Context()).Error("stripe checkout failed to save customer id", "error", err)
		}
	}

	fURL := frontendURL()
	priceID := os.Getenv("STRIPE_PRO_PRICE_ID")
	var lineItems []*stripe.CheckoutSessionLineItemParams
	if priceID != "" {
		lineItems = []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(priceID),
				Quantity: stripe.Int64(1),
			},
		}
	} else {
		lineItems = []*stripe.CheckoutSessionLineItemParams{
			{
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency: stripe.String("jpy"),
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name:        stripe.String("Pro Plan"),
						Description: stripe.String("API利用量無制限のProプラン"),
					},
					UnitAmount: stripe.Int64(1000),
					Recurring: &stripe.CheckoutSessionLineItemPriceDataRecurringParams{
						Interval: stripe.String("month"),
					},
				},
				Quantity: stripe.Int64(1),
			},
		}
	}

	checkoutParams := &stripe.CheckoutSessionParams{
		Customer:          stripe.String(stripeCustomer),
		Mode:              stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		LineItems:         lineItems,
		SuccessURL:        stripe.String(fURL + "/?checkout=success"),
		CancelURL:         stripe.String(fURL + "/?checkout=cancel"),
		ClientReferenceID: stripe.String(customerID),
	}
	// 連打による重複セッション作成だけを吸収する。分粒度なので期限切れ session を翌回まで引きずらない。
	idempotencyKey := "checkout-" + customerID + "-" + time.Now().UTC().Format("200601021504")
	checkoutParams.SetIdempotencyKey(idempotencyKey)

	sess, err := checkoutsession.New(checkoutParams)
	if err != nil {
		middleware.LoggerFromContext(r.Context()).Error("stripe checkout failed to create session", "error", err)
		http.Error(w, "failed to create checkout session", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(apispec.CheckoutSessionResponse{
		CheckoutUrl: sess.URL,
		SessionId:   sess.ID,
	})
}

func (h *StripeCheckoutHandler) CreateBillingPortalSession(w http.ResponseWriter, r *http.Request) {
	customerID, err := h.customerIDFromSession(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	stripeCustomerID, err := h.repo.GetStripeCustomerID(r.Context(), customerID)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "customer not found", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	if !stripeCustomerID.Valid || stripeCustomerID.String == "" {
		http.Error(w, "no stripe customer", http.StatusBadRequest)
		return
	}

	if stripe.Key == "" {
		http.Error(w, "Stripe not configured", http.StatusInternalServerError)
		return
	}

	params := &stripe.BillingPortalSessionParams{
		Customer:  stripe.String(stripeCustomerID.String),
		ReturnURL: stripe.String(frontendURL() + "/"),
	}
	sess, err := billingportalsession.New(params)
	if err != nil {
		middleware.LoggerFromContext(r.Context()).Error("billing portal failed to create session", "error", err)
		http.Error(w, "failed to create billing portal session", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(apispec.BillingPortalResponse{PortalUrl: sess.URL})
}

func (h *StripeCheckoutHandler) GetSubscription(w http.ResponseWriter, r *http.Request) {
	customerID, err := h.customerIDFromSession(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	sub, err := h.repo.GetSubscription(r.Context(), customerID)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "customer not found", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	resp := apispec.SubscriptionResponse{Plan: apispec.SubscriptionResponsePlan(sub.Plan)}
	if sub.SubscriptionStatus.Valid {
		resp.SubscriptionStatus = &sub.SubscriptionStatus.String
	}
	if sub.StripeCustomerID.Valid {
		resp.StripeCustomerId = &sub.StripeCustomerID.String
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
