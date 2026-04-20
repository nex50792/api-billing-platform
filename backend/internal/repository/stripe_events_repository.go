package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type StripeEventsRepository struct {
	DB *sql.DB
}

func NewStripeEventsRepository(db *sql.DB) *StripeEventsRepository {
	return &StripeEventsRepository{DB: db}
}

func (r *StripeEventsRepository) IsEventProcessed(ctx context.Context, eventID string) (bool, error) {
	var dummy string
	err := r.DB.QueryRowContext(ctx,
		`SELECT event_id FROM stripe_events WHERE event_id = $1`, eventID,
	).Scan(&dummy)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("repository.IsEventProcessed: %w", err)
	}
	return true, nil
}

func (r *StripeEventsRepository) RecordEvent(ctx context.Context, eventID string) error {
	_, err := r.DB.ExecContext(ctx, `INSERT INTO stripe_events (event_id) VALUES ($1)`, eventID)
	if err != nil {
		return fmt.Errorf("repository.RecordEvent: %w", err)
	}
	return nil
}
