package project

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/Egooroh/beacon/internal/domain"
)

// UseCase handles project lifecycle operations.
type UseCase struct {
	repo Repository
}

// New creates a project UseCase.
func New(repo Repository) *UseCase {
	return &UseCase{repo: repo}
}

// Create creates a new project with a freshly generated ingest token.
// Returns the project and the raw token — the caller must send the raw token to the client
// exactly once; only its hash is persisted.
func (uc *UseCase) Create(ctx context.Context, name string) (*domain.Project, string, error) {
	if name == "" {
		return nil, "", fmt.Errorf("%w: project name is required", domain.ErrInvalidInput)
	}
	rawToken, err := generateToken()
	if err != nil {
		return nil, "", fmt.Errorf("generate token: %w", err)
	}
	p, err := uc.repo.Create(ctx, name, domain.HashToken(rawToken))
	if err != nil {
		return nil, "", fmt.Errorf("create project: %w", err)
	}
	return p, rawToken, nil
}

// generateToken produces a 64-character cryptographically random hex string.
func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("rand: %w", err)
	}
	return hex.EncodeToString(b), nil
}
