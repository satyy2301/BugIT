package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	EventsEmitted = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "dre_events_emitted_total",
		Help: "Total io_events emitted from agent",
	})
	RingbufDrops = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "dre_ringbuf_drops_total",
		Help: "Ring buffer drops due to congestion",
	})
	BypassMode = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "dre_bypass_mode",
		Help: "1 when agent is in bypass mode",
	})
)

func Register() {
	prometheus.MustRegister(EventsEmitted, RingbufDrops, BypassMode)
}

func Handler() http.Handler {
	return promhttp.Handler()
}
