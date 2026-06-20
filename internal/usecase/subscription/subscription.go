package subscription

import (
	"context"
	"errors"
	"fmt"

	"github.com/Egooroh/beacon/internal/domain"
)

var validPlatforms = map[string]bool{"telegram": true, "slack": true}

// UseCase manages notification subscriptions for projects.
type UseCase struct {
	repo     Repository
	projects ProjectRepository
}

// New creates a subscription UseCase.
func New(repo Repository, projects ProjectRepository) *UseCase {
	return &UseCase{repo: repo, projects: projects}
}

// Subscribe adds a notification destination to a project.
func (uc *UseCase) Subscribe(ctx context.Context, projectID, platform, chatID string) (*domain.Subscription, error) {
	if !validPlatforms[platform] {
		return nil, fmt.Errorf("%w: unknown platform %q", domain.ErrInvalidInput, platform)
	}
	if chatID == "" {
		return nil, fmt.Errorf("%w: chat_id must not be empty", domain.ErrInvalidInput)
	}

	if _, err := uc.projects.FindByID(ctx, projectID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("find project: %w", err)
	}

	sub, err := uc.repo.Create(ctx, projectID, platform, chatID)
	if err != nil {
		return nil, fmt.Errorf("create subscription: %w", err)
	}
	return sub, nil
}

// List returns all notification destinations for a project.
func (uc *UseCase) List(ctx context.Context, projectID string) ([]*domain.Subscription, error) {
	subs, err := uc.repo.ListByProject(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("list subscriptions: %w", err)
	}
	return subs, nil
}
