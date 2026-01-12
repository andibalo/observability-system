package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	once              sync.Once
	HTTPRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"service", "method", "path", "status"},
	)

	HTTPRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"service", "method", "path"},
	)

	HTTPResponseSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_response_size_bytes",
			Help:    "HTTP response size in bytes",
			Buckets: prometheus.ExponentialBuckets(100, 10, 8),
		},
		[]string{"service", "method", "path"},
	)

	OrdersCreatedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "orders_created_total",
			Help: "Total number of orders created",
		},
		[]string{"service"},
	)

	OrdersByStatusTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "orders_by_status_total",
			Help: "Total number of orders by status",
		},
		[]string{"service", "status"},
	)
)

func InitMetrics(serviceName string) {
	once.Do(func() {
		prometheus.MustRegister(HTTPRequestsTotal)
		prometheus.MustRegister(HTTPRequestDuration)
		prometheus.MustRegister(HTTPResponseSize)
		prometheus.MustRegister(OrdersCreatedTotal)
		prometheus.MustRegister(OrdersByStatusTotal)
	})
}
