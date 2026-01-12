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

	InventoryChecksTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "inventory_checks_total",
			Help: "Total number of inventory check requests",
		},
		[]string{"service"},
	)

	StockReservationsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "stock_reservations_total",
			Help: "Total number of stock reservation requests",
		},
		[]string{"service", "status"},
	)
)

func InitMetrics(serviceName string) {
	once.Do(func() {
		prometheus.MustRegister(HTTPRequestsTotal)
		prometheus.MustRegister(HTTPRequestDuration)
		prometheus.MustRegister(HTTPResponseSize)
		prometheus.MustRegister(InventoryChecksTotal)
		prometheus.MustRegister(StockReservationsTotal)
	})
}
