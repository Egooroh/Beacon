package middleware

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.status = code
	sr.ResponseWriter.WriteHeader(code)
}

// IngestMetrics returns a middleware that records ingest handler latency and
// increments the events-ingested counter only for 202 Accepted responses.
func IngestMetrics(ingested prometheus.Counter, duration prometheus.Histogram) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sr := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			start := time.Now()
			next.ServeHTTP(sr, r)
			duration.Observe(time.Since(start).Seconds())
			if sr.status == http.StatusAccepted {
				ingested.Inc()
			}
		})
	}
}
