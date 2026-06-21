package ingest

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Egooroh/beacon/internal/domain"
)

// UseCase handles the fast ingest path: authenticate → parse → persist.
// Fingerprinting and grouping are intentionally deferred to the processing worker (FR-3).
type UseCase struct {
	events   EventRepository
	projects ProjectRepository
	parser   PayloadParser
	clock    Clock
}

// New creates an ingest UseCase with all required dependencies.
func New(events EventRepository, projects ProjectRepository, parser PayloadParser, clock Clock) *UseCase {
	return &UseCase{
		events:   events,
		projects: projects,
		parser:   parser,
		clock:    clock,
	}
}

// Accept authenticates the token, parses the payload, and persists the raw event.
// Returns domain.ErrUnauthorized for bad tokens, domain.ErrInvalidInput for bad payloads.
func (uc *UseCase) Accept(ctx context.Context, rawToken string, payload []byte) error {
	project, err := uc.projects.FindByTokenHash(ctx, domain.HashToken(rawToken))
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.ErrUnauthorized
		}
		return fmt.Errorf("find project: %w", err)
	}

	event, err := uc.parser.Parse(payload)
	if err != nil {
		return fmt.Errorf("%w: %s", domain.ErrInvalidInput, err.Error())
	}

	event.ProjectID = project.ID
	event.ReceivedAt = uc.clock.Now()
	event.RawPayload = payload

	if err := uc.events.Save(ctx, event); err != nil {
		return fmt.Errorf("save event: %w", err)
	}
	return nil
}

// AcceptForProject saves a pre-parsed event for a known project.
// Used by webhook adapters that perform their own authentication (e.g. Sentry HMAC).
func (uc *UseCase) AcceptForProject(ctx context.Context, projectID string, event *domain.Event) error {
	event.ProjectID = projectID
	event.ReceivedAt = uc.clock.Now()
	if err := uc.events.Save(ctx, event); err != nil {
		return fmt.Errorf("save event: %w", err)
	}
	return nil
}

// SystemClock is the production Clock implementation backed by time.Now.
var SystemClock Clock = realClock{}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }
