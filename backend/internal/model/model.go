package model

import (
	"time"
)

type Customer struct {
	ID                 string    `db:"id" json:"id"`
	Name               string    `db:"name" json:"name"`
	Email              string    `db:"email" json:"email"`
	Plan               string    `db:"plan" json:"plan"`
	StripeCustomerID   *string   `db:"stripe_customer_id" json:"stripe_customer_id,omitempty"`
	SubscriptionStatus *string   `db:"subscription_status" json:"subscription_status,omitempty"`
	CreatedAt          time.Time `db:"created_at" json:"created_at"`
}

type APIKey struct {
	ID            string    `db:"id" json:"id"`
	CustomerID    string    `db:"customer_id" json:"customer_id"`
	ProvidedAPIID *string   `db:"provided_api_id" json:"provided_api_id,omitempty"`
	UsageCount    int       `db:"usage_count" json:"usage_count"`
	UsageResetAt  time.Time `db:"usage_reset_at" json:"usage_reset_at"`
	CreatedAt     time.Time `db:"created_at" json:"created_at"`
}
