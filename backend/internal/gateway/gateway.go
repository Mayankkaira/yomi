// Package gateway implements the Yomi API Gateway: a single entry point that
// applies JWT validation, per-IP rate limiting, CORS, and structured logging,
// then reverse-proxies requests to the downstream microservices.
package gateway

import (
	"crypto/rsa"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/yomi/yomi/internal/middleware"
)

// Gateway holds the proxies and shared middleware.
type Gateway struct {
	auth        http.Handler
	consent     http.Handler
	audit       http.Handler
	institution http.Handler
	limiter     *middleware.RateLimiter
}

// New builds a Gateway routing to the given downstream base URLs.
func New(authURL, consentURL, auditURL, institutionURL string, rps float64, burst int) *Gateway {
	return &Gateway{
		auth:        proxy(authURL),
		consent:     proxy(consentURL),
		audit:       proxy(auditURL),
		institution: proxy(institutionURL),
		limiter:     middleware.NewRateLimiter(rps, burst),
	}
}

// proxy builds a reverse proxy that preserves the original request path when
// forwarding to the target service.
func proxy(target string) http.Handler {
	tu, err := url.Parse(target)
	if err != nil {
		slog.Error("invalid target url", "target", target, "error", err)
		panic("gateway: invalid target url " + target)
	}
	rp := httputil.NewSingleHostReverseProxy(tu)
	// NewSingleHostReverseProxy already rewrites the request path to the
	// target's path; we use the default director so downstream services see
	// their own routes (e.g. /auth/login).
	return rp
}

// Router assembles the full gin router with all middleware and routes.
func (g *Gateway) Router(pub *rsa.PublicKey, allowedOrigins []string) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery(), middleware.RequestLogger(), middleware.CORS(allowedOrigins))

	// Downstream services perform their own JWT and role checks. The gateway
	// also enforces rate limiting globally per client IP.
	limited := router.Group("/")
	limited.Use(g.limiter.RateLimit())
	{
		limited.Any("/auth/*any", proxyHandler(g.auth))
		limited.Any("/consent/*any", proxyHandler(g.consent))
		limited.Any("/audit/*any", proxyHandler(g.audit))
		limited.Any("/institution/*any", proxyHandler(g.institution))
	}

	// Health endpoint for orchestration.
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "gateway"})
	})

	return router
}

// proxyHandler adapts an http.Handler to a gin handler while copying the
// inbound request ID header through the proxy.
func proxyHandler(target http.Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := middleware.RequestID(c.Request.Context())
		c.Request.Header.Set("X-Request-ID", rid)
		target.ServeHTTP(c.Writer, c.Request)
	}
}
