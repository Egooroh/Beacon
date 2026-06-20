package ingest

import (
	"context"
	"time"

	"github.com/Egooroh/beacon/internal/domain"
)

// EventRepository persists raw ingest events before they are processed.
type EventRepository interface {
	Save(ctx context.Context, e *domain.Event) error
}

// ProjectRepository looks up projects by their stored token hash.
type ProjectRepository interface {
	FindByTokenHash(ctx context.Context, tokenHash string) (*domain.Project, error)
}

// PayloadParser converts a raw JSON payload into a domain Event.
type PayloadParser interface {
	Parse(raw []byte) (*domain.Event, error)
}

// Clock is the time source; injectable for deterministic tests.
type Clock interface {
	Now() time.Time
}
