// Command audit runs the Yomi Audit Ledger service. It consumes consent
// lifecycle events from Kafka and appends them to the hash-chained, append-only
// audit_log table, and serves the audit query API.
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
	"github.com/yomi/yomi/internal/audit"
	"github.com/yomi/yomi/internal/config"
	"github.com/yomi/yomi/internal/db"
	"github.com/yomi/yomi/internal/kafka"
	"github.com/yomi/yomi/internal/middleware"
	"github.com/yomi/yomi/internal/models"
)

func main() {
	_ = godotenv.Load()
	cfg := config.Load("audit")

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

	if err := db.Run(cfg.DBURL, audit.MigrationsFS, audit.MigrationsDir); err != nil {
		slog.Error("run migrations", "error", err)
		os.Exit(1)
	}

	repo := audit.NewRepository(pool)

	// Create the topic before the consumer joins so it gets a real partition
	// assignment even on a cold start where the topic does not exist yet.
	if err := kafka.EnsureTopic(ctx, cfg.KafkaBrokers, kafka.TopicConsentEvents, 3, 1); err != nil {
		slog.Error("ensure kafka topic", "error", err)
		os.Exit(1)
	}

	// Consume consent events and append them to the ledger. The consumer runs
	// in its own goroutine so the HTTP API and ledger ingestion share the
	// process.
	consumer := kafka.NewConsumer(kafka.ConsumerConfig{
		GroupID: "audit-ledger",
		Brokers: cfg.KafkaBrokers,
		Topic:   kafka.TopicConsentEvents,
	}, func(ctx context.Context, evt models.ConsentEvent) error {
		entry := audit.FromEvent(evt)
		if _, err := repo.Append(ctx, entry); err != nil {
			return err
		}
		slog.Info("audit entry appended", "sequence", "n/a", "event_type", evt.EventType, "event_id", evt.EventID)
		return nil
	})
	defer consumer.Close()

	consumerErr := make(chan error, 1)
	go func() {
		consumerErr <- consumer.Run(ctx)
	}()

	handler := audit.NewHandler(repo)
	pub, err := middleware.LoadPublicKey(cfg.JWTPublicKeyPath)
	if err != nil {
		slog.Error("load public key", "error", err)
		os.Exit(1)
	}

	router := gin.New()
	router.Use(gin.Recovery(), middleware.RequestLogger())

	api := router.Group("/audit")
	api.Use(middleware.JWTRequired(pub))
	{
		api.GET("/citizen/:citizenId", handler.ByCitizen)
		api.GET("/institution/:institutionId", handler.ByInstitution)
		api.GET("/event/:eventId", handler.ByID)
		api.GET("/verify", handler.Verify)
	}

	srv := &http.Server{
		Addr:              cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		slog.Info("audit service listening", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("audit server error", "error", err)
			os.Exit(1)
		}
	}()

	select {
	case <-ctx.Done():
	case err := <-consumerErr:
		if err != nil {
			slog.Error("audit consumer stopped with error", "error", err)
		}
	}

	slog.Info("audit service shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown", "error", err)
	}
}
