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
func New(log *slog.Logger, db DBPinger) http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.RequestID)
	r.Use(middleware.Recover(log))
	r.Use(middleware.Logging(log))

	health := handler.NewHealthHandler(db)
	r.Get("/healthz", health.Healthz)
	r.Get("/readyz", health.Readyz)
	r.Handle("/metrics", promhttp.Handler())

	return r
}
