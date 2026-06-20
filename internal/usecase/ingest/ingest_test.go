package ingest_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Egooroh/beacon/internal/domain"
	"github.com/Egooroh/beacon/internal/usecase/ingest"
)

// ── fakes ────────────────────────────────────────────────────────────────────

type fakeEventRepo struct{ saved []*domain.Event }

func (f *fakeEventRepo) Save(_ context.Context, e *domain.Event) error {
	f.saved = append(f.saved, e)
	return nil
}

type fakeProjectRepo struct {
	project *domain.Project
	err     error
}

func (f *fakeProjectRepo) FindByTokenHash(_ context.Context, _ string) (*domain.Project, error) {
	return f.project, f.err
}

type fakeParser struct {
	event *domain.Event
	err   error
}

func (f *fakeParser) Parse(_ []byte) (*domain.Event, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.event, nil
}

type fakeClock struct{ t time.Time }

func (c *fakeClock) Now() time.Time { return c.t }

// ── helpers ───────────────────────────────────────────────────────────────────

func validProject() *domain.Project {
	return &domain.Project{ID: "proj-1", TokenHash: domain.HashToken("valid-token")}
}

func validEvent() *domain.Event {
	return &domain.Event{Level: domain.LevelError, Message: "something broke"}
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestAccept_SavesEventWithProjectAndTimestamp(t *testing.T) {
	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	events := &fakeEventRepo{}
	uc := ingest.New(
		events,
		&fakeProjectRepo{project: validProject()},
		&fakeParser{event: validEvent()},
		&fakeClock{t: now},
	)

	err := uc.Accept(context.Background(), "valid-token", []byte(`{"message":"test"}`))

	require.NoError(t, err)
	require.Len(t, events.saved, 1)
	assert.Equal(t, "proj-1", events.saved[0].ProjectID)
	assert.Equal(t, now, events.saved[0].ReceivedAt)
	assert.Equal(t, []byte(`{"message":"test"}`), events.saved[0].RawPayload)
}

func TestAccept_InvalidToken_ReturnsUnauthorized(t *testing.T) {
	events := &fakeEventRepo{}
	uc := ingest.New(
		events,
		&fakeProjectRepo{err: domain.ErrNotFound},
		&fakeParser{event: validEvent()},
		&fakeClock{},
	)

	err := uc.Accept(context.Background(), "bad-token", []byte(`{}`))

	assert.ErrorIs(t, err, domain.ErrUnauthorized)
	assert.Empty(t, events.saved)
}

func TestAccept_BadPayload_ReturnsInvalidInput(t *testing.T) {
	events := &fakeEventRepo{}
	uc := ingest.New(
		events,
		&fakeProjectRepo{project: validProject()},
		&fakeParser{err: errors.New("missing message field")},
		&fakeClock{},
	)

	err := uc.Accept(context.Background(), "valid-token", []byte(`{}`))

	assert.ErrorIs(t, err, domain.ErrInvalidInput)
	assert.Empty(t, events.saved)
}

func TestAccept_StorageFailure_PropagatesError(t *testing.T) {
	storageErr := errors.New("db unavailable")
	uc := ingest.New(
		&errEventRepo{err: storageErr},
		&fakeProjectRepo{project: validProject()},
		&fakeParser{event: validEvent()},
		&fakeClock{},
	)

	err := uc.Accept(context.Background(), "valid-token", []byte(`{}`))

	require.Error(t, err)
	assert.ErrorIs(t, err, storageErr)
}

type errEventRepo struct{ err error }

func (e *errEventRepo) Save(_ context.Context, _ *domain.Event) error { return e.err }
