package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	App           AppConfig
	HTTP          HTTPConfig
	Security      SecurityConfig
	CORS          CORSConfig
	Database      DatabaseConfig
	Redis         RedisConfig
	Observability ObservabilityConfig
}

type AppConfig struct {
	Name string
	Env  string
}

type HTTPConfig struct {
	Addr            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
	MaxBodyBytes    int64
}

type SecurityConfig struct {
	JWTSecret       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	TrustedProxies  []string
}

type CORSConfig struct {
	AllowedOrigins []string
}

type DatabaseConfig struct {
	DSN             string
	DSNEnv          string
	MaxOpenConns    int32
	MaxIdleConns    int32
	ConnMaxLifetime time.Duration
}

type RedisConfig struct {
	Addr     string
	Password string
	Required bool
}

type ObservabilityConfig struct {
	PprofEnabled bool
}

func Load(configDir string) (Config, error) {
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "dev"
	}
	switch env {
	case "dev", "test", "prod":
	default:
		return Config{}, fmt.Errorf("unsupported APP_ENV %q", env)
	}

	// #nosec G304,G703 -- APP_ENV is restricted to the three literal names above.
	content, err := os.ReadFile(filepath.Join(configDir, env+".yaml"))
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	var raw rawConfig
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	decoder.KnownFields(true)
	if err := decoder.Decode(&raw); err != nil {
		return Config{}, fmt.Errorf("unmarshal config: %w", err)
	}

	cfg := raw.toConfig()
	cfg.App.Env = env
	cfg.Security.JWTSecret = os.Getenv("JWT_SECRET")
	cfg.Redis.Password = os.Getenv("REDIS_PASSWORD")

	dsnEnv := cfg.Database.DSNEnv
	if dsnEnv == "" {
		dsnEnv = "DATABASE_URL"
		cfg.Database.DSNEnv = dsnEnv
	}
	cfg.Database.DSN = os.Getenv(dsnEnv)

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c Config) Validate() error {
	if c.App.Env == "" {
		return errors.New("app env is required")
	}
	if c.HTTP.Addr == "" {
		return errors.New("http addr is required")
	}
	if c.HTTP.ReadTimeout <= 0 {
		return errors.New("http read timeout must be positive")
	}
	if c.HTTP.WriteTimeout <= 0 {
		return errors.New("http write timeout must be positive")
	}
	if c.HTTP.IdleTimeout <= 0 {
		return errors.New("http idle timeout must be positive")
	}
	if c.HTTP.ShutdownTimeout <= 0 {
		return errors.New("http shutdown timeout must be positive")
	}
	if c.HTTP.MaxBodyBytes <= 0 {
		return errors.New("http max body bytes must be positive")
	}
	if c.Security.AccessTokenTTL <= 0 {
		return errors.New("security access token ttl must be positive")
	}
	if c.Security.RefreshTokenTTL <= 0 {
		return errors.New("security refresh token ttl must be positive")
	}
	if c.Database.MaxOpenConns <= 0 {
		return errors.New("database max open conns must be positive")
	}
	if c.Database.MaxIdleConns < 0 {
		return errors.New("database max idle conns must be non-negative")
	}
	if c.Database.MaxIdleConns > c.Database.MaxOpenConns {
		return errors.New("database max idle conns must not exceed max open conns")
	}
	if c.Database.ConnMaxLifetime <= 0 {
		return errors.New("database conn max lifetime must be positive")
	}
	if c.Redis.Required && strings.TrimSpace(c.Redis.Addr) == "" {
		return errors.New("redis addr is required when redis is required")
	}
	if c.App.Env == "prod" {
		if len(c.Security.JWTSecret) < 24 {
			return errors.New("prod jwt secret must be at least 24 characters")
		}
		if strings.TrimSpace(c.Database.DSN) == "" {
			return errors.New("prod database dsn is required")
		}
		if len(c.CORS.AllowedOrigins) == 0 {
			return errors.New("prod cors allowed origins is required")
		}
		for _, origin := range c.CORS.AllowedOrigins {
			origin = strings.TrimSpace(origin)
			if origin == "" {
				return errors.New("prod cors allowed origins must not include empty string")
			}
			if origin == "*" {
				return errors.New("prod cors allowed origins must not include wildcard")
			}
		}
	}

	return nil
}

