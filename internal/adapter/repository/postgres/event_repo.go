package pgstore

import (
	"context"
	"fmt"
	"time"

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

// ListUnprocessed returns up to limit events not yet assigned to an issue.
func (r *EventRepository) ListUnprocessed(ctx context.Context, limit int) ([]*domain.Event, error) {
	const q = `
		SELECT id, project_id, level, message, environment, release, payload, received_at
		FROM events
		WHERE processed = false
		ORDER BY received_at
		LIMIT $1`

	rows, err := r.db.Query(ctx, q, limit)
	if err != nil {
		return nil, fmt.Errorf("list unprocessed: %w", err)
	}
	defer rows.Close()

	var events []*domain.Event
	for rows.Next() {
		var (
			ev       = &domain.Event{}
			levelStr string
			rcvAt    time.Time
		)
		if err := rows.Scan(
			&ev.ID, &ev.ProjectID, &levelStr,
			&ev.Message, &ev.Environment, &ev.Release,
			&ev.RawPayload, &rcvAt,
		); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		ev.Level = domain.Level(levelStr)
		ev.ReceivedAt = rcvAt
		events = append(events, ev)
	}
	return events, rows.Err()
}

// SetProcessed links an event to its issue and marks it as processed atomically.
func (r *EventRepository) SetProcessed(ctx context.Context, eventID, issueID string, fp domain.Fingerprint) error {
	const q = `
		UPDATE events
		SET issue_id = $2, fingerprint = $3, processed = true
		WHERE id = $1`

	if _, err := r.db.Exec(ctx, q, eventID, issueID, string(fp)); err != nil {
		return fmt.Errorf("set processed: %w", err)
	}
	return nil
}
