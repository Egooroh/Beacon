// Package server wires HTTP routes, middleware, and handlers into a single http.Handler.
package server

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/Egooroh/beacon/internal/transport/http/handler"
	"github.com/Egooroh/beacon/internal/transport/http/middleware"
)

// DBPinger checks database connectivity; satisfied by *pgxpool.Pool.
type DBPinger interface {
	Ping(ctx context.Context) error
}

// New assembles and returns the application HTTP router.
func New(
	log *slog.Logger,
	db DBPinger,
	ingester handler.Ingester,
	projects handler.ProjectCreator,
	subscriber handler.Subscriber,
	issues handler.IssueManager,
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

		// Ingest requires a valid X-Beacon-Token; the use case performs DB auth.
		r.With(middleware.RequireToken).Post("/ingest", handler.NewIngestHandler(ingester).Handle)
	})

	return r
}
