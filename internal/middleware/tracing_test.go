package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestTracingCreatesRequestSpanWithCorrelation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	spanRecorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spanRecorder))
	defer func() { _ = provider.Shutdown(context.Background()) }()
	tracer := provider.Tracer("test")

	router := gin.New()
	router.Use(RequestID(), Tracing(tracer))
	router.GET("/ping", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/ping", nil)
	router.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d", response.Code)
	}

	spans := spanRecorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("ended spans = %d, want 1", len(spans))
	}
	span := spans[0]
	if span.Name() != "GET /ping" {
		t.Fatalf("span name = %q, want %q", span.Name(), "GET /ping")
	}

	gotStatus := false
	gotRequestID := false
	for _, kv := range span.Attributes() {
		switch string(kv.Key) {
		case "http.response.status_code":
			gotStatus = true
			if kv.Value.Type() != attribute.INT64 || kv.Value.AsInt64() != 204 {
				t.Fatalf("status attribute = %v, want int 204", kv.Value.AsInterface())
			}
		case "request_id":
			gotRequestID = kv.Value.AsString() != ""
		}
	}
	if !gotStatus {
		t.Fatal("missing http.response.status_code attribute")
	}
	if !gotRequestID {
		t.Fatal("request_id attribute is empty; tracing must run after RequestID")
	}
}

func TestTracingIsNoopWhenTracerNil(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestID(), Tracing(nil))
	router.GET("/ping", func(c *gin.Context) { c.Status(http.StatusOK) })

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/ping", nil)
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
}
