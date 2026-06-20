package project

import (
	"context"

	"github.com/Egooroh/beacon/internal/domain"
)

// Repository persists and retrieves projects.
type Repository interface {
	Create(ctx context.Context, name, tokenHash string) (*domain.Project, error)
}
