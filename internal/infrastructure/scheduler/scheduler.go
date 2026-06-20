package scheduler

import (
	"context"
	"log/slog"
	"time"
)

// WorkerFunc is called on each scheduler tick.
type WorkerFunc func(ctx context.Context) error

// RunWorker executes fn every interval until ctx is cancelled.
// Errors from fn are logged and the worker continues — one bad tick
// must not stop ongoing processing.
func RunWorker(ctx context.Context, log *slog.Logger, name string, interval time.Duration, fn WorkerFunc) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	log.Info("worker started", "name", name, "interval", interval)
	for {
		select {
		case <-ctx.Done():
			log.Info("worker stopped", "name", name)
			return
		case <-ticker.C:
			if err := fn(ctx); err != nil {
				log.Error("worker tick failed", "name", name, "error", err)
			}
		}
	}
}
