package pgstore

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Egooroh/beacon/internal/domain"
)

// EventRepository stores events in PostgreSQL.
type EventRepository struct {
	db *pgxpool.Pool
}

// NewEventRepository creates an EventRepository backed by the given pool.
func NewEventRepository(db *pgxpool.Pool) *EventRepository {
	return &EventRepository{db: db}
}

// Save inserts a raw event and populates e.ID with the database-generated UUID.
func (r *EventRepository) Save(ctx context.Context, e *domain.Event) error {
	const q = `
		INSERT INTO events (project_id, level, message, environment, release, payload, received_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id`

	if err := r.db.QueryRow(ctx, q,
		e.ProjectID,
		string(e.Level),
		e.Message,
		e.Environment,
		e.Release,
		e.RawPayload,
		e.ReceivedAt,
	).Scan(&e.ID); err != nil {
		return fmt.Errorf("insert event: %w", err)
	}
	return nil
}
