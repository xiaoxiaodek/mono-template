package observability

import (
	"errors"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func TestNewMetricsCanReuseARegistryWithoutPanicking(t *testing.T) {
	registry := prometheus.NewRegistry()
	first, err := NewMetrics(registry, "1.2.3", "abc123")
	if err != nil {
		t.Fatalf("first NewMetrics: %v", err)
	}
	second, err := NewMetrics(registry, "1.2.3", "abc123")
	if err != nil {
		t.Fatalf("second NewMetrics: %v", err)
	}

	first.ObserveHTTPRequest("GET", "/users/:id", 200, 15*time.Millisecond)
	second.ObserveHTTPRequest("POST", "/users", 201, 25*time.Millisecond)

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	want := map[string]bool{
		"vort_build_info":                    false,
		"vort_http_requests_total":           false,
		"vort_http_request_duration_seconds": false,
	}
	for _, family := range families {
		if _, ok := want[family.GetName()]; ok {
			want[family.GetName()] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("metric %q was not registered", name)
		}
	}
}

func TestNewMetricsReturnsRegistererFailure(t *testing.T) {
	want := errors.New("register unavailable")
	metrics, err := NewMetrics(rejectingRegisterer{err: want}, "1.2.3", "abc123")
	if metrics != nil {
		t.Fatalf("metrics = %#v, want nil", metrics)
	}
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}

func TestNewMetricsReturnsIncompatibleCollectorCollision(t *testing.T) {
	registry := prometheus.NewRegistry()
	registry.MustRegister(prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "vort",
			Subsystem: "http",
			Name:      "requests_total",
			Help:      "An incompatible metric using the same name.",
		},
		[]string{"method"},
	))

	metrics, err := NewMetrics(registry, "1.2.3", "abc123")
	if metrics != nil {
		t.Fatalf("metrics = %#v, want nil", metrics)
	}
	if err == nil {
		t.Fatal("NewMetrics returned nil error for incompatible collector")
	}
}

type rejectingRegisterer struct{ err error }

func (r rejectingRegisterer) Register(prometheus.Collector) error  { return r.err }
func (r rejectingRegisterer) MustRegister(...prometheus.Collector) { panic("unexpected MustRegister") }
func (r rejectingRegisterer) Unregister(prometheus.Collector) bool { return false }
