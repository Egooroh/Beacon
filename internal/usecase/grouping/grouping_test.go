package grouping_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Egooroh/beacon/internal/domain"
	"github.com/Egooroh/beacon/internal/usecase/grouping"
)

// ── fakes ────────────────────────────────────────────────────────────────────

type fakeEventRepo struct {
	events    []*domain.Event
	processed []string
	listErr   error
	setErr    error
}

func (f *fakeEventRepo) ListUnprocessed(_ context.Context, limit int) ([]*domain.Event, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	if len(f.events) < limit {
		return f.events, nil
	}
	return f.events[:limit], nil
}

func (f *fakeEventRepo) SetProcessed(_ context.Context, eventID, _ string, _ domain.Fingerprint) error {
	if f.setErr != nil {
		return f.setErr
	}
	f.processed = append(f.processed, eventID)
	return nil
}

type fakeIssueRepo struct {
	issue   *domain.Issue
	created bool
	err     error
	calls   int
}

func (f *fakeIssueRepo) Upsert(_ context.Context, _ *domain.Event) (*domain.Issue, bool, error) {
	f.calls++
	return f.issue, f.created, f.err
}

func (f *fakeIssueRepo) TopByCount(_ context.Context, _ string, _ time.Time, _ int) ([]*domain.Issue, error) {
	return nil, nil
}

func (f *fakeIssueRepo) CountInWindow(_ context.Context, _ string, _ time.Duration) (int64, error) {
	return 0, nil
}

func (f *fakeIssueRepo) UpdateStatus(_ context.Context, _ string, _ domain.IssueStatus) error {
	return nil
}

type fakeFingerprinter struct{ fp domain.Fingerprint }

func (f *fakeFingerprinter) Compute(_ *domain.Event) domain.Fingerprint { return f.fp }

type fakeClock struct{ t time.Time }

func (c *fakeClock) Now() time.Time { return c.t }

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestProcessBatch_CreatesNewIssue(t *testing.T) {
	events := &fakeEventRepo{
		events: []*domain.Event{{ID: "ev-1", ProjectID: "p-1", Message: "oops"}},
	}
	issue := &domain.Issue{ID: "issue-1"}
	issues := &fakeIssueRepo{issue: issue, created: true}
	fp := fakeFingerprinter{fp: "abc123"}

	uc := grouping.New(events, issues, &fp, &fakeClock{}, discardLogger(), 10)
	err := uc.ProcessBatch(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 1, issues.calls)
	assert.Equal(t, []string{"ev-1"}, events.processed)
}

func TestProcessBatch_EmptyQueue_DoesNothing(t *testing.T) {
	events := &fakeEventRepo{}
	issues := &fakeIssueRepo{issue: &domain.Issue{ID: "i"}}

	uc := grouping.New(events, issues, &fakeFingerprinter{}, &fakeClock{}, discardLogger(), 10)
	err := uc.ProcessBatch(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 0, issues.calls)
}

func TestProcessBatch_UpsertFailure_SkipsEventContinuesBatch(t *testing.T) {
	events := &fakeEventRepo{
		events: []*domain.Event{
			{ID: "ev-1", Message: "first"},
			{ID: "ev-2", Message: "second"},
		},
	}
	callCount := 0
	issues := &fakeIssueRepo{}
	issues.err = errors.New("db error") // will fail on first call

	// Use a real issue repo that fails first then succeeds
	failOnce := &failOnceIssueRepo{issue: &domain.Issue{ID: "i"}}
	uc := grouping.New(events, failOnce, &fakeFingerprinter{}, &fakeClock{}, discardLogger(), 10)
	err := uc.ProcessBatch(context.Background())

	require.NoError(t, err) // batch itself must not fail
	_ = callCount
	assert.Len(t, events.processed, 1, "second event should still be processed")
}

type failOnceIssueRepo struct {
	issue *domain.Issue
	calls int
}

func (f *failOnceIssueRepo) Upsert(_ context.Context, _ *domain.Event) (*domain.Issue, bool, error) {
	f.calls++
	if f.calls == 1 {
		return nil, false, errors.New("transient error")
	}
	return f.issue, false, nil
}

func (f *failOnceIssueRepo) TopByCount(_ context.Context, _ string, _ time.Time, _ int) ([]*domain.Issue, error) {
	return nil, nil
}

func (f *failOnceIssueRepo) CountInWindow(_ context.Context, _ string, _ time.Duration) (int64, error) {
	return 0, nil
}

func (f *failOnceIssueRepo) UpdateStatus(_ context.Context, _ string, _ domain.IssueStatus) error {
	return nil
}

func TestProcessBatch_FingerprintIsSetOnEvent(t *testing.T) {
	ev := &domain.Event{ID: "ev-1", Message: "test"}
	events := &fakeEventRepo{events: []*domain.Event{ev}}
	issues := &fakeIssueRepo{issue: &domain.Issue{ID: "i"}}
	fp := &fakeFingerprinter{fp: "deadbeef"}

	uc := grouping.New(events, issues, fp, &fakeClock{}, discardLogger(), 10)
	require.NoError(t, uc.ProcessBatch(context.Background()))

	assert.Equal(t, domain.Fingerprint("deadbeef"), ev.Fingerprint)
}

func TestProcessBatch_ListError_ReturnsError(t *testing.T) {
	events := &fakeEventRepo{listErr: errors.New("db down")}
	issues := &fakeIssueRepo{issue: &domain.Issue{ID: "i"}}

	uc := grouping.New(events, issues, &fakeFingerprinter{}, &fakeClock{}, discardLogger(), 10)
	err := uc.ProcessBatch(context.Background())

	assert.Error(t, err)
}
