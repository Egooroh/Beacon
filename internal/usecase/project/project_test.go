package project_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Egooroh/beacon/internal/domain"
	"github.com/Egooroh/beacon/internal/usecase/project"
)

type fakeRepo struct{ created *domain.Project }

func (f *fakeRepo) Create(_ context.Context, name, tokenHash string) (*domain.Project, error) {
	f.created = &domain.Project{ID: "proj-uuid", Name: name, TokenHash: tokenHash}
	return f.created, nil
}

func TestCreate_ReturnsProjectAndRawToken(t *testing.T) {
	repo := &fakeRepo{}
	uc := project.New(repo)

	p, rawToken, err := uc.Create(context.Background(), "my-service")
	require.NoError(t, err)

	assert.Equal(t, "my-service", p.Name)
	assert.NotEmpty(t, rawToken)
	// Only the hash must be stored; raw token must never reach the repo.
	assert.Equal(t, domain.HashToken(rawToken), p.TokenHash)
	assert.NotEqual(t, rawToken, p.TokenHash, "raw token must not equal its own hash")
}

func TestCreate_TwoCallsProduceDifferentTokens(t *testing.T) {
	uc := project.New(&fakeRepo{})

	_, t1, err := uc.Create(context.Background(), "svc-a")
	require.NoError(t, err)
	_, t2, err := uc.Create(context.Background(), "svc-b")
	require.NoError(t, err)

	assert.NotEqual(t, t1, t2)
}

func TestCreate_EmptyName_ReturnsInvalidInput(t *testing.T) {
	uc := project.New(&fakeRepo{})

	_, _, err := uc.Create(context.Background(), "")

	assert.ErrorIs(t, err, domain.ErrInvalidInput)
}
