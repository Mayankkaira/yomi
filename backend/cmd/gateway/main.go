// Command gateway runs the Yomi API Gateway, the single entry point for all
// client and institution traffic. It applies rate limiting, CORS, and
// structured logging, then reverse-proxies to the downstream services.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/yomi/yomi/internal/config"
	"github.com/yomi/yomi/internal/gateway"
	"github.com/yomi/yomi/internal/middleware"
)

func main() {
	_ = godotenv.Load()
	cfg := config.Load("gateway")

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pub, err := middleware.LoadPublicKey(cfg.JWTPublicKeyPath)
	if err != nil {
		slog.Error("load public key", "error", err)
		os.Exit(1)
	}

	origins := strings.Split(os.Getenv("CORS_ORIGINS"), ",")
	if len(origins) == 1 && origins[0] == "" {
		origins = []string{"http://localhost:5173", "http://localhost:8080"}
	}

	gw := gateway.New(
		cfg.ServiceAuthURL,
		cfg.ServiceConsentURL,
		cfg.ServiceAuditURL,
		cfg.ServiceInstitutionURL,
		cfg.RateLimitRPS,
		cfg.RateLimitBurst,
	)
	router := gw.Router(pub, origins)

	srv := &http.Server{
		Addr:              cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		slog.Info("gateway listening", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("gateway server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("gateway shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown", "error", err)
	}
}
