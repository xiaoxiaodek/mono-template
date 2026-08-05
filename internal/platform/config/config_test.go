package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadAppliesEnvironmentAndTestYAML(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("TEST_DATABASE_URL", "postgres://user:pass@localhost:5432/vort_test?sslmode=disable")
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/vort_dev?sslmode=disable")
	t.Setenv("REDIS_PASSWORD", "redis-secret")

	cfg, err := Load(filepath.Join("..", "..", "..", "configs"))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.App.Env != "test" {
		t.Fatalf("App.Env = %q, want test", cfg.App.Env)
	}
	if cfg.HTTP.Addr != ":0" {
		t.Fatalf("HTTP.Addr = %q, want :0", cfg.HTTP.Addr)
	}
	if cfg.HTTP.ReadTimeout != 2*time.Second {
		t.Fatalf("HTTP.ReadTimeout = %s, want 2s", cfg.HTTP.ReadTimeout)
	}
	if cfg.Security.JWTSecret != "test-secret" {
		t.Fatalf("Security.JWTSecret = %q, want env JWT secret", cfg.Security.JWTSecret)
	}
	if cfg.Security.AccessTokenTTL != 5*time.Minute {
		t.Fatalf("Security.AccessTokenTTL = %s, want 5m", cfg.Security.AccessTokenTTL)
	}
	// #nosec G101 -- fixed non-production credential is a config-loading test fixture.
	if cfg.Database.DSN != "postgres://user:pass@localhost:5432/vort_test?sslmode=disable" {
		t.Fatalf("Database.DSN = %q, want TEST_DATABASE_URL value", cfg.Database.DSN)
	}
	if cfg.Redis.Password != "redis-secret" {
		t.Fatalf("Redis.Password = %q, want REDIS_PASSWORD env value", cfg.Redis.Password)
	}
}

func TestLoadDoesNotFallBackToDatabaseURLWhenDSNEnvIsExplicit(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("TEST_DATABASE_URL", "")
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/vort_dev?sslmode=disable")

	cfg, err := Load(filepath.Join("..", "..", "..", "configs"))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Database.DSNEnv != "TEST_DATABASE_URL" {
		t.Fatalf("Database.DSNEnv = %q, want TEST_DATABASE_URL", cfg.Database.DSNEnv)
	}
	if cfg.Database.DSN != "" {
		t.Fatalf("Database.DSN = %q, want empty when TEST_DATABASE_URL is unset", cfg.Database.DSN)
	}
}

func TestLoadRejectsUnknownYAMLFields(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APP_ENV", "test")
	t.Setenv("JWT_SECRET", "test-secret")
	writeConfig(t, dir, "test.yaml", `app:
  name: vort-ads-operation-api
http:
  addr: ":0"
  read_timeout: 2s
  write_timeout: 2s
  idle_timeout: 10s
  shutdown_timeout: 2s
  max_body_bytes: 1048576
  unknown_field: true
security:
  access_token_ttl: 5m
  refresh_token_ttl: 1h
cors:
  allowed_origins: ["http://localhost:5173"]
database:
  dsn_env: "TEST_DATABASE_URL"
  max_open_conns: 5
  max_idle_conns: 2
  conn_max_lifetime: 5m
redis:
  required: false
  ratelimit_fail_open: false
observability:
  pprof_enabled: false
  swagger_enabled: true
`)

	_, err := Load(dir)
	if err == nil {
		t.Fatal("Load returned nil error, want unknown YAML field rejection")
	}
	if !strings.Contains(err.Error(), "unknown_field") {
		t.Fatalf("Load error = %q, want unknown field name", err)
	}
}

func TestLoadRejectsProdWithoutJWTSecret(t *testing.T) {
	t.Setenv("APP_ENV", "prod")
	t.Setenv("JWT_SECRET", "")
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/vort?sslmode=disable")

	_, err := Load(filepath.Join("..", "..", "..", "configs"))
	if err == nil {
		t.Fatal("Load returned nil error, want prod JWT secret validation error")
	}
}

func TestLoadRejectsUnsupportedEnvironmentBeforeReadingConfig(t *testing.T) {
	t.Setenv("APP_ENV", "../prod")

	_, err := Load(filepath.Join("..", "..", "..", "configs"))
	if err == nil {
		t.Fatal("Load returned nil error, want unsupported environment error")
	}
	if !strings.Contains(err.Error(), "unsupported APP_ENV") {
		t.Fatalf("Load error = %q, want unsupported APP_ENV", err)
	}
}

