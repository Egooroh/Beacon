package alerting_test

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
	"github.com/Egooroh/beacon/internal/usecase/alerting"
)

// ── fakes ─────────────────────────────────────────────────────────────────────

type fakeNotifier struct {
	platform string
	calls    []string // chat IDs notified
	err      error
}

func (f *fakeNotifier) Notify(_ context.Context, _ domain.Alert, chatID string) error {
	if f.err != nil {
		return f.err
	}
	f.calls = append(f.calls, chatID)
	return nil
}

func (f *fakeNotifier) Platform() string { return f.platform }

type fakeIssueRepo struct {
	updateErr     error
	lastAlertCalls int
	openIssues    []*domain.Issue
	windowCount   int64
}

func (f *fakeIssueRepo) UpdateLastAlertAt(_ context.Context, _ string, _ time.Time) error {
	f.lastAlertCalls++
	return f.updateErr
}

func (f *fakeIssueRepo) ListOpen(_ context.Context, _ int) ([]*domain.Issue, error) {
	return f.openIssues, nil
}

func (f *fakeIssueRepo) CountInWindow(_ context.Context, _ string, _ time.Duration) (int64, error) {
	return f.windowCount, nil
}

type fakeProjectRepo struct {
	project *domain.Project
	err     error
}

func (f *fakeProjectRepo) FindByID(_ context.Context, _ string) (*domain.Project, error) {
	return f.project, f.err
}

type fakeSubRepo struct {
	subs []*domain.Subscription
}

func (f *fakeSubRepo) ListByProject(_ context.Context, _ string) ([]*domain.Subscription, error) {
	return f.subs, nil
}

type fixedClock struct{ t time.Time }

func (c *fixedClock) Now() time.Time { return c.t }

func newService(n alerting.Notifier, ir alerting.IssueRepository, pr alerting.ProjectRepository,
	sr alerting.SubscriptionRepository, clk alerting.Clock, cooldown time.Duration,
) *alerting.Service {
	return alerting.New([]alerting.Notifier{n}, ir, pr, sr, clk, slog.New(slog.NewTextHandler(io.Discard, nil)),
		cooldown, 5.0, 10, nil)
}

// ── MaybeAlert tests ──────────────────────────────────────────────────────────

func TestMaybeAlert_SendsToMatchingSubscription(t *testing.T) {
	n := &fakeNotifier{platform: "telegram"}
	ir := &fakeIssueRepo{}
	pr := &fakeProjectRepo{project: &domain.Project{ID: "p", Name: "MyApp"}}
	sr := &fakeSubRepo{subs: []*domain.Subscription{
		{Platform: "telegram", ChatID: "-111"},
	}}
	clk := &fixedClock{t: time.Now()}

	issue := &domain.Issue{ID: "i", ProjectID: "p"}
	svc := newService(n, ir, pr, sr, clk, 15*time.Minute)

	require.NoError(t, svc.MaybeAlert(context.Background(), issue, domain.AlertNewIssue))
	assert.Equal(t, []string{"-111"}, n.calls)
	assert.Equal(t, 1, ir.lastAlertCalls)
}

func TestMaybeAlert_SkipsNonMatchingPlatform(t *testing.T) {
	n := &fakeNotifier{platform: "telegram"}
	ir := &fakeIssueRepo{}
	sr := &fakeSubRepo{subs: []*domain.Subscription{
		{Platform: "slack", ChatID: "#general"},
	}}

	issue := &domain.Issue{ID: "i", ProjectID: "p"}
	svc := newService(n, ir, &fakeProjectRepo{project: &domain.Project{}}, sr, &fixedClock{t: time.Now()}, time.Minute)

	require.NoError(t, svc.MaybeAlert(context.Background(), issue, domain.AlertNewIssue))
	assert.Empty(t, n.calls)
	assert.Equal(t, 0, ir.lastAlertCalls) // no subscription matched → don't record
}

func TestMaybeAlert_CooldownSuppressesAlert(t *testing.T) {
	n := &fakeNotifier{platform: "telegram"}
	ir := &fakeIssueRepo{}
	sr := &fakeSubRepo{subs: []*domain.Subscription{{Platform: "telegram", ChatID: "-1"}}}

	now := time.Now()
	lastAlert := now.Add(-5 * time.Minute) // 5m ago, cooldown is 15m
	issue := &domain.Issue{ID: "i", ProjectID: "p", LastAlertAt: &lastAlert}

	svc := newService(n, ir, &fakeProjectRepo{project: &domain.Project{}}, sr, &fixedClock{t: now}, 15*time.Minute)

	require.NoError(t, svc.MaybeAlert(context.Background(), issue, domain.AlertNewIssue))
	assert.Empty(t, n.calls, "must not notify within cooldown")
}

