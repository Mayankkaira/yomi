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
	"github.com/yomi/yomi/internal/config"
	"github.com/yomi/yomi/internal/db"
	"github.com/yomi/yomi/internal/institution"
	"github.com/yomi/yomi/internal/middleware"
)

func main() {
	_ = godotenv.Load()
	cfg := config.Load("institution")

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

	if err := db.Run(cfg.DBURL, institution.MigrationsFS, institution.MigrationsDir); err != nil {
		slog.Error("run migrations", "error", err)
		os.Exit(1)
	}

	repo := institution.NewRepository(pool)
	handler := institution.NewHandler(repo)

	pub, err := middleware.LoadPublicKey(cfg.JWTPublicKeyPath)
	if err != nil {
		slog.Error("load public key", "error", err)
		os.Exit(1)
	}

	router := gin.New()
	router.Use(gin.Recovery(), middleware.RequestLogger())

	api := router.Group("/institution")
	{

		api.POST("/register", handler.Register)

		api.POST("/authenticate", institution.RequireInternalSecret(cfg.InternalServiceSecret), handler.Authenticate)

		api.GET("/:id", middleware.JWTRequired(pub), handler.Get)
		api.POST("/:id/rotate-key", middleware.JWTRequired(pub), handler.RotateKey)
		api.PUT("/:id/role", middleware.JWTRequired(pub), middleware.RequireRole("admin"), handler.SetRole)
	}

	srv := &http.Server{
		Addr:              cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		slog.Info("institution service listening", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("institution server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("institution service shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown", "error", err)
	}
}
