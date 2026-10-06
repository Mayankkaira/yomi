// Package kafka provides typed producer and consumer wrappers around
// kafka-go for the consent-events topic.
package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/yomi/yomi/internal/config"
	"github.com/yomi/yomi/internal/models"
)

const (
	TopicConsentEvents = "consent-events"
	TopicAuditEvents   = "audit-events"
)

// EnsureTopic creates the topic if it does not exist. Consumers must call
// this before starting their reader: joining a group for a topic that does not
// exist yet yields a zero-partition assignment that kafka-go never repairs
// when the topic is later auto-created.
func EnsureTopic(ctx context.Context, brokers []string, topic string, partitions, replication int) error {
	conn, err := kafka.DialContext(ctx, "tcp", brokers[0])
	if err != nil {
		return fmt.Errorf("dial kafka: %w", err)
	}
	defer conn.Close()

	controller, err := conn.Controller()
	if err != nil {
		return fmt.Errorf("kafka controller lookup: %w", err)
	}
	controllerConn, err := kafka.DialContext(ctx, "tcp", net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port)))
	if err != nil {
		return fmt.Errorf("dial kafka controller: %w", err)
	}
	defer controllerConn.Close()

	err = controllerConn.CreateTopics(kafka.TopicConfig{
		Topic:             topic,
		NumPartitions:     partitions,
		ReplicationFactor: replication,
	})
	if err != nil {
		// TopicAlreadyExists is reported for a topic created concurrently;
		// treat it as success.
		if errors.Is(err, kafka.TopicAlreadyExists) {
			return nil
		}
		return fmt.Errorf("create topic %s: %w", topic, err)
	}
	slog.Info("kafka topic ensured", "topic", topic, "partitions", partitions)
	return nil
}

// Producer writes consent lifecycle events to the consent-events topic.
type Producer struct {
	writer *kafka.Writer
}

// NewProducer creates a Kafka writer for the consent-events topic.
func NewProducer(cfg *config.Config) *Producer {
	w := &kafka.Writer{
		Addr:         kafka.TCP(cfg.KafkaBrokers...),
		Topic:        TopicConsentEvents,
		Balancer:     &kafka.Hash{},
		RequiredAcks: kafka.RequireAll,
		MaxAttempts:  5,
		BatchTimeout: 10 * time.Millisecond,
	}
	return &Producer{writer: w}
}

// Close flushes and closes the underlying writer.
func (p *Producer) Close() error {
	return p.writer.Close()
}

// Publish serializes and writes a consent event. It blocks until the message
// is acknowledged or the context is cancelled.
func (p *Producer) Publish(ctx context.Context, event models.ConsentEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal consent event: %w", err)
	}
	msg := kafka.Message{Key: []byte(event.RequestID), Value: payload}
	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("write consent event: %w", err)
	}
	slog.Info("consent event published", "event_type", event.EventType, "request_id", event.RequestID)
	return nil
}

// Consumer reads consent events from the consent-events topic and dispatches
// each to the handler. It runs until ctx is cancelled.
type Consumer struct {
	reader  *kafka.Reader
	handler func(context.Context, models.ConsentEvent) error
}

// ConsumerConfig describes how to construct the underlying kafka reader.
type ConsumerConfig struct {
	GroupID string
	Brokers []string
	Topic   string
}

// NewConsumer creates a consumer with the given group, topic and handler.
func NewConsumer(cfg ConsumerConfig, handler func(context.Context, models.ConsentEvent) error) *Consumer {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  cfg.Brokers,
		GroupID:  cfg.GroupID,
		Topic:    cfg.Topic,
		MinBytes: 1,
		MaxBytes: 10e6,
		// Start from the beginning so events produced before this consumer
		// first joined are still captured. The ledger append is idempotent on
		// event ID, so replays are safe.
		StartOffset:    kafka.FirstOffset,
		CommitInterval: time.Second,
	})
	return &Consumer{reader: r, handler: handler}
}

// Close shuts down the reader.
func (c *Consumer) Close() error {
	return c.reader.Close()
}

// Run blocks consuming messages until ctx is cancelled. A failed message is
// logged but does not abort the loop; the offset is committed only after a
// successful handler call, so unprocessed messages are redelivered.
func (c *Consumer) Run(ctx context.Context) error {
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			slog.Error("fetch kafka message", "error", err)
			continue
		}
		var event models.ConsentEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			slog.Error("decode consent event", "error", err)
			// Unprocessable message: commit to avoid a poison-pill loop.
			if err := c.reader.CommitMessages(ctx, msg); err != nil {
				slog.Error("commit decode-failed message", "error", err)
			}
			continue
		}
		if err := c.handler(ctx, event); err != nil {
			slog.Error("handle consent event", "error", err, "event_id", event.EventID)
			continue
		}
		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			slog.Error("commit message", "error", err)
		}
	}
}
