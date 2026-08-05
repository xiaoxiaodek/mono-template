package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"

	"github.com/vort-ads/vort-ads-template/internal/platform/response"
	"github.com/vort-ads/vort-ads-template/internal/platform/security"
)

type notifyingResponseWriter struct {
	mu      sync.Mutex
	header  http.Header
	body    bytes.Buffer
	status  int
	written chan struct{}
	once    sync.Once
}

func newNotifyingResponseWriter() *notifyingResponseWriter {
	return &notifyingResponseWriter{
		header:  make(http.Header),
		status:  http.StatusOK,
		written: make(chan struct{}),
	}
}

func (w *notifyingResponseWriter) Header() http.Header { return w.header }

func (w *notifyingResponseWriter) WriteHeader(status int) {
	w.mu.Lock()
	w.status = status
	w.mu.Unlock()
}

func (w *notifyingResponseWriter) Write(body []byte) (int, error) {
	w.mu.Lock()
	written, err := w.body.Write(body)
	w.mu.Unlock()
	w.once.Do(func() { close(w.written) })
	return written, err
}

func (w *notifyingResponseWriter) snapshot() (int, http.Header, []byte) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.status, w.header.Clone(), bytes.Clone(w.body.Bytes())
}

func TestRequestIDAddsHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestID())
	router.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	responseRecorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/ping", nil)
	router.ServeHTTP(responseRecorder, request)

	if got := responseRecorder.Header().Get("X-Request-ID"); got == "" {
		t.Fatal("missing request id header")
	}
}

func TestTimeoutReturnsGatewayTimeout(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestID(), Timeout(time.Millisecond))
	router.GET("/slow", func(c *gin.Context) {
		<-c.Request.Context().Done()
	})

	responseRecorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/slow", nil)
	router.ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusGatewayTimeout {
		t.Fatalf("status = %d, want %d", responseRecorder.Code, http.StatusGatewayTimeout)
	}
}

func TestTimeoutDoesNotWaitForHandlerThatIgnoresCancellation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestID(), Timeout(time.Millisecond))
	releaseHandler := make(chan struct{})
	router.GET("/slow", func(c *gin.Context) {
		<-releaseHandler
		c.String(http.StatusOK, "too late")
	})

	responseWriter := newNotifyingResponseWriter()
	request := httptest.NewRequest(http.MethodGet, "/slow", nil)
	requestDone := make(chan struct{})
	go func() {
		defer close(requestDone)
		router.ServeHTTP(responseWriter, request)
	}()

	select {
	case <-responseWriter.written:
	case <-time.After(250 * time.Millisecond):
		close(releaseHandler)
		<-requestDone
		t.Fatal("timeout response was not written within 250ms")
	}

	status, header, body := responseWriter.snapshot()
	if status != http.StatusGatewayTimeout {
		t.Fatalf("status = %d, want %d", status, http.StatusGatewayTimeout)
	}
	if got := header.Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want JSON", got)
	}
	var envelope response.Envelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("decode timeout response: %v", err)
	}
	if envelope.Code != "DEPENDENCY_ERROR" || envelope.Message != "request timed out" || envelope.RequestID == "" {
		t.Fatalf("unexpected timeout response: %+v", envelope)
	}

	close(releaseHandler)
	<-requestDone
}

func TestTimeoutWritesStableEnvelopeOverHTTPServer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestID(), Timeout(10*time.Millisecond))
	router.GET("/slow", func(c *gin.Context) {
		<-c.Request.Context().Done()
	})
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	responseMessage, err := server.Client().Get(server.URL + "/slow")
	if err != nil {
		t.Fatalf("GET slow endpoint: %v", err)
	}
	defer responseMessage.Body.Close()
	if responseMessage.StatusCode != http.StatusGatewayTimeout {
		t.Fatalf("status = %d, want %d", responseMessage.StatusCode, http.StatusGatewayTimeout)
	}
	var envelope response.Envelope
	if err := json.NewDecoder(responseMessage.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode timeout response: %v", err)
	}
	if envelope.Code != "DEPENDENCY_ERROR" || envelope.Message != "request timed out" || envelope.RequestID == "" {
		t.Fatalf("unexpected timeout response: %+v", envelope)
	}
}

func TestAuthRejectsRefreshToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	manager := security.NewJWTManager("test-secret-with-enough-length", time.Minute, time.Hour)
	token, err := manager.SignRefreshToken(security.Principal{UserID: "usr_1"})
	if err != nil {
		t.Fatalf("sign refresh token: %v", err)
	}

	router := gin.New()
	router.Use(RequestID(), Auth(manager))
	router.GET("/private", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	responseRecorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/private", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", responseRecorder.Code, http.StatusUnauthorized)
	}
}

func TestRequirePermissionsDeniesMissingPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(PrincipalKey, security.Principal{UserID: "usr_1", Permissions: []string{"users:read"}})
	})
	router.GET("/private", RequirePermissions("users:write"), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	responseRecorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/private", nil)
	router.ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", responseRecorder.Code, http.StatusForbidden)
	}
}

func TestRateLimitUsesIndependentClientIPBuckets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limiter := NewLocalKeyedRateLimiter(rate.Every(time.Hour), 1, time.Hour, 100)
	router := gin.New()
	router.Use(RequestID(), KeyedRateLimit(limiter, ClientIPRateLimitKey()))
	router.GET("/limited", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	request := func(ip string) int {
		responseRecorder := httptest.NewRecorder()
		httpRequest := httptest.NewRequest(http.MethodGet, "/limited", nil)
		httpRequest.RemoteAddr = ip + ":1234"
		router.ServeHTTP(responseRecorder, httpRequest)
		return responseRecorder.Code
	}

	if got := request("192.0.2.1"); got != http.StatusNoContent {
		t.Fatalf("first client first status = %d", got)
	}
	if got := request("192.0.2.1"); got != http.StatusTooManyRequests {
		t.Fatalf("first client second status = %d", got)
	}
	if got := request("192.0.2.2"); got != http.StatusNoContent {
		t.Fatalf("second client first status = %d", got)
	}
}

func TestClientIPRateLimiterBoundsAndExpiresVisitors(t *testing.T) {
	limiter := NewClientIPRateLimiter(rate.Every(time.Hour), 1, time.Minute, 2)
	now := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	limiter.now = func() time.Time { return now }

	limiter.Allow("192.0.2.1")
	limiter.Allow("192.0.2.2")
	limiter.Allow("192.0.2.3")
	if got := len(limiter.visitors); got != 2 {
		t.Fatalf("visitor count = %d, want 2", got)
	}

	now = now.Add(2 * time.Minute)
	limiter.Allow("192.0.2.4")
	if got := len(limiter.visitors); got != 1 {
		t.Fatalf("visitor count after expiry = %d, want 1", got)
	}
}
