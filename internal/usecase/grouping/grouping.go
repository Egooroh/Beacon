package grouping

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Egooroh/beacon/internal/domain"
)

// UseCase fingerprints pending events and groups them into issues.
type UseCase struct {
	events        EventRepository
	issues        IssueRepository
	fingerprinter Fingerprinter
	alerter       Alerter
	clock         Clock
	log           *slog.Logger
	batchSize     int
	metrics       MetricsRecorder
}

// New creates a grouping UseCase. metrics may be nil to disable recording.
func New(
	events EventRepository,
	issues IssueRepository,
	fingerprinter Fingerprinter,
	alerter Alerter,
	clock Clock,
	log *slog.Logger,
	batchSize int,
	metrics MetricsRecorder,
) *UseCase {
	return &UseCase{
		events:        events,
		issues:        issues,
		fingerprinter: fingerprinter,
		alerter:       alerter,
		clock:         clock,
		log:           log,
		batchSize:     batchSize,
		metrics:       metrics,
	}
}

// ProcessBatch reads up to batchSize unprocessed events and groups them into issues.
// A failure on one event is logged and skipped — the batch continues (FR-5).
func (uc *UseCase) ProcessBatch(ctx context.Context) error {
	events, err := uc.events.ListUnprocessed(ctx, uc.batchSize)
	if err != nil {
		return fmt.Errorf("list unprocessed: %w", err)
	}
	if uc.metrics != nil {
		uc.metrics.SetProcessingLag(len(events))
	}
	for _, ev := range events {
		if err := uc.processOne(ctx, ev); err != nil {
			uc.log.Error("failed to process event", "event_id", ev.ID, "error", err)
		}
	}
	return nil
}

func (uc *UseCase) processOne(ctx context.Context, ev *domain.Event) error {
	ev.Fingerprint = uc.fingerprinter.Compute(ev)

	issue, created, err := uc.issues.Upsert(ctx, ev)
	if err != nil {
		return fmt.Errorf("upsert issue: %w", err)
	}
	if err := uc.events.SetProcessed(ctx, ev.ID, issue.ID, ev.Fingerprint); err != nil {
		return fmt.Errorf("set processed: %w", err)
	}

	if uc.metrics != nil {
		uc.metrics.RecordEventProcessed()
		if created {
			uc.metrics.RecordIssueCreated()
		}
	}

	switch {
	case created:
		uc.log.Info("new issue", "issue_id", issue.ID, "title", issue.Title)
		uc.alert(ctx, issue, domain.AlertNewIssue)
	case issue.Status == domain.StatusResolved:
		// Regression: a resolved issue is seeing events again — reopen it.
		if err := uc.issues.UpdateStatus(ctx, issue.ID, domain.StatusOpen); err != nil {
			uc.log.Error("reopen issue", "issue_id", issue.ID, "error", err)
		}
		issue.Status = domain.StatusOpen
		uc.log.Info("issue regression", "issue_id", issue.ID, "title", issue.Title)
		uc.alert(ctx, issue, domain.AlertRegression)
	}
	return nil
}

// alert calls the alerter and logs errors without propagating them.
func (uc *UseCase) alert(ctx context.Context, issue *domain.Issue, t domain.AlertType) {
	if err := uc.alerter.MaybeAlert(ctx, issue, t); err != nil {
		uc.log.Error("alert failed", "issue_id", issue.ID, "alert_type", t, "error", err)
	}
}

// SystemClock is the production Clock backed by time.Now.
var SystemClock Clock = realClock{}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }
