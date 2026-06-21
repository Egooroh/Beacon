package alerting

import (
	"context"
	"time"

	"github.com/Egooroh/beacon/internal/domain"
)

// Notifier sends a formatted alert to a single destination.
// chatID is platform-specific (Telegram chat_id, Slack channel ID, …).
type Notifier interface {
	Notify(ctx context.Context, alert domain.Alert, chatID string) error
	// Platform returns the identifier matched against Subscription.Platform.
	Platform() string
}

// IssueRepository is the slice of persistence operations needed by alerting.
type IssueRepository interface {
	UpdateLastAlertAt(ctx context.Context, issueID string, t time.Time) error
	// ListOpen returns up to limit open issues across all projects (spike scan).
	ListOpen(ctx context.Context, limit int) ([]*domain.Issue, error)
	CountInWindow(ctx context.Context, issueID string, window time.Duration) (int64, error)
}

// ProjectRepository fetches project metadata for alert rendering.
type ProjectRepository interface {
	FindByID(ctx context.Context, id string) (*domain.Project, error)
}

// SubscriptionRepository resolves where to send alerts for a project.
type SubscriptionRepository interface {
	ListByProject(ctx context.Context, projectID string) ([]*domain.Subscription, error)
}

// Clock is the time source; injectable for tests.
type Clock interface {
	Now() time.Time
}

// MetricsRecorder captures alerting metrics. A nil value disables recording.
type MetricsRecorder interface {
	RecordAlertSent(alertType string)
}
