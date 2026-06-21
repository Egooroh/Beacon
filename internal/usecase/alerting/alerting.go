package alerting

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Egooroh/beacon/internal/domain"
)

// Service handles notification routing with cooldown and spike detection.
type Service struct {
	notifiers     map[string]Notifier
	issues        IssueRepository
	projects      ProjectRepository
	subscriptions SubscriptionRepository
	clock         Clock
	log           *slog.Logger
	cooldown      time.Duration
	spikeFactor   float64
	spikeMin      int64
	metrics       MetricsRecorder
}

// New creates an alerting Service. notifiers is a slice of platform-specific
// senders; each is indexed by its Platform() value for O(1) lookup.
// metrics may be nil to disable recording.
func New(
	notifiers []Notifier,
	issues IssueRepository,
	projects ProjectRepository,
	subscriptions SubscriptionRepository,
	clock Clock,
	log *slog.Logger,
	cooldown time.Duration,
	spikeFactor float64,
	spikeMin int64,
	metrics MetricsRecorder,
) *Service {
	nm := make(map[string]Notifier, len(notifiers))
	for _, n := range notifiers {
		nm[n.Platform()] = n
	}
	return &Service{
		notifiers:     nm,
		issues:        issues,
		projects:      projects,
		subscriptions: subscriptions,
		clock:         clock,
		log:           log,
		cooldown:      cooldown,
		spikeFactor:   spikeFactor,
		spikeMin:      spikeMin,
		metrics:       metrics,
	}
}

// MaybeAlert sends an alert for the given issue if the cooldown has elapsed.
// Implements the grouping.Alerter port.
func (s *Service) MaybeAlert(ctx context.Context, issue *domain.Issue, alertType domain.AlertType) error {
	if issue.LastAlertAt != nil && s.clock.Now().Sub(*issue.LastAlertAt) < s.cooldown {
		return nil // still within cooldown window
	}

	project, err := s.projects.FindByID(ctx, issue.ProjectID)
	if err != nil {
		return fmt.Errorf("find project: %w", err)
	}

	subs, err := s.subscriptions.ListByProject(ctx, issue.ProjectID)
	if err != nil {
		return fmt.Errorf("list subscriptions: %w", err)
	}

	alert := domain.Alert{Type: alertType, Issue: issue, Project: project}

	sent := 0
	for _, sub := range subs {
		n, ok := s.notifiers[sub.Platform]
		if !ok {
			continue
		}
		if err := n.Notify(ctx, alert, sub.ChatID); err != nil {
			s.log.Error("notify failed",
				"platform", sub.Platform,
				"chat_id", sub.ChatID,
				"error", err,
			)
		} else {
			sent++
			if s.metrics != nil {
				s.metrics.RecordAlertSent(string(alertType))
			}
		}
	}

	if sent == 0 {
		return nil // no subscriptions matched — nothing to record
	}

	now := s.clock.Now()
	if err := s.issues.UpdateLastAlertAt(ctx, issue.ID, now); err != nil {
		return fmt.Errorf("update last_alert_at: %w", err)
	}
	return nil
}

// CheckSpikes scans open issues for event-rate anomalies and alerts on spikes.
func (s *Service) CheckSpikes(ctx context.Context) error {
	const checkLimit = 500
	issues, err := s.issues.ListOpen(ctx, checkLimit)
	if err != nil {
		return fmt.Errorf("list open issues: %w", err)
	}

	for _, issue := range issues {
		if err := s.checkSpike(ctx, issue); err != nil {
			s.log.Error("spike check failed", "issue_id", issue.ID, "error", err)
		}
	}
	return nil
}

func (s *Service) checkSpike(ctx context.Context, issue *domain.Issue) error {
	countNow, err := s.issues.CountInWindow(ctx, issue.ID, time.Hour)
	if err != nil {
		return err
	}
	countPrev, err := s.issues.CountInWindow(ctx, issue.ID, 2*time.Hour)
	if err != nil {
		return err
	}
	// countPrev is the 2h window; subtract the 1h window to get the prior hour.
	prevHour := countPrev - countNow
	if prevHour <= 0 {
		return nil // no baseline
	}
	if countNow < s.spikeMin {
		return nil // below minimum threshold
	}
	if float64(countNow) < s.spikeFactor*float64(prevHour) {
		return nil
	}
	return s.MaybeAlert(ctx, issue, domain.AlertSpike)
}

// SystemClock is the production Clock backed by time.Now.
var SystemClock Clock = realClock{}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }
