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
	"github.com/yomi/yomi/internal/cache"
	"github.com/yomi/yomi/internal/config"
	"github.com/yomi/yomi/internal/consent"
	"github.com/yomi/yomi/internal/db"
	"github.com/yomi/yomi/internal/instclient"
	"github.com/yomi/yomi/internal/kafka"
	"github.com/yomi/yomi/internal/middleware"
)

func main() {
	_ = godotenv.Load()
	cfg := config.Load("consent")

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

	if err := db.Run(cfg.DBURL, consent.MigrationsFS, consent.MigrationsDir); err != nil {
		slog.Error("run migrations", "error", err)
		os.Exit(1)
	}

	rdb, err := cache.Connect(ctx, cfg.RedisAddr, cfg.RedisPass, cfg.RedisDB)
	if err != nil {
		slog.Error("connect redis", "error", err)
		os.Exit(1)
	}
	defer rdb.Close()

	if err := kafka.EnsureTopic(ctx, cfg.KafkaBrokers, kafka.TopicConsentEvents, 3, 1); err != nil {
		slog.Error("ensure kafka topic", "error", err)
		os.Exit(1)
	}
	producer := kafka.NewProducer(cfg)
	defer producer.Close()

	repo := consent.NewRepository(pool)
	svc := consent.NewService(repo, producer, rdb, cfg.ConsentTokenTTL)
	instClient := instclient.NewClient(cfg.ServiceInstitutionURL, cfg.InternalServiceSecret)
	handler := consent.NewHandler(svc, instClient)

	pub, err := middleware.LoadPublicKey(cfg.JWTPublicKeyPath)
	if err != nil {
		slog.Error("load public key", "error", err)
		os.Exit(1)
	}

	router := gin.New()
	router.Use(gin.Recovery(), middleware.RequestLogger())

	router.POST("/consent/request", handler.Request)

	router.GET("/consent/scopes", handler.ScopeCatalog)

	internal := router.Group("/consent")
	internal.Use(consent.RequireInternalSecret(cfg.InternalServiceSecret))
	{

		internal.POST("/verify-token", handler.VerifyToken)
	}

	api := router.Group("/consent")
	api.Use(middleware.JWTRequired(pub), middleware.RequireRole("citizen"))
	{
		api.GET("/pending/:citizenId", handler.Pending)
		api.POST("/approve/:requestId", handler.Approve)
		api.POST("/reject/:requestId", handler.Reject)
		api.DELETE("/revoke/:requestId", handler.Revoke)
		api.GET("/history/:citizenId", handler.History)

		api.POST("/check-token", handler.VerifyToken)
	}

	srv := &http.Server{
		Addr:              cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		slog.Info("consent service listening", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("consent server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("consent service shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown", "error", err)
	}
}
