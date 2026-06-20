package main

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Egooroh/beacon/internal/adapter/fingerprint"
	"github.com/Egooroh/beacon/internal/adapter/ingest/generic"
	pgstore "github.com/Egooroh/beacon/internal/adapter/repository/postgres"
	"github.com/Egooroh/beacon/internal/infrastructure/config"
	"github.com/Egooroh/beacon/internal/infrastructure/logger"
	"github.com/Egooroh/beacon/internal/infrastructure/metrics"
	infrapg "github.com/Egooroh/beacon/internal/infrastructure/postgres"
	"github.com/Egooroh/beacon/internal/infrastructure/scheduler"
	server "github.com/Egooroh/beacon/internal/transport/http"
	"github.com/Egooroh/beacon/internal/usecase/grouping"
	"github.com/Egooroh/beacon/internal/usecase/ingest"
	"github.com/Egooroh/beacon/internal/usecase/project"
	"github.com/Egooroh/beacon/migrations"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}

	log := logger.New(cfg.LogLevel)
	slog.SetDefault(log)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	pool, err := infrapg.NewPool(ctx, cfg.DBDSN)
	if err != nil {
		log.Error("connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	migrationsFS, err := fs.Sub(migrations.FS, ".")
	if err != nil {
		log.Error("migrations fs", "error", err)
		os.Exit(1)
	}
	if err := infrapg.RunMigrations(ctx, cfg.DBDSN, migrationsFS); err != nil {
		log.Error("run migrations", "error", err)
		os.Exit(1)
	}

	if _, err := metrics.New(); err != nil {
		log.Error("register metrics", "error", err)
		os.Exit(1)
	}

	// ── Adapters ──────────────────────────────────────────────────────────────
	eventRepo   := pgstore.NewEventRepository(pool)
	projectRepo := pgstore.NewProjectRepository(pool)
	issueRepo   := pgstore.NewIssueRepository(pool)
	parser      := generic.New()
	fp          := fingerprint.New()

	// ── Use cases ─────────────────────────────────────────────────────────────
	ingestUC    := ingest.New(eventRepo, projectRepo, parser, ingest.SystemClock)
	projectUC   := project.New(projectRepo)
	groupingUC  := grouping.New(eventRepo, issueRepo, fp, grouping.SystemClock, log, cfg.ProcessBatch)

	// ── HTTP server ───────────────────────────────────────────────────────────
	r := server.New(log, pool, ingestUC, projectUC)

	srv := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info("starting HTTP server", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("HTTP server", "error", err)
			cancel()
		}
	}()

	// ── Background workers ────────────────────────────────────────────────────
	go scheduler.RunWorker(ctx, log, "processor", cfg.ProcessInterval, groupingUC.ProcessBatch)

	<-ctx.Done()
	log.Info("shutting down")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("server shutdown", "error", err)
	}
}
