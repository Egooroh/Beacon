package issue

import (
	"context"
	"errors"
	"fmt"

	"github.com/Egooroh/beacon/internal/domain"
)

var validStatuses = map[domain.IssueStatus]bool{
	domain.StatusOpen:     true,
	domain.StatusResolved: true,
	domain.StatusMuted:    true,
	domain.StatusIgnored:  true,
}

// UseCase handles issue listing and lifecycle management.
type UseCase struct {
	repo Repository
}

// New creates an issue UseCase.
func New(repo Repository) *UseCase {
	return &UseCase{repo: repo}
}

// List returns issues for a project, optionally filtered by status.
// limit is clamped to [1, 100]; offset must be ≥ 0.
func (uc *UseCase) List(ctx context.Context, projectID, status string, limit, offset int) ([]*domain.Issue, int64, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	items, total, err := uc.repo.ListByProject(ctx, projectID, status, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list issues: %w", err)
	}
	return items, total, nil
}

// SetStatus changes the lifecycle status of an issue.
func (uc *UseCase) SetStatus(ctx context.Context, issueID string, newStatus domain.IssueStatus) error {
	if !validStatuses[newStatus] {
		return fmt.Errorf("%w: invalid status %q", domain.ErrInvalidInput, newStatus)
	}
	if _, err := uc.repo.FindByID(ctx, issueID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.ErrNotFound
		}
		return fmt.Errorf("find issue: %w", err)
	}
	if err := uc.repo.UpdateStatus(ctx, issueID, newStatus); err != nil {
		return fmt.Errorf("update status: %w", err)
	}
	return nil
}
