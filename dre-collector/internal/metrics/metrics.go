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
)

func Register() {
	prometheus.MustRegister(EventsIngested, SnapshotsExported)
}

func Handler() http.Handler {
	return promhttp.Handler()
}
