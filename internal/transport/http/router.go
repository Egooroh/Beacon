// Package server wires HTTP routes, middleware, and handlers into a single http.Handler.
package server

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/Egooroh/beacon/internal/infrastructure/metrics"
	"github.com/Egooroh/beacon/internal/transport/http/handler"
	"github.com/Egooroh/beacon/internal/transport/http/middleware"
)

// DBPinger checks database connectivity; satisfied by *pgxpool.Pool.
type DBPinger interface {
	Ping(ctx context.Context) error
}

// New assembles and returns the application HTTP router.
// sentryWebhook may be nil; when non-nil its route is registered.
// collector may be nil; when non-nil ingest metrics are recorded.
func New(
	log *slog.Logger,
	db DBPinger,
	ingester handler.Ingester,
	projects handler.ProjectCreator,
	subscriber handler.Subscriber,
	issues handler.IssueManager,
	sentryWebhook *handler.SentryWebhookHandler,
	ingestRateLimit int,
	collector *metrics.Collector,
) http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.RequestID)
	r.Use(middleware.Recover(log))
	r.Use(middleware.Logging(log))

	// Infrastructure endpoints — always available.
	health := handler.NewHealthHandler(db)
	r.Get("/healthz", health.Healthz)
	r.Get("/readyz", health.Readyz)
	r.Handle("/metrics", promhttp.Handler())

	// Web dashboard UI.
	r.Handle("/ui", http.RedirectHandler("/ui/", http.StatusMovedPermanently))
	r.Handle("/ui/*", http.StripPrefix("/ui", handler.NewUIHandler()))

	// REST API v1.
	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/projects", handler.NewProjectHandler(projects).Create)

		subH := handler.NewSubscriptionHandler(subscriber)
		r.Route("/projects/{id}/subscriptions", func(r chi.Router) {
			r.Post("/", subH.Create)
			r.Get("/", subH.List)
		})

		issueH := handler.NewIssueHandler(issues)
		r.Get("/projects/{id}/issues", issueH.List)
		r.Patch("/issues/{id}/status", issueH.SetStatus)

		// Ingest: rate limiting + token auth + optional metrics recording.
		ingestChain := []func(http.Handler) http.Handler{
			middleware.RateLimit(ingestRateLimit, time.Minute),
			middleware.RequireToken,
		}
		if collector != nil {
			ingestChain = append(ingestChain,
				middleware.IngestMetrics(collector.EventsIngested, collector.IngestDuration))
		}
		r.With(ingestChain...).Post("/ingest", handler.NewIngestHandler(ingester).Handle)

		// Sentry webhook: HMAC auth handled inside the handler.
		if sentryWebhook != nil {
			r.With(middleware.RateLimit(ingestRateLimit, time.Minute)).
				Post("/projects/{id}/webhook/sentry", sentryWebhook.Handle)
		}
	})

	return r
}
