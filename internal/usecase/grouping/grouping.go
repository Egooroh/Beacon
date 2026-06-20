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
	clock         Clock
	log           *slog.Logger
	batchSize     int
}

// New creates a grouping UseCase.
func New(
	events EventRepository,
	issues IssueRepository,
	fingerprinter Fingerprinter,
	clock Clock,
	log *slog.Logger,
	batchSize int,
) *UseCase {
	return &UseCase{
		events:        events,
		issues:        issues,
		fingerprinter: fingerprinter,
		clock:         clock,
		log:           log,
		batchSize:     batchSize,
	}
}

// ProcessBatch reads up to batchSize unprocessed events and groups them into issues.
// A failure on one event is logged and skipped — the batch continues (FR-5).
func (uc *UseCase) ProcessBatch(ctx context.Context) error {
	events, err := uc.events.ListUnprocessed(ctx, uc.batchSize)
	if err != nil {
		return fmt.Errorf("list unprocessed: %w", err)
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
	if created {
		uc.log.Info("new issue",
			"issue_id", issue.ID,
			"fingerprint", issue.Fingerprint,
			"title", issue.Title,
		)
	}
	return nil
}

// SystemClock is the production Clock backed by time.Now.
var SystemClock Clock = realClock{}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }
