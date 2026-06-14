package handler

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type metrics struct {
	reg            *prometheus.Registry
	connectedUsers prometheus.Gauge
	answersTotal   *prometheus.CounterVec
	scoreLatency   prometheus.Histogram
}

func newMetrics(activeSessions func() int) *metrics {
	reg := prometheus.NewRegistry()
	m := &metrics{
		reg: reg,
		connectedUsers: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "quiz_connected_users", Help: "Currently connected WebSocket users.",
		}),
		answersTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "quiz_answers_total", Help: "Answers processed, by outcome.",
		}, []string{"outcome"}),
		scoreLatency: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name: "quiz_score_update_seconds", Help: "Answer processing + broadcast latency.",
			Buckets: []float64{.001, .005, .01, .05, .1, .25, .5, 1},
		}),
	}
	reg.MustRegister(
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "quiz_active_sessions", Help: "Quiz sessions currently active.",
		}, func() float64 { return float64(activeSessions()) }),
		m.connectedUsers, m.answersTotal, m.scoreLatency,
		collectors.NewGoCollector(),
	)
	return m
}

func (m *metrics) handler() http.Handler {
	return promhttp.HandlerFor(m.reg, promhttp.HandlerOpts{})
}

// record observes an answer outcome and its end-to-end latency.
func (m *metrics) record(start time.Time, outcome string) {
	m.answersTotal.WithLabelValues(outcome).Inc()
	m.scoreLatency.Observe(time.Since(start).Seconds())
}
