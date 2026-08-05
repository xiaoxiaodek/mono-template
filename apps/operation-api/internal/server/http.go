package server

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	swaggerfiles "github.com/swaggo/files"
	ginswagger "github.com/swaggo/gin-swagger"
	swag "github.com/swaggo/swag"
	"go.opentelemetry.io/otel/trace"

	_ "github.com/vort-ads/vort-ads-template/apps/operation-api/docs"
	identityservice "github.com/vort-ads/vort-ads-template/apps/operation-api/internal/service/identity"
	"github.com/vort-ads/vort-ads-template/internal/middleware"
	"github.com/vort-ads/vort-ads-template/internal/platform/apperrors"
	"github.com/vort-ads/vort-ads-template/internal/platform/observability"
	"github.com/vort-ads/vort-ads-template/internal/platform/response"
)

const (
	defaultMaxBodyBytes  = int64(1 << 20)
	defaultRequestTimout = 30 * time.Second
)

type ReadinessCheck func(context.Context) error

type Dependencies struct {
	Logger               zerolog.Logger
	IdentityHandler      *identityservice.Handler
	ReadinessChecks      map[string]ReadinessCheck
	Metrics              *observability.Metrics
	MetricsHandler       http.Handler
	GlobalRateLimiter    middleware.KeyedRateLimiter
	ClientIPKeyedLimiter middleware.KeyedRateLimiter
	UserRateLimiter      middleware.KeyedRateLimiter
	AuthIPRateLimiter    middleware.KeyedRateLimiter
	CORSConfig           middleware.CORSConfig
	TrustedProxies       []string
	MaxBodyBytes         int64
	RequestTimeout       time.Duration
	PprofEnabled         bool
	SwaggerEnabled       bool
	Tracer               trace.Tracer
}

func NewHTTPServer(dependencies Dependencies) *gin.Engine {
	engine := gin.New()
	if err := engine.SetTrustedProxies(dependencies.TrustedProxies); err != nil {
		panic("configure trusted proxies: " + err.Error())
	}

	maxBodyBytes := dependencies.MaxBodyBytes
	if maxBodyBytes <= 0 {
		maxBodyBytes = defaultMaxBodyBytes
	}
	requestTimeout := dependencies.RequestTimeout
	if requestTimeout <= 0 {
		requestTimeout = defaultRequestTimout
	}

	engine.Use(
		middleware.RequestID(),
		middleware.SecurityHeaders(),
		middleware.Tracing(dependencies.Tracer),
		middleware.Recovery(dependencies.Logger),
		middleware.AccessLog(dependencies.Logger),
		middleware.CORS(dependencies.CORSConfig),
		middleware.BodyLimit(maxBodyBytes),
		middleware.Timeout(requestTimeout),
		observeRequests(dependencies.Metrics),
	)

	engine.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, response.OK(middleware.GetRequestID(c), gin.H{"status": "ok"}))
	})
	engine.GET("/readyz", readinessHandler(dependencies.ReadinessChecks))

	metricsHandler := dependencies.MetricsHandler
	if metricsHandler == nil {
		metricsHandler = promhttp.Handler()
	}
	engine.GET("/metrics", gin.WrapH(metricsHandler))

	if dependencies.PprofEnabled {
		observability.RegisterPprofRoutes(engine, true)
	}
	if dependencies.SwaggerEnabled {
		engine.GET("/swagger/doc.json", func(c *gin.Context) {
			doc, err := swag.ReadDoc()
			if err != nil {
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}
			c.Header("Content-Type", "application/json; charset=utf-8")
			c.String(http.StatusOK, doc)
		})
		engine.GET("/docs/*any", ginswagger.WrapHandler(swaggerfiles.Handler,
			ginswagger.URL("/swagger/doc.json"),
		))
	}
	if dependencies.IdentityHandler != nil {
		api := engine.Group("/api/v1")
		api.Use(middleware.KeyedRateLimit(dependencies.GlobalRateLimiter, middleware.GlobalRateLimitKey("operation-api")))
		api.Use(middleware.KeyedRateLimit(dependencies.ClientIPKeyedLimiter, middleware.ClientIPRateLimitKey()))
		api.Use(middleware.KeyedRateLimit(dependencies.AuthIPRateLimiter, middleware.AuthEndpointIPKey()))
		dependencies.IdentityHandler.RegisterRoutes(
			api,
			middleware.KeyedRateLimit(dependencies.UserRateLimiter, middleware.AuthenticatedUserRateLimitKey()),
		)
	}

	return engine
}

func readinessHandler(checks map[string]ReadinessCheck) gin.HandlerFunc {
	return func(c *gin.Context) {
		results := make(map[string]string, len(checks))
		ready := true
		for name, check := range checks {
			if check == nil || check(c.Request.Context()) == nil {
				results[name] = "ok"
				continue
			}
			results[name] = "unavailable"
			ready = false
		}

		if !ready {
			c.JSON(http.StatusServiceUnavailable, response.Error(
				middleware.GetRequestID(c), apperrors.CodeDependencyError, "required dependency unavailable",
			))
			return
		}
		c.JSON(http.StatusOK, response.OK(middleware.GetRequestID(c), gin.H{
			"status": "ready", "checks": results,
		}))
	}
}

func observeRequests(metrics *observability.Metrics) gin.HandlerFunc {
	return func(c *gin.Context) {
		started := time.Now()
		c.Next()
		if metrics == nil {
			return
		}
		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		metrics.ObserveHTTPRequest(c.Request.Method, route, c.Writer.Status(), time.Since(started))
	}
}