func TestMaybeAlert_SendsAfterCooldownExpires(t *testing.T) {
	n := &fakeNotifier{platform: "telegram"}
	ir := &fakeIssueRepo{}
	sr := &fakeSubRepo{subs: []*domain.Subscription{{Platform: "telegram", ChatID: "-1"}}}

	now := time.Now()
	lastAlert := now.Add(-20 * time.Minute) // 20m ago, cooldown is 15m
	issue := &domain.Issue{ID: "i", ProjectID: "p", LastAlertAt: &lastAlert}

	svc := newService(n, ir, &fakeProjectRepo{project: &domain.Project{}}, sr, &fixedClock{t: now}, 15*time.Minute)

	require.NoError(t, svc.MaybeAlert(context.Background(), issue, domain.AlertNewIssue))
	assert.Len(t, n.calls, 1)
}

func TestMaybeAlert_ProjectLookupError_ReturnsError(t *testing.T) {
	n := &fakeNotifier{platform: "telegram"}
	pr := &fakeProjectRepo{err: errors.New("db error")}

	issue := &domain.Issue{ID: "i", ProjectID: "p"}
	svc := newService(n, &fakeIssueRepo{}, pr, &fakeSubRepo{}, &fixedClock{t: time.Now()}, time.Minute)

	assert.Error(t, svc.MaybeAlert(context.Background(), issue, domain.AlertNewIssue))
}

// ── CheckSpikes tests ──────────────────────────────────────────────────────────

func TestCheckSpikes_SpikeTriggersAlert(t *testing.T) {
	n := &fakeNotifier{platform: "telegram"}
	ir2 := &countingWindowRepo{
		openIssues:    []*domain.Issue{{ID: "i", ProjectID: "p"}},
		windowCounts:  []int64{55, 66}, // 1h=55, 2h=66 → prevHour=11; 55 >= 5*11=55 → alert
		updateLastAt:  nil,
	}
	sr := &fakeSubRepo{subs: []*domain.Subscription{{Platform: "telegram", ChatID: "-1"}}}
	pr := &fakeProjectRepo{project: &domain.Project{ID: "p", Name: "App"}}

	svc := alerting.New([]alerting.Notifier{n}, ir2, pr, sr, &fixedClock{t: time.Now()},
		slog.New(slog.NewTextHandler(io.Discard, nil)), time.Minute, 5.0, 10, nil)

	require.NoError(t, svc.CheckSpikes(context.Background()))
	assert.Len(t, n.calls, 1, "spike should trigger one alert")
}

func TestCheckSpikes_NoBaseline_NoAlert(t *testing.T) {
	n := &fakeNotifier{platform: "telegram"}
	ir := &countingWindowRepo{
		openIssues:   []*domain.Issue{{ID: "i", ProjectID: "p"}},
		windowCounts: []int64{50, 50}, // 1h=50, 2h=50 → prevHour=0 → no baseline
	}
	svc := alerting.New([]alerting.Notifier{n}, ir, &fakeProjectRepo{project: &domain.Project{}},
		&fakeSubRepo{}, &fixedClock{t: time.Now()},
		slog.New(slog.NewTextHandler(io.Discard, nil)), time.Minute, 5.0, 10, nil)

	require.NoError(t, svc.CheckSpikes(context.Background()))
	assert.Empty(t, n.calls)
}

// countingWindowRepo returns pre-set counts in sequence.
type countingWindowRepo struct {
	openIssues   []*domain.Issue
	windowCounts []int64
	call         int
	updateLastAt func(context.Context, string, time.Time) error
}

func (r *countingWindowRepo) UpdateLastAlertAt(ctx context.Context, id string, t time.Time) error {
	if r.updateLastAt != nil {
		return r.updateLastAt(ctx, id, t)
	}
	return nil
}

func (r *countingWindowRepo) ListOpen(_ context.Context, _ int) ([]*domain.Issue, error) {
	return r.openIssues, nil
}

func (r *countingWindowRepo) CountInWindow(_ context.Context, _ string, _ time.Duration) (int64, error) {
	if r.call >= len(r.windowCounts) {
		return 0, nil
	}
	v := r.windowCounts[r.call]
	r.call++
	return v, nil
}
