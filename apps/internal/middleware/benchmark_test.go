package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func BenchmarkRequestID(b *testing.B) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestID())
	router.GET("/", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("X-Request-ID", "req_benchmark")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
	}
}
