package issue

import (
	"context"

	"github.com/Egooroh/beacon/internal/domain"
)

// Repository is the persistence slice needed by the issue use case.
type Repository interface {
	FindByID(ctx context.Context, id string) (*domain.Issue, error)
	ListByProject(ctx context.Context, projectID, status string, limit, offset int) ([]*domain.Issue, int64, error)
	UpdateStatus(ctx context.Context, id string, s domain.IssueStatus) error
}
