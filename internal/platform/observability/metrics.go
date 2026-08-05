package observability

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics owns the service-level HTTP and build collectors.
type Metrics struct {
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
	build    *prometheus.GaugeVec
}

// NewMetrics registers the platform collectors. Reusing the same registry is
// safe: compatible already-registered collectors are reused. Any other
// registration failure is returned so startup cannot silently lose metrics.
func NewMetrics(registerer prometheus.Registerer, version, commit string) (*Metrics, error) {
	requests, err := registerCollector(registerer, prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "vort",
			Subsystem: "http",
			Name:      "requests_total",
			Help:      "Total HTTP requests processed.",
		},
		[]string{"route", "method", "status"},
	))
	if err != nil {
		return nil, fmt.Errorf("register HTTP requests metric: %w", err)
	}

	duration, err := registerCollector(registerer, prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "vort",
			Subsystem: "http",
			Name:      "request_duration_seconds",
			Help:      "HTTP request duration in seconds.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"route", "method", "status"},
	))
	if err != nil {
		return nil, fmt.Errorf("register HTTP duration metric: %w", err)
	}

	build, err := registerCollector(registerer, prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "vort",
			Name:      "build_info",
			Help:      "Application build information.",
		},
		[]string{"version", "commit"},
	))
	if err != nil {
		return nil, fmt.Errorf("register build info metric: %w", err)
	}
	build.WithLabelValues(version, commit).Set(1)

	return &Metrics{requests: requests, duration: duration, build: build}, nil
}

// ObserveHTTPRequest records one completed request. Route should be the
// normalized route template, not a raw URL path, to keep label cardinality
// bounded.
func (m *Metrics) ObserveHTTPRequest(method, route string, status int, elapsed time.Duration) {
	statusLabel := strconv.Itoa(status)
	m.requests.WithLabelValues(route, method, statusLabel).Inc()
	m.duration.WithLabelValues(route, method, statusLabel).Observe(elapsed.Seconds())
}

func registerCollector[T prometheus.Collector](registerer prometheus.Registerer, collector T) (T, error) {
	var zero T
	err := registerer.Register(collector)
	if err == nil {
		return collector, nil
	}

	var registered prometheus.AlreadyRegisteredError
	if !errors.As(err, &registered) {
		return zero, err
	}
	existing, ok := registered.ExistingCollector.(T)
	if !ok {
		return zero, fmt.Errorf("existing collector has incompatible type %T", registered.ExistingCollector)
	}
	return existing, nil
}
