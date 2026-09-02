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

	// AppURL is the interface's own origin, used to build links that appear
	// in email and to send a payer back after a gateway. Configuration rather
	// than anything read off a request: deriving it from a Host header would
	// let whoever sent the request choose the domain in an email.
	AppURL string

	Database Database
	Auth     Auth
	Booking  Booking
	Mail     Mail
	Payment  Payment
	Media    Media
}

// Media configures where uploaded images are kept.
//
// A directory on disk, served by this process. That is the right shape at this
// size and the wrong one behind more than one instance -- see
// internal/platform/media for the decision written out. An empty Dir turns
// uploads off rather than half-enabling them.
type Media struct {
	Dir    string
	Prefix string
}

func (m Media) Configured() bool { return m.Dir != "" }

// Mail configures transactional email. With no SMTP host set, cmd/api uses a
// sender that logs instead -- fine for development, and refused in production.
type Mail struct {
	SMTPHost string
	SMTPPort int
	Username string
	Password string
	From     string
	FromName string
}

// Configured reports whether a real mail server was named.
func (m Mail) Configured() bool { return m.SMTPHost != "" && m.From != "" }

// Payment configures the gateways. A provider whose credentials are absent is
// simply not offered, which is why these are not required: a deployment that
// takes only cash is a real deployment.
type Payment struct {
	EsewaSecretKey   []byte
	EsewaProductCode string
	EsewaFormURL     string
	EsewaStatusURL   string

	KhaltiSecretKey string
	KhaltiBaseURL   string
}

func (p Payment) EsewaConfigured() bool {
	return len(p.EsewaSecretKey) > 0 && p.EsewaProductCode != ""
}

func (p Payment) KhaltiConfigured() bool { return p.KhaltiSecretKey != "" }

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
		AppURL:         strings.TrimRight(envOr("APP_URL", "http://localhost:5173"), "/"),
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

	cfg.Media = Media{
		Dir:    os.Getenv("MEDIA_DIR"),
		Prefix: envOr("MEDIA_URL_PREFIX", "/media"),
	}

	tzName := envOr("ARENA_TIMEZONE", "Asia/Kathmandu")
	tz, err := time.LoadLocation(tzName)
	if err != nil {
		fail("ARENA_TIMEZONE %q is not a known zone: %v", tzName, err)
	}
	cfg.Booking.Timezone = tz

	cfg.Mail = Mail{
		SMTPHost: os.Getenv("SMTP_HOST"),
		SMTPPort: intEnvOr("SMTP_PORT", 587, fail),
		Username: os.Getenv("SMTP_USERNAME"),
		Password: os.Getenv("SMTP_PASSWORD"),
		From:     os.Getenv("MAIL_FROM"),
		FromName: envOr("MAIL_FROM_NAME", "Khel Arena"),
	}

	cfg.Payment = Payment{
		EsewaSecretKey:   []byte(os.Getenv("ESEWA_SECRET_KEY")),
		EsewaProductCode: os.Getenv("ESEWA_PRODUCT_CODE"),
		EsewaFormURL:     os.Getenv("ESEWA_FORM_URL"),
		EsewaStatusURL:   os.Getenv("ESEWA_STATUS_URL"),
		KhaltiSecretKey:  os.Getenv("KHALTI_SECRET_KEY"),
		KhaltiBaseURL:    os.Getenv("KHALTI_BASE_URL"),
	}

	// A half-configured gateway is worse than an absent one: it is offered to
	// players and then fails at the moment they try to pay.
	if cfg.Payment.EsewaConfigured() && (cfg.Payment.EsewaFormURL == "" || cfg.Payment.EsewaStatusURL == "") {
		fail("ESEWA_FORM_URL and ESEWA_STATUS_URL are required when ESEWA_SECRET_KEY is set")
	}
	if cfg.Payment.KhaltiConfigured() && cfg.Payment.KhaltiBaseURL == "" {
		fail("KHALTI_BASE_URL is required when KHALTI_SECRET_KEY is set")
	}

	// Production has to be able to send mail. Password reset is the only way
	// back into a locked-out account, and a deployment that logs the reset
	// link instead of sending it has no recovery path at all.
	if cfg.IsProduction() && !cfg.Mail.Configured() {
		fail("SMTP_HOST and MAIL_FROM are required when APP_ENV=production (password reset needs to send email)")
	}

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
