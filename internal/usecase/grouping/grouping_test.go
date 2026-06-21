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
	issue         *domain.Issue
	created       bool
	err           error
	calls         int
	updatedStatus domain.IssueStatus
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

func (f *fakeIssueRepo) UpdateStatus(_ context.Context, _ string, s domain.IssueStatus) error {
	f.updatedStatus = s
	return nil
}

type fakeFingerprinter struct{ fp domain.Fingerprint }

func (f *fakeFingerprinter) Compute(_ *domain.Event) domain.Fingerprint { return f.fp }

type fakeClock struct{ t time.Time }

func (c *fakeClock) Now() time.Time { return c.t }

type fakeAlerter struct {
	calls []domain.AlertType
	err   error
}

func (f *fakeAlerter) MaybeAlert(_ context.Context, _ *domain.Issue, t domain.AlertType) error {
	f.calls = append(f.calls, t)
	return f.err
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newUC(events grouping.EventRepository, issues grouping.IssueRepository,
	fp grouping.Fingerprinter, alerter grouping.Alerter,
) *grouping.UseCase {
	return grouping.New(events, issues, fp, alerter, &fakeClock{}, discardLogger(), 10, nil)
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestProcessBatch_CreatesNewIssue(t *testing.T) {
	events := &fakeEventRepo{
		events: []*domain.Event{{ID: "ev-1", ProjectID: "p-1", Message: "oops"}},
	}
	issue := &domain.Issue{ID: "issue-1"}
	issues := &fakeIssueRepo{issue: issue, created: true}
	alerter := &fakeAlerter{}

	err := newUC(events, issues, &fakeFingerprinter{fp: "abc123"}, alerter).
		ProcessBatch(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 1, issues.calls)
	assert.Equal(t, []string{"ev-1"}, events.processed)
}

func TestProcessBatch_NewIssue_TriggersNewIssueAlert(t *testing.T) {
	events := &fakeEventRepo{
		events: []*domain.Event{{ID: "ev-1", Message: "boom"}},
	}
	issues := &fakeIssueRepo{issue: &domain.Issue{ID: "i"}, created: true}
	alerter := &fakeAlerter{}

	require.NoError(t, newUC(events, issues, &fakeFingerprinter{}, alerter).
		ProcessBatch(context.Background()))

	require.Len(t, alerter.calls, 1)
	assert.Equal(t, domain.AlertNewIssue, alerter.calls[0])
}

func TestProcessBatch_ExistingOpenIssue_NoAlert(t *testing.T) {
	events := &fakeEventRepo{
		events: []*domain.Event{{ID: "ev-1", Message: "again"}},
	}
	issues := &fakeIssueRepo{issue: &domain.Issue{ID: "i", Status: domain.StatusOpen}, created: false}
	alerter := &fakeAlerter{}

	require.NoError(t, newUC(events, issues, &fakeFingerprinter{}, alerter).
		ProcessBatch(context.Background()))

	assert.Empty(t, alerter.calls)
}

func TestProcessBatch_ResolvedIssueGetsNewEvent_RegressionAlert(t *testing.T) {
	events := &fakeEventRepo{
		events: []*domain.Event{{ID: "ev-1", Message: "back again"}},
	}
	issues := &fakeIssueRepo{
		issue:   &domain.Issue{ID: "i", Status: domain.StatusResolved},
		created: false,
	}
	alerter := &fakeAlerter{}

	require.NoError(t, newUC(events, issues, &fakeFingerprinter{}, alerter).
		ProcessBatch(context.Background()))

	require.Len(t, alerter.calls, 1)
	assert.Equal(t, domain.AlertRegression, alerter.calls[0])
	assert.Equal(t, domain.StatusOpen, issues.updatedStatus, "resolved issue must be reopened")
}

func TestProcessBatch_EmptyQueue_DoesNothing(t *testing.T) {
	events := &fakeEventRepo{}
	issues := &fakeIssueRepo{issue: &domain.Issue{ID: "i"}}

	err := newUC(events, issues, &fakeFingerprinter{}, &fakeAlerter{}).
		ProcessBatch(context.Background())

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
	failOnce := &failOnceIssueRepo{issue: &domain.Issue{ID: "i"}}

	err := newUC(events, failOnce, &fakeFingerprinter{}, &fakeAlerter{}).
		ProcessBatch(context.Background())

	require.NoError(t, err)
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

	require.NoError(t, newUC(events, issues, &fakeFingerprinter{fp: "deadbeef"}, &fakeAlerter{}).
		ProcessBatch(context.Background()))

	assert.Equal(t, domain.Fingerprint("deadbeef"), ev.Fingerprint)
}

func TestProcessBatch_ListError_ReturnsError(t *testing.T) {
	events := &fakeEventRepo{listErr: errors.New("db down")}
	issues := &fakeIssueRepo{issue: &domain.Issue{ID: "i"}}

	err := newUC(events, issues, &fakeFingerprinter{}, &fakeAlerter{}).
		ProcessBatch(context.Background())

	assert.Error(t, err)
}