type rawConfig struct {
	App struct {
		Name string `yaml:"name"`
		Env  string `yaml:"env"`
	} `yaml:"app"`
	HTTP struct {
		Addr            string   `yaml:"addr"`
		ReadTimeout     duration `yaml:"read_timeout"`
		WriteTimeout    duration `yaml:"write_timeout"`
		IdleTimeout     duration `yaml:"idle_timeout"`
		ShutdownTimeout duration `yaml:"shutdown_timeout"`
		MaxBodyBytes    int64    `yaml:"max_body_bytes"`
	} `yaml:"http"`
	Security struct {
		AccessTokenTTL  duration `yaml:"access_token_ttl"`
		RefreshTokenTTL duration `yaml:"refresh_token_ttl"`
		TrustedProxies  []string `yaml:"trusted_proxies"`
	} `yaml:"security"`
	CORS struct {
		AllowedOrigins []string `yaml:"allowed_origins"`
	} `yaml:"cors"`
	Database struct {
		DSNEnv          string   `yaml:"dsn_env"`
		MaxOpenConns    int32    `yaml:"max_open_conns"`
		MaxIdleConns    int32    `yaml:"max_idle_conns"`
		ConnMaxLifetime duration `yaml:"conn_max_lifetime"`
	} `yaml:"database"`
	Redis struct {
		Addr     string `yaml:"addr"`
		Required bool   `yaml:"required"`
	} `yaml:"redis"`
	Observability struct {
		PprofEnabled bool `yaml:"pprof_enabled"`
	} `yaml:"observability"`
}

func (r rawConfig) toConfig() Config {
	return Config{
		App: AppConfig{
			Name: r.App.Name,
			Env:  r.App.Env,
		},
		HTTP: HTTPConfig{
			Addr:            r.HTTP.Addr,
			ReadTimeout:     time.Duration(r.HTTP.ReadTimeout),
			WriteTimeout:    time.Duration(r.HTTP.WriteTimeout),
			IdleTimeout:     time.Duration(r.HTTP.IdleTimeout),
			ShutdownTimeout: time.Duration(r.HTTP.ShutdownTimeout),
			MaxBodyBytes:    r.HTTP.MaxBodyBytes,
		},
		Security: SecurityConfig{
			AccessTokenTTL:  time.Duration(r.Security.AccessTokenTTL),
			RefreshTokenTTL: time.Duration(r.Security.RefreshTokenTTL),
			TrustedProxies:  r.Security.TrustedProxies,
		},
		CORS: CORSConfig{
			AllowedOrigins: r.CORS.AllowedOrigins,
		},
		Database: DatabaseConfig{
			DSNEnv:          r.Database.DSNEnv,
			MaxOpenConns:    r.Database.MaxOpenConns,
			MaxIdleConns:    r.Database.MaxIdleConns,
			ConnMaxLifetime: time.Duration(r.Database.ConnMaxLifetime),
		},
		Redis: RedisConfig{
			Addr:     r.Redis.Addr,
			Required: r.Redis.Required,
		},
		Observability: ObservabilityConfig{
			PprofEnabled: r.Observability.PprofEnabled,
		},
	}
}

type duration time.Duration

func (d *duration) UnmarshalYAML(value *yaml.Node) error {
	parsed, err := time.ParseDuration(value.Value)
	if err != nil {
		return fmt.Errorf("parse duration %q: %w", value.Value, err)
	}

	*d = duration(parsed)
	return nil
}
