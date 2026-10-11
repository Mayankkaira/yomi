// Package middleware provides shared HTTP middleware: JWT validation, token
// bucket rate limiting, structured logging with request ID propagation, and CORS.
package middleware

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type ctxKey string

const (
	CtxRequestID ctxKey = "request_id"
	CtxSubjectID ctxKey = "subject_id"
	CtxRole      ctxKey = "role"
)

// ErrInvalidToken is returned when a JWT fails validation.
var ErrInvalidToken = errors.New("invalid token")

// Claims is the custom JWT payload for Yomi access tokens.
type Claims struct {
	SubjectID string `json:"sub_id"`
	Role      string `json:"role"`
	jwt.RegisteredClaims
}

// LoadPublicKey reads and parses an RSA public key in PKCS#1 PEM form.
func LoadPublicKey(path string) (*rsa.PublicKey, error) {
	if path == "" {
		return nil, errors.New("no public key path configured")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read public key: %w", err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("invalid PEM block in public key")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("public key is not RSA")
	}
	return rsaPub, nil
}

// LoadPrivateKey reads and parses an RSA private key in PEM form. Both PKCS#1
// and PKCS#8 encodings are accepted (openssl genrsa emits PKCS#8 on newer
// versions).
func LoadPrivateKey(path string) (*rsa.PrivateKey, error) {
	if path == "" {
		return nil, errors.New("no private key path configured")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read private key: %w", err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("invalid PEM block in private key")
	}
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("private key is not RSA")
		}
		return rsaKey, nil
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}
	return key, nil
}

// parseToken validates the signature and claims of a bearer token.
func parseToken(tokenString string, pub *rsa.PublicKey) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return pub, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

// JWTRequired validates the Authorization bearer token and injects the
// subject ID and role into the request context.
func JWTRequired(pub *rsa.PublicKey) gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing bearer token", "code": "UNAUTHORIZED"})
			return
		}
		claims, err := parseToken(strings.TrimPrefix(auth, "Bearer "), pub)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token", "code": "UNAUTHORIZED"})
			return
		}
		ctx := context.WithValue(c.Request.Context(), CtxSubjectID, claims.SubjectID)
		ctx = context.WithValue(ctx, CtxRole, claims.Role)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// RequireRole aborts the request unless the authenticated role matches one of
// the allowed roles. Must be used after JWTRequired.
func RequireRole(allowed ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role := Role(c.Request.Context())
		for _, a := range allowed {
			if role == a {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "insufficient permissions", "code": "FORBIDDEN"})
	}
}

// RequireAnyRole aborts the request unless the authenticated role is one of
// the allowed roles. Alias of RequireRole kept for call-site readability.
func RequireAnyRole(allowed ...string) gin.HandlerFunc {
	return RequireRole(allowed...)
}

// SubjectID returns the authenticated subject ID from the request context.
func SubjectID(ctx context.Context) string {
	id, _ := ctx.Value(CtxSubjectID).(string)
	return id
}

// Role returns the authenticated role from the request context.
func Role(ctx context.Context) string {
	role, _ := ctx.Value(CtxRole).(string)
	return role
}

// RequestID returns the propagated request ID from the context, generating a
// fresh one if absent.
func RequestID(ctx context.Context) string {
	if id, ok := ctx.Value(CtxRequestID).(string); ok && id != "" {
		return id
	}
	return uuid.NewString()
}

// tokenBucket is a leaky-token-bucket limiter for a single client key.
type tokenBucket struct {
	tokens float64
	last   time.Time
}

// RateLimiter enforces a token bucket per client IP.
type RateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*tokenBucket
	rps     float64
	burst   float64
}

// NewRateLimiter returns a limiter refilling at rps tokens/second up to burst.
func NewRateLimiter(rps float64, burst int) *RateLimiter {
	rl := &RateLimiter{
		buckets: make(map[string]*tokenBucket),
		rps:     rps,
		burst:   float64(burst),
	}
	go rl.sweep()
	return rl
}

// allow consumes one token for the key, returning false when the bucket is empty.
func (rl *RateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	b, ok := rl.buckets[key]
	if !ok {
		b = &tokenBucket{tokens: rl.burst, last: now}
		rl.buckets[key] = b
	}
	elapsed := now.Sub(b.last).Seconds()
	b.tokens = min(b.tokens+elapsed*rl.rps, rl.burst)
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// sweep periodically prunes buckets that have idled past their refill window.
func (rl *RateLimiter) sweep() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		for k, b := range rl.buckets {
			if time.Since(b.last) > 10*time.Minute {
				delete(rl.buckets, k)
			}
		}
		rl.mu.Unlock()
	}
}

// RateLimit returns a middleware that limits requests per client IP.
func (rl *RateLimiter) RateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		if !rl.allow(ip) {
			c.Header("Retry-After", "1")
			c.AbortWithStatusJSON(http.StatusTooManyRequests,
				gin.H{"error": "rate limit exceeded", "code": "RATE_LIMITED"})
			return
		}
		c.Next()
	}
}

// RequestLogger emits a structured JSON log line per request with the request
// ID propagated through the context and response header.
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := c.GetHeader("X-Request-ID")
		if rid == "" {
			rid = uuid.NewString()
		}
		ctx := context.WithValue(c.Request.Context(), CtxRequestID, rid)
		c.Request = c.Request.WithContext(ctx)
		c.Header("X-Request-ID", rid)

		start := time.Now()
		c.Next()

		slog.Info("http_request",
			"request_id", rid,
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"latency_ms", time.Since(start).Milliseconds(),
			"client_ip", c.ClientIP(),
		)
	}
}

// CORS configures permissive CORS for the gateway. In production this should
// be scoped to the frontend origin.
func CORS(allowedOrigins []string) gin.HandlerFunc {
	originSet := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		originSet[o] = true
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" && (originSet[origin] || originSet["*"]) {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Credentials", "true")
		}
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID, X-Yomi-Internal")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
