package observability

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRegisterPprofRoutesRegistersStandardIndexWhenEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	internal := router.Group("/internal")

	RegisterPprofRoutes(internal, true)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/internal/debug/pprof/", nil)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
}

func TestRegisterPprofRoutesDoesNothingWhenDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	RegisterPprofRoutes(router, false)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}
