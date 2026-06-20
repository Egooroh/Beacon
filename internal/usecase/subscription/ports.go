package subscription

import (
	"context"

	"github.com/Egooroh/beacon/internal/domain"
)

// Repository persists and reads subscriptions.
type Repository interface {
	Create(ctx context.Context, projectID, platform, chatID string) (*domain.Subscription, error)
	ListByProject(ctx context.Context, projectID string) ([]*domain.Subscription, error)
}

// ProjectRepository verifies that the project exists before subscribing.
type ProjectRepository interface {
	FindByID(ctx context.Context, id string) (*domain.Project, error)
}
