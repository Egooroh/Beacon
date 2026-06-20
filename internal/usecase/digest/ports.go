package digest

import (
	"context"
	"time"

	"github.com/Egooroh/beacon/internal/domain"
)

// IssueRepository provides the top-N query for digest reports.
type IssueRepository interface {
	TopByCount(ctx context.Context, projectID string, since time.Time, limit int) ([]*domain.Issue, error)
}

// SubscriptionRepository resolves which projects have subscribers and where to send.
type SubscriptionRepository interface {
	ListProjectsWithSubscriptions(ctx context.Context) ([]string, error)
	ListByProject(ctx context.Context, projectID string) ([]*domain.Subscription, error)
}

// ProjectRepository fetches project metadata for rendering the digest header.
type ProjectRepository interface {
	FindByID(ctx context.Context, id string) (*domain.Project, error)
}

// Notifier sends a pre-formatted text message to a single chat destination.
type Notifier interface {
	Send(ctx context.Context, chatID, text string) error
	Platform() string
}

// Clock is the time source; injectable for tests.
type Clock interface {
	Now() time.Time
}
