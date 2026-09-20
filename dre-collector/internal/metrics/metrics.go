package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	EventsIngested = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "dre_collector_events_ingested_total",
		Help: "Total events ingested by collector",
	})
	SnapshotsExported = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "dre_collector_snapshots_total",
		Help: "Total snapshots exported",
	})
	SnapshotExportDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "dre_collector_snapshot_export_seconds",
		Help:    "Snapshot export duration in seconds",
		Buckets: prometheus.DefBuckets,
	})
	SnapshotExportFailures = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "dre_collector_snapshot_export_failures_total",
		Help: "Failed snapshot exports",
	})
	IsLeader = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "dre_collector_is_leader",
		Help: "1 when this collector pod holds the leader lease",
	})
)

func Register() {
	prometheus.MustRegister(EventsIngested, SnapshotsExported, SnapshotExportDuration, SnapshotExportFailures, IsLeader)
}

func Handler() http.Handler {
	return promhttp.Handler()
}
