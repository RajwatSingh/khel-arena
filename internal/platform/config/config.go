// Package config loads runtime configuration from the environment.
//
// Everything is read once at startup and validated eagerly: a misconfigured
// service should fail to boot with a clear message, not fail on the first
// request that happens to touch the missing setting.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env            string // "development" | "production"
	HTTPAddr       string
	AllowedOrigins []string

	Database Database
	Auth     Auth
	Booking  Booking
}

type Database struct {
	URL             string
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
	MaxConnIdleTime time.Duration
}

type Auth struct {
	// JWTSecret signs access tokens. Must be at least 32 bytes.
	JWTSecret       []byte
	Issuer          string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

type Booking struct {
	// HoldWindow is how long an unpaid pending booking blocks its slot
	// before the janitor releases it.
	HoldWindow time.Duration
	// Timezone is the wall-clock zone arenas publish their hours in.
	Timezone *time.Location
}

// Load reads configuration from the environment, applying defaults suitable
// for local development. It returns every problem it finds at once rather
// than making the operator rerun to discover the next one.
func Load() (Config, error) {
	var problems []string
	fail := func(format string, args ...any) {
		problems = append(problems, fmt.Sprintf(format, args...))
	}

	var allowedOrigins []string
	if org := os.Getenv("CORS_ALLOWED_ORIGINS"); org != "" {
		for _, o := range strings.Split(org, ",") {
			allowedOrigins = append(allowedOrigins, strings.TrimSpace(o))
		}
	}

	cfg := Config{
		Env:            envOr("APP_ENV", "development"),
		HTTPAddr:       envOr("HTTP_ADDR", ":8080"),
		AllowedOrigins: allowedOrigins,
	}

	cfg.Database.URL = os.Getenv("DATABASE_URL")
	if cfg.Database.URL == "" {
		fail("DATABASE_URL is required (postgres://user:pass@host:5432/khel_arena)")
	}
	cfg.Database.MaxConns = int32(intEnvOr("DB_MAX_CONNS", 16, fail))
	cfg.Database.MinConns = int32(intEnvOr("DB_MIN_CONNS", 2, fail))
	cfg.Database.MaxConnLifetime = durEnvOr("DB_CONN_MAX_LIFETIME", time.Hour, fail)
	cfg.Database.MaxConnIdleTime = durEnvOr("DB_CONN_MAX_IDLE_TIME", 30*time.Minute, fail)
	if cfg.Database.MinConns > cfg.Database.MaxConns {
		fail("DB_MIN_CONNS (%d) exceeds DB_MAX_CONNS (%d)", cfg.Database.MinConns, cfg.Database.MaxConns)
	}

	secret := os.Getenv("JWT_SECRET")
	switch {
	case secret == "":
		fail("JWT_SECRET is required (generate with: openssl rand -base64 48)")
	case len(secret) < 32:
		fail("JWT_SECRET must be at least 32 bytes, got %d", len(secret))
	}
	cfg.Auth.JWTSecret = []byte(secret)
	cfg.Auth.Issuer = envOr("JWT_ISSUER", "khel-arena")
	cfg.Auth.AccessTokenTTL = durEnvOr("ACCESS_TOKEN_TTL", 15*time.Minute, fail)
	cfg.Auth.RefreshTokenTTL = durEnvOr("REFRESH_TOKEN_TTL", 30*24*time.Hour, fail)

	cfg.Booking.HoldWindow = durEnvOr("BOOKING_HOLD_WINDOW", 15*time.Minute, fail)

	tzName := envOr("ARENA_TIMEZONE", "Asia/Kathmandu")
	tz, err := time.LoadLocation(tzName)
	if err != nil {
		fail("ARENA_TIMEZONE %q is not a known zone: %v", tzName, err)
	}
	cfg.Booking.Timezone = tz

	if len(problems) > 0 {
		return Config{}, errors.New("invalid configuration:\n  - " + strings.Join(problems, "\n  - "))
	}
	return cfg, nil
}

func (c Config) IsProduction() bool { return c.Env == "production" }

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func intEnvOr(key string, fallback int, fail func(string, ...any)) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		fail("%s must be an integer, got %q", key, raw)
		return fallback
	}
	return v
}

func durEnvOr(key string, fallback time.Duration, fail func(string, ...any)) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	v, err := time.ParseDuration(raw)
	if err != nil {
		fail("%s must be a duration like 15m or 720h, got %q", key, raw)
		return fallback
	}
	if v <= 0 {
		fail("%s must be positive, got %q", key, raw)
		return fallback
	}
	return v
}
