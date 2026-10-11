package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all runtime configuration for a service, sourced from the
// environment. Every service constructs its own Config via Load().
type Config struct {
	ServiceName string
	Port        string

	DBURL string

	RedisAddr string
	RedisPass string
	RedisDB   int

	KafkaBrokers []string

	// JWT signing keys (PEM file paths). Auth signs, every other service
	// (including the gateway) validates with the public key.
	JWTPrivateKeyPath string
	JWTPublicKeyPath  string
	AccessTokenTTL    time.Duration
	RefreshTokenTTL   time.Duration

	// Gateway routing targets (only populated in the gateway service).
	ServiceAuthURL        string
	ServiceConsentURL     string
	ServiceAuditURL       string
	ServiceInstitutionURL string

	RateLimitRPS   float64
	RateLimitBurst int

	RefreshCookieName   string
	RefreshCookieDomain string

	// HMAC secret used to sign citizen data-authorization consent tokens.
	ConsentTokenSecret string
	ConsentTokenTTL    time.Duration

	// ReceiptSecret signs the tamper-evident receipts handed to citizens. It is
	// held only by the consent service (which mints a receipt at approval time)
	// and the audit service (which can rebuild the same receipt from its own
	// ledger row), so anyone else needs both services to forge one.
	ReceiptSecret string

	// InternalServiceSecret authorizes service-to-service calls (e.g. consent
	// → institution authenticate). Must match across services.
	InternalServiceSecret string
}

// Load reads configuration from the environment, applying sane defaults so a
// local development run works with the bundled docker-compose .env.
func Load(service string) *Config {
	cfg := &Config{
		ServiceName:           service,
		Port:                  ":" + getEnv("PORT", "8080"),
		DBURL:                 os.Getenv("DB_URL"),
		RedisAddr:             getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPass:             os.Getenv("REDIS_PASSWORD"),
		RedisDB:               getIntEnv("REDIS_DB", 0),
		KafkaBrokers:          splitCsv(os.Getenv("KAFKA_BROKERS")),
		JWTPrivateKeyPath:     os.Getenv("JWT_PRIVATE_KEY_PATH"),
		JWTPublicKeyPath:      os.Getenv("JWT_PUBLIC_KEY_PATH"),
		AccessTokenTTL:        getDurEnv("ACCESS_TOKEN_TTL", 15*time.Minute),
		RefreshTokenTTL:       getDurEnv("REFRESH_TOKEN_TTL", 7*24*time.Hour),
		ServiceAuthURL:        getEnv("SERVICE_AUTH_URL", "http://localhost:8081"),
		ServiceConsentURL:     getEnv("SERVICE_CONSENT_URL", "http://localhost:8082"),
		ServiceAuditURL:       getEnv("SERVICE_AUDIT_URL", "http://localhost:8083"),
		ServiceInstitutionURL: getEnv("SERVICE_INSTITUTION_URL", "http://localhost:8084"),
		RateLimitRPS:          getFloatEnv("RATE_LIMIT_RPS", 20),
		RateLimitBurst:        getIntEnv("RATE_LIMIT_BURST", 40),
		RefreshCookieName:     getEnv("REFRESH_COOKIE_NAME", "yomi_refresh"),
		RefreshCookieDomain:   os.Getenv("REFRESH_COOKIE_DOMAIN"),
		ConsentTokenSecret:    getEnv("CONSENT_TOKEN_SECRET", "dev-only-change-me"),
		ConsentTokenTTL:       getDurEnv("CONSENT_TOKEN_TTL", 24*time.Hour),
		ReceiptSecret:         getEnv("RECEIPT_SECRET", ""),
		InternalServiceSecret: os.Getenv("INTERNAL_SERVICE_SECRET"),
	}

	if cfg.ServiceName == "gateway" {
		cfg.Port = ":8080"
	} else {
		// Each service has a fixed default port; allow override via PORT.
		cfg.Port = ":" + getEnv("PORT", defaultPort(service))
	}
	return cfg
}

func defaultPort(service string) string {
	switch service {
	case "auth":
		return "8081"
	case "consent":
		return "8082"
	case "audit":
		return "8083"
	case "institution":
		return "8084"
	default:
		return "8080"
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getIntEnv(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func getFloatEnv(key string, def float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return def
	}
	return f
}

func getDurEnv(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}

func splitCsv(s string) []string {
	if s == "" {
		return []string{"localhost:9092"}
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return []string{"localhost:9092"}
	}
	return out
}
