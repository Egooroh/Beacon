package digest

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Egooroh/beacon/internal/domain"
)

const defaultTopN = 10

// Service sends periodic digests of the top issues per project to all subscribers.
type Service struct {
	issues        IssueRepository
	subscriptions SubscriptionRepository
	projects      ProjectRepository
	notifiers     map[string]Notifier
	clock         Clock
	log           *slog.Logger
	window        time.Duration
	topN          int
}

// New creates a digest Service. notifiers is a slice of platform-specific
// senders; each is indexed by its Platform() value for O(1) lookup.
func New(
	issues IssueRepository,
	subscriptions SubscriptionRepository,
	projects ProjectRepository,
	notifiers []Notifier,
	clock Clock,
	log *slog.Logger,
	window time.Duration,
	topN int,
) *Service {
	if topN <= 0 {
		topN = defaultTopN
	}
	nm := make(map[string]Notifier, len(notifiers))
	for _, n := range notifiers {
		nm[n.Platform()] = n
	}
	return &Service{
		issues:        issues,
		subscriptions: subscriptions,
		projects:      projects,
		notifiers:     nm,
		clock:         clock,
		log:           log,
		window:        window,
		topN:          topN,
	}
}

// SendDigest delivers a top-issues report to every subscribed project.
// Intended to be called by the scheduler on DigestInterval.
func (s *Service) SendDigest(ctx context.Context) error {
	projectIDs, err := s.subscriptions.ListProjectsWithSubscriptions(ctx)
	if err != nil {
		return fmt.Errorf("list subscribed projects: %w", err)
	}
	for _, pid := range projectIDs {
		if err := s.sendForProject(ctx, pid); err != nil {
			s.log.Error("digest failed", "project_id", pid, "error", err)
		}
	}
	return nil
}

func (s *Service) sendForProject(ctx context.Context, projectID string) error {
	since := s.clock.Now().Add(-s.window)
	issues, err := s.issues.TopByCount(ctx, projectID, since, s.topN)
	if err != nil {
		return fmt.Errorf("top issues: %w", err)
	}
	if len(issues) == 0 {
		return nil // nothing worth reporting
	}

	project, err := s.projects.FindByID(ctx, projectID)
	if err != nil {
		return fmt.Errorf("find project: %w", err)
	}

	subs, err := s.subscriptions.ListByProject(ctx, projectID)
	if err != nil {
		return fmt.Errorf("list subscriptions: %w", err)
	}

	text := formatDigest(project, issues, s.window)
	for _, sub := range subs {
		n, ok := s.notifiers[sub.Platform]
		if !ok {
			continue
		}
		if err := n.Send(ctx, sub.ChatID, text); err != nil {
			s.log.Error("send digest failed", "chat_id", sub.ChatID, "error", err)
		}
	}
	return nil
}

func formatDigest(project *domain.Project, issues []*domain.Issue, window time.Duration) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[DIGEST] %s — top issues (last %s)\n", project.Name, window)
	for i, iss := range issues {
		fmt.Fprintf(&b, "%d. [%s] %s — %d events\n", i+1, iss.Level, iss.Title, iss.EventsCount)
	}
	return b.String()
}

// SystemClock is the production Clock.
var SystemClock Clock = realClock{}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }
