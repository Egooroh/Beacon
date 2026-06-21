package digest_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Egooroh/beacon/internal/domain"
	"github.com/Egooroh/beacon/internal/usecase/digest"
)

// ── fakes ─────────────────────────────────────────────────────────────────────

type fakeIssueRepo struct {
	issues []*domain.Issue
}

func (f *fakeIssueRepo) TopByCount(_ context.Context, _ string, _ time.Time, _ int) ([]*domain.Issue, error) {
	return f.issues, nil
}

type fakeSubRepo struct {
	projectIDs []string
	subs       []*domain.Subscription
}

func (f *fakeSubRepo) ListProjectsWithSubscriptions(_ context.Context) ([]string, error) {
	return f.projectIDs, nil
}

func (f *fakeSubRepo) ListByProject(_ context.Context, _ string) ([]*domain.Subscription, error) {
	return f.subs, nil
}

type fakeProjRepo struct {
	project *domain.Project
}

func (f *fakeProjRepo) FindByID(_ context.Context, _ string) (*domain.Project, error) {
	return f.project, nil
}

type fakeNotifier struct {
	platform string
	sent     []string // texts sent
}

func (f *fakeNotifier) Send(_ context.Context, _ string, text string) error {
	f.sent = append(f.sent, text)
	return nil
}

func (f *fakeNotifier) Platform() string { return f.platform }

type fixedClock struct{ t time.Time }

func (c *fixedClock) Now() time.Time { return c.t }

func newService(ir digest.IssueRepository, sr digest.SubscriptionRepository,
	pr digest.ProjectRepository, n digest.Notifier,
) *digest.Service {
	return digest.New(ir, sr, pr, []digest.Notifier{n}, &fixedClock{t: time.Now()},
		slog.New(slog.NewTextHandler(io.Discard, nil)), time.Hour, 10)
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestSendDigest_SendsToMatchingSubscribers(t *testing.T) {
	issues := []*domain.Issue{
		{Title: "DB timeout", Level: domain.LevelError, EventsCount: 42},
		{Title: "OOM", Level: domain.LevelFatal, EventsCount: 7},
	}
	n := &fakeNotifier{platform: "telegram"}
	svc := newService(
		&fakeIssueRepo{issues: issues},
		&fakeSubRepo{
			projectIDs: []string{"p1"},
			subs:       []*domain.Subscription{{Platform: "telegram", ChatID: "-1"}},
		},
		&fakeProjRepo{project: &domain.Project{Name: "MyApp"}},
		n,
	)

	require.NoError(t, svc.SendDigest(context.Background()))
	require.Len(t, n.sent, 1)
	assert.Contains(t, n.sent[0], "MyApp")
	assert.Contains(t, n.sent[0], "DB timeout")
	assert.Contains(t, n.sent[0], "42")
}

func TestSendDigest_NoIssues_SendsNothing(t *testing.T) {
	n := &fakeNotifier{platform: "telegram"}
	svc := newService(
		&fakeIssueRepo{issues: nil},
		&fakeSubRepo{projectIDs: []string{"p1"}, subs: []*domain.Subscription{{Platform: "telegram", ChatID: "-1"}}},
		&fakeProjRepo{project: &domain.Project{Name: "Empty"}},
		n,
	)

	require.NoError(t, svc.SendDigest(context.Background()))
	assert.Empty(t, n.sent)
}

func TestSendDigest_NoSubscribedProjects_SendsNothing(t *testing.T) {
	n := &fakeNotifier{platform: "telegram"}
	svc := newService(
		&fakeIssueRepo{issues: []*domain.Issue{{Title: "err"}}},
		&fakeSubRepo{projectIDs: nil},
		&fakeProjRepo{project: &domain.Project{}},
		n,
	)

	require.NoError(t, svc.SendDigest(context.Background()))
	assert.Empty(t, n.sent)
}

func TestSendDigest_SkipsNonMatchingPlatform(t *testing.T) {
	n := &fakeNotifier{platform: "telegram"}
	svc := newService(
		&fakeIssueRepo{issues: []*domain.Issue{{Title: "err", EventsCount: 1}}},
		&fakeSubRepo{
			projectIDs: []string{"p1"},
			subs:       []*domain.Subscription{{Platform: "slack", ChatID: "#alerts"}},
		},
		&fakeProjRepo{project: &domain.Project{Name: "App"}},
		n,
	)

	require.NoError(t, svc.SendDigest(context.Background()))
	assert.Empty(t, n.sent)
}
