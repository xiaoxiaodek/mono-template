package bootstrap

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/time/rate"

	bizidentity "github.com/vort-ads/vort-ads-template/apps/operation-api/internal/biz/identity"
	identitydata "github.com/vort-ads/vort-ads-template/apps/operation-api/internal/data/identity"
	"github.com/vort-ads/vort-ads-template/apps/operation-api/internal/data/identity/memory"
	"github.com/vort-ads/vort-ads-template/apps/operation-api/internal/data/identity/postgres"
	identityredis "github.com/vort-ads/vort-ads-template/apps/operation-api/internal/data/identity/redis"
	"github.com/vort-ads/vort-ads-template/apps/operation-api/internal/server"
	identityservice "github.com/vort-ads/vort-ads-template/apps/operation-api/internal/service/identity"
	"github.com/vort-ads/vort-ads-template/internal/middleware"
	"github.com/vort-ads/vort-ads-template/internal/platform/cache"
	"github.com/vort-ads/vort-ads-template/internal/platform/config"
	"github.com/vort-ads/vort-ads-template/internal/platform/database"
	"github.com/vort-ads/vort-ads-template/internal/platform/logger"
	"github.com/vort-ads/vort-ads-template/internal/platform/observability"
	platformruntime "github.com/vort-ads/vort-ads-template/internal/platform/runtime"
	"github.com/vort-ads/vort-ads-template/internal/platform/security"
	"github.com/vort-ads/vort-ads-template/pkg/idgen"
)

const (
	dependencyCheckTimeout = 2 * time.Second
	requestTimeoutHeadroom = 500 * time.Millisecond
	rateLimitKeyTTL        = 10 * time.Minute
	globalRatePerSecond    = int64(1_000)
	globalRateBurst        = int64(2_000)
	clientIPRatePerSecond  = int64(100)
	clientIPRateBurst      = int64(200)
	userRatePerSecond      = int64(50)
	userRateBurst          = int64(100)
	// Auth endpoints use stricter limits to prevent brute-force attacks.
	authIPRatePerSecond = int64(1)
	authIPRateBurst     = int64(10)
)

var (
	Version = "dev"
	Commit  = "unknown"
)

type App struct {
	Config config.Config
	Logger zerolog.Logger
	Server *http.Server

	database       *pgxpool.Pool
	redis          *redis.Client
	tracerProvider *sdktrace.TracerProvider
}

func New(ctx context.Context, configDir string) (*App, error) {
	cfg, err := config.Load(configDir)
	if err != nil {
		return nil, fmt.Errorf("load configuration: %w", err)
	}
	log := logger.New(cfg.App.Env)
	configureGinMode(cfg.App.Env)

	tracerProvider, err := observability.NewTracerProvider(ctx, cfg.App.Name, cfg.Observability.Tracing)
	if err != nil {
		return nil, fmt.Errorf("create tracer provider: %w", err)
	}
	var tracer trace.Tracer
	if tracerProvider != nil {
		tracer = tracerProvider.Tracer(cfg.App.Name)
	}

	var pool *pgxpool.Pool
	var redisClient *redis.Client
	if strings.TrimSpace(cfg.Database.DSN) != "" {
		pool, err = database.New(ctx, cfg.Database)
		if err != nil {
			shutdownStartup(ctx, tracerProvider, pool, nil)
			return nil, err
		}
	}

	redisAvailable := false
	if strings.TrimSpace(cfg.Redis.Addr) != "" {
		redisClient = cache.New(cfg.Redis)
		checkCtx, cancel := context.WithTimeout(ctx, dependencyCheckTimeout)
		pingErr := cache.Ping(checkCtx, redisClient)
		cancel()
		redisAvailable = pingErr == nil
		if pingErr != nil && !cfg.Redis.Required {
			log.Warn().Err(pingErr).Msg("optional Redis unavailable; using local refresh token fallback")
			_ = redisClient.Close()
			redisClient = nil
		}
	}

	registry := prometheus.NewRegistry()
	metrics, err := observability.NewMetrics(registry, Version, Commit)
	if err != nil {
		shutdownStartup(ctx, tracerProvider, pool, redisClient)
		return nil, fmt.Errorf("create metrics: %w", err)
	}

	jwtManager := security.NewJWTManager(
		cfg.Security.JWTSecret,
		cfg.Security.AccessTokenTTL,
		cfg.Security.RefreshTokenTTL,
	)
	var users bizidentity.UserRepository = memory.NewUserRepository()
	var registration bizidentity.RegistrationUnitOfWork
	if pool != nil {
		postgresUsers := postgres.NewUserRepository(pool)
		users = postgresUsers
		registration = postgresUsers
	}
	identityUsecase := bizidentity.NewUsecase(
		users,
		selectRefreshTokenStore(pool, redisClient, redisAvailable, cfg.Redis.Required),
		security.BcryptPasswordHasher{},
		identitydata.NewTokenManager(jwtManager),
		idgen.New,
		registration,
	)
	identityHandler := identityservice.NewHandler(identityUsecase, middleware.Auth(jwtManager))

	readinessChecks := make(map[string]server.ReadinessCheck)
	if pool != nil {
		readinessChecks["postgres"] = func(ctx context.Context) error {
			checkCtx, cancel := context.WithTimeout(ctx, dependencyCheckTimeout)
			defer cancel()
			return database.Ping(checkCtx, pool)
		}
	}
	if cfg.Redis.Required && redisClient != nil {
		readinessChecks["redis"] = func(ctx context.Context) error {
			checkCtx, cancel := context.WithTimeout(ctx, dependencyCheckTimeout)
			defer cancel()
			return cache.Ping(checkCtx, redisClient)
		}
	}

	rateLimitPolicies := selectRateLimitPolicies(redisClient, redisAvailable, cfg.Redis.Required, cfg.Redis.RateLimitFailOpen)
	handler := server.NewHTTPServer(server.Dependencies{
		Logger:               log,
		IdentityHandler:      &identityHandler,
		ReadinessChecks:      readinessChecks,
		Metrics:              metrics,
		MetricsHandler:       promhttp.HandlerFor(registry, promhttp.HandlerOpts{}),
		GlobalRateLimiter:    rateLimitPolicies.global,
		ClientIPKeyedLimiter: rateLimitPolicies.clientIP,
		UserRateLimiter:      rateLimitPolicies.user,
		AuthIPRateLimiter:    rateLimitPolicies.authIP,
		CORSConfig: middleware.CORSConfig{
			AllowedOrigins:   cfg.CORS.AllowedOrigins,
			AllowedMethods:   cfg.CORS.AllowedMethods,
			AllowedHeaders:   cfg.CORS.AllowedHeaders,
			AllowCredentials: cfg.CORS.AllowCredentials,
			MaxAge:           cfg.CORS.MaxAge,
		},
		TrustedProxies: cfg.Security.TrustedProxies,
		MaxBodyBytes:   cfg.HTTP.MaxBodyBytes,
		RequestTimeout: requestTimeoutFor(cfg.HTTP.WriteTimeout),
		PprofEnabled:   cfg.Observability.PprofEnabled,
		SwaggerEnabled: cfg.Observability.SwaggerEnabled,
		Tracer:         tracer,
	})

	return &App{
		Config:         cfg,
		Logger:         log,
		Server:         platformruntime.NewHTTPServer(cfg.HTTP, handler),
		database:       pool,
		redis:          redisClient,
		tracerProvider: tracerProvider,
	}, nil
}

