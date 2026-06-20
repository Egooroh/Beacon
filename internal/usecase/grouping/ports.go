package grouping

import (
	"context"
	"time"

	"github.com/Egooroh/beacon/internal/domain"
)

// EventRepository reads unprocessed events and marks them done.
type EventRepository interface {
	ListUnprocessed(ctx context.Context, limit int) ([]*domain.Event, error)
	// SetProcessed links an event to its issue and marks it as processed.
	SetProcessed(ctx context.Context, eventID, issueID string, fp domain.Fingerprint) error
}

// IssueRepository manages grouped incidents.
type IssueRepository interface {
	// Upsert creates a new issue or increments its event count.
	// created is true only on first occurrence of a fingerprint.
	Upsert(ctx context.Context, ev *domain.Event) (issue *domain.Issue, created bool, err error)
	TopByCount(ctx context.Context, projectID string, since time.Time, limit int) ([]*domain.Issue, error)
	CountInWindow(ctx context.Context, issueID string, window time.Duration) (int64, error)
	UpdateStatus(ctx context.Context, issueID string, s domain.IssueStatus) error
}

// Fingerprinter computes a deterministic fingerprint for an event.
type Fingerprinter interface {
	Compute(e *domain.Event) domain.Fingerprint
}

// Alerter dispatches notifications for significant issue state changes.
type Alerter interface {
	MaybeAlert(ctx context.Context, issue *domain.Issue, t domain.AlertType) error
}

// Clock is the time source; injectable for deterministic tests.
type Clock interface {
	Now() time.Time
}
