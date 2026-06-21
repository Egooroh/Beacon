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

	"github.com/Egooroh/beacon/internal/domain"
	"github.com/Egooroh/beacon/internal/adapter/fingerprint"
	"github.com/Egooroh/beacon/internal/adapter/ingest/generic"
	sentryparser "github.com/Egooroh/beacon/internal/adapter/ingest/sentry"
	slacknotify "github.com/Egooroh/beacon/internal/adapter/notify/slack"
	tgnotify "github.com/Egooroh/beacon/internal/adapter/notify/telegram"
	pgstore "github.com/Egooroh/beacon/internal/adapter/repository/postgres"
	"github.com/Egooroh/beacon/internal/infrastructure/config"
	"github.com/Egooroh/beacon/internal/infrastructure/logger"
	"github.com/Egooroh/beacon/internal/infrastructure/metrics"
	infrapg "github.com/Egooroh/beacon/internal/infrastructure/postgres"
	"github.com/Egooroh/beacon/internal/infrastructure/scheduler"
	server "github.com/Egooroh/beacon/internal/transport/http"
	"github.com/Egooroh/beacon/internal/transport/http/handler"
	"github.com/Egooroh/beacon/internal/transport/telegrambot"
	"github.com/Egooroh/beacon/internal/usecase/alerting"
	"github.com/Egooroh/beacon/internal/usecase/digest"
	"github.com/Egooroh/beacon/internal/usecase/grouping"
	"github.com/Egooroh/beacon/internal/usecase/ingest"
	issueuc "github.com/Egooroh/beacon/internal/usecase/issue"
	"github.com/Egooroh/beacon/internal/usecase/project"
	"github.com/Egooroh/beacon/internal/usecase/subscription"
	"github.com/Egooroh/beacon/migrations"
)

const defaultTopN = 10

type noopAlerter struct{}

func (noopAlerter) MaybeAlert(context.Context, *domain.Issue, domain.AlertType) error { return nil }

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

	collector, err := metrics.New()
	if err != nil {
		log.Error("register metrics", "error", err)
		os.Exit(1)
	}

	// ── Adapters ──────────────────────────────────────────────────────────────
	eventRepo   := pgstore.NewEventRepository(pool)
	projectRepo := pgstore.NewProjectRepository(pool)
	issueRepo   := pgstore.NewIssueRepository(pool)
	subRepo     := pgstore.NewSubscriptionRepository(pool)
	parser      := generic.New()
	fp          := fingerprint.New()

	// ── Notifiers (one per configured platform) ────────────────────────────────
	var alertNotifiers []alerting.Notifier
	var digestNotifiers []digest.Notifier
	if cfg.TelegramToken != "" {
		tg := tgnotify.New(cfg.TelegramToken)
		alertNotifiers = append(alertNotifiers, tg)
		digestNotifiers = append(digestNotifiers, tg)
	}
	if cfg.SlackToken != "" {
		sl := slacknotify.New(cfg.SlackToken)
		alertNotifiers = append(alertNotifiers, sl)
		digestNotifiers = append(digestNotifiers, sl)
	}

	// ── Use cases ─────────────────────────────────────────────────────────────
	ingestUC   := ingest.New(eventRepo, projectRepo, parser, ingest.SystemClock)
	projectUC  := project.New(projectRepo)
	subscripUC := subscription.New(subRepo, projectRepo)
	issueUC    := issueuc.New(issueRepo)

	// ── Alerting (only when at least one notifier is configured) ───────────────
	var alertingUC *alerting.Service
	if len(alertNotifiers) > 0 {
		alertingUC = alerting.New(
			alertNotifiers, issueRepo, projectRepo, subRepo,
			alerting.SystemClock, log,
			cfg.AlertCooldown, cfg.SpikeFactor, cfg.SpikeMin,
			collector,
		)
	}

	var groupAlerter grouping.Alerter
	if alertingUC != nil {
		groupAlerter = alertingUC
	} else {
		groupAlerter = noopAlerter{}
	}

	groupingUC := grouping.New(
		eventRepo, issueRepo, fp, groupAlerter,
		grouping.SystemClock, log, cfg.ProcessBatch,
		collector,
	)

	// ── Digest worker ─────────────────────────────────────────────────────────
	var digestSvc *digest.Service
	if len(digestNotifiers) > 0 {
		digestSvc = digest.New(
			issueRepo, subRepo, projectRepo,
			digestNotifiers,
			digest.SystemClock, log,
			cfg.DigestInterval, defaultTopN,
		)
	}

	// ── Telegram bot ──────────────────────────────────────────────────────────
	if cfg.TelegramToken != "" {
		tgBot, err := telegrambot.New(
			cfg.TelegramToken,
			projectUC, subscripUC, issueUC, subRepo,
			log, cfg.PublicURL,
		)
		if err != nil {
			log.Error("create telegram bot", "error", err)
		} else {
			go tgBot.Run(ctx)
		}
	}

	// ── Sentry webhook handler ─────────────────────────────────────────────────
	sentryHandler := handler.NewSentryWebhookHandler(ingestUC, sentryparser.New(), cfg.SentrySecret)

	// ── HTTP server ───────────────────────────────────────────────────────────
	r := server.New(log, pool, ingestUC, projectUC, subscripUC, issueUC,
		sentryHandler, cfg.IngestRateLimit, collector)

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
	if alertingUC != nil {
		go scheduler.RunWorker(ctx, log, "spike-checker", cfg.DigestInterval, alertingUC.CheckSpikes)
	}
	if digestSvc != nil {
		go scheduler.RunWorker(ctx, log, "digest", cfg.DigestInterval, digestSvc.SendDigest)
	}

	<-ctx.Done()
	log.Info("shutting down")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("server shutdown", "error", err)
	}
}
