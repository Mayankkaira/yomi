// Command auth runs the Yomi Auth service. It reads config from the
// environment, runs embedded migrations, and serves the JWT/refresh API.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/yomi/yomi/internal/auth"
	"github.com/yomi/yomi/internal/cache"
	"github.com/yomi/yomi/internal/config"
	"github.com/yomi/yomi/internal/db"
	"github.com/yomi/yomi/internal/middleware"
)

func main() {
	_ = godotenv.Load()
	cfg := config.Load("auth")

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, cfg.DBURL)
	if err != nil {
		slog.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := db.Run(cfg.DBURL, auth.MigrationsFS, auth.MigrationsDir); err != nil {
		slog.Error("run migrations", "error", err)
		os.Exit(1)
	}

	rdb, err := cache.Connect(ctx, cfg.RedisAddr, cfg.RedisPass, cfg.RedisDB)
	if err != nil {
		slog.Error("connect redis", "error", err)
		os.Exit(1)
	}
	defer rdb.Close()

	priv, err := middleware.LoadPrivateKey(cfg.JWTPrivateKeyPath)
	if err != nil {
		slog.Error("load private key", "error", err)
		os.Exit(1)
	}
	pub, err := middleware.LoadPublicKey(cfg.JWTPublicKeyPath)
	if err != nil {
		slog.Error("load public key", "error", err)
		os.Exit(1)
	}

	repo := auth.NewRepository(pool, rdb)
	tokens := auth.NewTokenService(priv, pub, rdb, cfg.AccessTokenTTL, cfg.RefreshTokenTTL)
	handler := auth.NewHandler(repo, tokens)

	router := gin.New()
	router.Use(gin.Recovery(), middleware.RequestLogger())

	api := router.Group("/auth")
	{
		api.POST("/register", handler.Register)
		api.POST("/login", handler.Login)
		api.POST("/refresh", handler.Refresh)
		api.POST("/logout", handler.Logout)
	}

	srv := &http.Server{
		Addr:              cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		slog.Info("auth service listening", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("auth server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("auth service shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown", "error", err)
	}
}