type apiRateLimitPolicies struct {
	global   middleware.KeyedRateLimiter
	clientIP middleware.KeyedRateLimiter
	user     middleware.KeyedRateLimiter
	authIP   middleware.KeyedRateLimiter
}

func selectRateLimitPolicies(client *redis.Client, available, required bool, failOpen bool) apiRateLimitPolicies {
	if client != nil && (available || required) {
		policies := apiRateLimitPolicies{
			global: middleware.NewRedisTokenBucketRateLimiter(
				client, "vort-ads:ratelimit", globalRatePerSecond, globalRateBurst,
			),
			clientIP: middleware.NewRedisTokenBucketRateLimiter(
				client, "vort-ads:ratelimit", clientIPRatePerSecond, clientIPRateBurst,
			),
			user: middleware.NewRedisTokenBucketRateLimiter(
				client, "vort-ads:ratelimit", userRatePerSecond, userRateBurst,
			),
			authIP: middleware.NewRedisTokenBucketRateLimiter(
				client, "vort-ads:ratelimit", authIPRatePerSecond, authIPRateBurst,
			),
		}
		// When fail-open is configured, Redis errors allow requests through
		// instead of returning 503. Use this when availability outranks strict
		// rate enforcement.
		if failOpen {
			policies.global = middleware.WithFailOpen(policies.global)
			policies.clientIP = middleware.WithFailOpen(policies.clientIP)
			policies.user = middleware.WithFailOpen(policies.user)
			policies.authIP = middleware.WithFailOpen(policies.authIP)
		}
		return policies
	}

	return apiRateLimitPolicies{
		global: middleware.NewLocalKeyedRateLimiter(
			rate.Limit(globalRatePerSecond), int(globalRateBurst), rateLimitKeyTTL, 2,
		),
		clientIP: middleware.NewLocalKeyedRateLimiter(
			rate.Limit(clientIPRatePerSecond), int(clientIPRateBurst), rateLimitKeyTTL, 10_000,
		),
		user: middleware.NewLocalKeyedRateLimiter(
			rate.Limit(userRatePerSecond), int(userRateBurst), rateLimitKeyTTL, 50_000,
		),
		authIP: middleware.NewLocalKeyedRateLimiter(
			rate.Limit(authIPRatePerSecond), int(authIPRateBurst), rateLimitKeyTTL, 10_000,
		),
	}
}

func requestTimeoutFor(writeTimeout time.Duration) time.Duration {
	headroom := requestTimeoutHeadroom
	if headroom >= writeTimeout {
		headroom = writeTimeout / 2
	}
	return writeTimeout - headroom
}

func selectRefreshTokenStore(pool *pgxpool.Pool, client *redis.Client, available, required bool) bizidentity.RefreshTokenStore {
	if pool != nil {
		return postgres.NewRefreshTokenStore(pool)
	}
	if client != nil && (available || required) {
		return identityredis.NewTokenStore(client)
	}
	return memory.NewTokenStore()
}

func configureGinMode(environment string) {
	if strings.EqualFold(strings.TrimSpace(environment), "prod") {
		gin.SetMode(gin.ReleaseMode)
	}
}

func (a *App) Shutdown(ctx context.Context) error {
	err := platformruntime.Shutdown(ctx, a.Server, a.Config.HTTP.ShutdownTimeout)
	shutdownStartup(ctx, a.tracerProvider, a.database, a.redis)
	return err
}

// shutdownStartup cleans up infrastructure dependencies. It is safe to call
// with any nil argument and must be called on both successful startup (via
// App.Shutdown) and early-return error paths inside New.
func shutdownStartup(ctx context.Context, tracerProvider *sdktrace.TracerProvider, pool *pgxpool.Pool, redisClient *redis.Client) {
	if redisClient != nil {
		_ = redisClient.Close()
	}
	if pool != nil {
		pool.Close()
	}
	if tracerProvider != nil {
		_ = observability.ShutdownTracer(ctx, tracerProvider)
	}
}