func TestValidateRejectsInvalidRuntimeSettings(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
		want   string
	}{
		{
			name: "http max body bytes must be positive",
			mutate: func(cfg *Config) {
				cfg.HTTP.MaxBodyBytes = 0
			},
			want: "http max body bytes must be positive",
		},
		{
			name: "database max open conns must be positive",
			mutate: func(cfg *Config) {
				cfg.Database.MaxOpenConns = 0
			},
			want: "database max open conns must be positive",
		},
		{
			name: "database max idle conns must be non-negative",
			mutate: func(cfg *Config) {
				cfg.Database.MaxIdleConns = -1
			},
			want: "database max idle conns must be non-negative",
		},
		{
			name: "database max idle conns cannot exceed max open conns",
			mutate: func(cfg *Config) {
				cfg.Database.MaxOpenConns = 2
				cfg.Database.MaxIdleConns = 3
			},
			want: "database max idle conns must not exceed max open conns",
		},
		{
			name: "database conn max lifetime must be positive",
			mutate: func(cfg *Config) {
				cfg.Database.ConnMaxLifetime = 0
			},
			want: "database conn max lifetime must be positive",
		},
		{
			name: "prod database dsn must be present",
			mutate: func(cfg *Config) {
				cfg.Database.DSN = ""
			},
			want: "prod database dsn is required",
		},
		{
			name: "required redis must have addr",
			mutate: func(cfg *Config) {
				cfg.Redis.Addr = ""
			},
			want: "redis addr is required when redis is required",
		},
		{
			name: "prod cors origins must be present",
			mutate: func(cfg *Config) {
				cfg.CORS.AllowedOrigins = nil
			},
			want: "prod cors allowed origins is required",
		},
		{
			name: "prod cors origins cannot include wildcard",
			mutate: func(cfg *Config) {
				cfg.CORS.AllowedOrigins = []string{"https://admin.example.com", "*"}
			},
			want: "prod cors allowed origins must not include wildcard",
		},
		{
			name: "prod cors origins cannot include empty string",
			mutate: func(cfg *Config) {
				cfg.CORS.AllowedOrigins = []string{"https://admin.example.com", ""}
			},
			want: "prod cors allowed origins must not include empty string",
		},
		{
			name: "prod pprof must be disabled",
			mutate: func(cfg *Config) {
				cfg.Observability.PprofEnabled = true
			},
			want: "prod pprof must be disabled",
		},
		{
			name: "prod swagger must be disabled",
			mutate: func(cfg *Config) {
				cfg.Observability.SwaggerEnabled = true
			},
			want: "prod swagger must be disabled",
		},
		{
			name: "tracing enabled requires endpoint",
			mutate: func(cfg *Config) {
				cfg.Observability.Tracing = TracingConfig{Enabled: true}
			},
			want: "tracing endpoint is required when tracing is enabled",
		},
		{
			name: "tracing sampling ratio must be within [0, 1]",
			mutate: func(cfg *Config) {
				cfg.Observability.Tracing = TracingConfig{Enabled: true, Endpoint: "http://collector:4318", SamplingRatio: 1.5}
			},
			want: "tracing sampling ratio must be within [0, 1]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validProdConfig()
			tt.mutate(&cfg)

			err := cfg.Validate()
			if err == nil {
				t.Fatal("Validate returned nil error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate error = %q, want %q", err, tt.want)
			}
		})
	}
}

func validProdConfig() Config {
	return Config{
		App: AppConfig{
			Name: "vort-ads-operation-api",
			Env:  "prod",
		},
		HTTP: HTTPConfig{
			Addr:            ":8080",
			ReadTimeout:     5 * time.Second,
			WriteTimeout:    10 * time.Second,
			IdleTimeout:     60 * time.Second,
			ShutdownTimeout: 15 * time.Second,
			MaxBodyBytes:    1 << 20,
		},
		Security: SecurityConfig{
			JWTSecret:       "test-secret-with-enough-length",
			AccessTokenTTL:  15 * time.Minute,
			RefreshTokenTTL: 168 * time.Hour,
		},
		CORS: CORSConfig{
			AllowedOrigins: []string{"https://admin.example.com"},
		},
		Database: DatabaseConfig{
			DSN:             "postgres://user:pass@localhost:5432/vort?sslmode=disable", // #nosec G101 -- test fixture.
			DSNEnv:          "DATABASE_URL",
			MaxOpenConns:    50,
			MaxIdleConns:    10,
			ConnMaxLifetime: 30 * time.Minute,
		},
		Redis: RedisConfig{
			Addr:              "redis:6379",
			Required:          true,
			RateLimitFailOpen: false,
		},
		Observability: ObservabilityConfig{
			PprofEnabled:   false,
			SwaggerEnabled: false,
		},
	}
}

func writeConfig(t *testing.T, dir string, name string, content string) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
}
