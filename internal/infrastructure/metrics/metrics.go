// Package metrics registers application-level Prometheus collectors.
// All counters and histograms are registered on the default registry.
package metrics

import "github.com/prometheus/client_golang/prometheus"

// Collector holds all application metrics.
// It implements grouping.MetricsRecorder and alerting.MetricsRecorder.
type Collector struct {
	EventsIngested  prometheus.Counter
	EventsProcessed prometheus.Counter
	IssuesCreated   prometheus.Counter
	AlertsSent      *prometheus.CounterVec
	IngestDuration  prometheus.Histogram
	ProcessingLag   prometheus.Gauge
}

// New creates and registers all metrics with the default Prometheus registry.
func New() (*Collector, error) {
	c := &Collector{
		EventsIngested: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "beacon_events_ingested_total",
			Help: "Total number of events accepted by the ingest endpoint.",
		}),
		EventsProcessed: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "beacon_events_processed_total",
			Help: "Total number of events processed by the grouping worker.",
		}),
		IssuesCreated: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "beacon_issues_created_total",
			Help: "Total number of new issues created.",
		}),
		AlertsSent: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "beacon_alerts_sent_total",
			Help: "Total alerts sent, partitioned by type (new_issue/spike/regression).",
		}, []string{"type"}),
		IngestDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "beacon_ingest_duration_seconds",
			Help:    "Latency of the ingest HTTP handler.",
			Buckets: prometheus.DefBuckets,
		}),
		ProcessingLag: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "beacon_processing_lag",
			Help: "Number of unprocessed events waiting in the queue.",
		}),
	}

	collectors := []prometheus.Collector{
		c.EventsIngested,
		c.EventsProcessed,
		c.IssuesCreated,
		c.AlertsSent,
		c.IngestDuration,
		c.ProcessingLag,
	}
	for _, col := range collectors {
		if err := prometheus.Register(col); err != nil {
			return nil, err
		}
	}
	return c, nil
}

// RecordEventProcessed increments the events-processed counter.
func (c *Collector) RecordEventProcessed() { c.EventsProcessed.Inc() }

// RecordIssueCreated increments the issues-created counter.
func (c *Collector) RecordIssueCreated() { c.IssuesCreated.Inc() }

// SetProcessingLag records the current number of unprocessed events.
func (c *Collector) SetProcessingLag(n int) { c.ProcessingLag.Set(float64(n)) }

// RecordAlertSent increments the alerts-sent counter for the given alert type.
func (c *Collector) RecordAlertSent(alertType string) {
	c.AlertsSent.WithLabelValues(alertType).Inc()
}
