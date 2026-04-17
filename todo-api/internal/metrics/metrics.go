package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	CompleteRequests = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "todo_complete_requests_total",
			Help: "PATCH /todos/{id}/complete requests grouped by resulting HTTP status.",
		},
		[]string{"status"},
	)

	CompleteLatency = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "todo_complete_latency_seconds",
			Help:    "Latency of PATCH /todos/{id}/complete in seconds.",
			Buckets: prometheus.DefBuckets,
		},
	)
)
