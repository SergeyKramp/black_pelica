// Package main is the entry point for the Customer Engagement Service.
// It wires together the Kafka consumer, PostgreSQL store, email sender, and
// reminder scheduler, then runs them concurrently until a shutdown signal.
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"hema/ces/internal/config"
	"hema/ces/internal/consumer"
	"hema/ces/internal/db"
	"hema/ces/internal/email"
	"hema/ces/internal/reminder"
	"hema/ces/internal/scheduler"

	"github.com/IBM/sarama"
)

func main() {
	cfgPath := flag.String("config", "config.yaml", "path to the YAML configuration file")
	debug := flag.Bool("debug", false, "enable debug logging")
	flag.Parse()

	if *debug {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})))
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Load configuration.
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		slog.Error("load config", "path", *cfgPath, "error", err)
		os.Exit(1)
	}

	// Connect to PostgreSQL.
	pool, err := db.Connect(ctx, cfg.Database)
	if err != nil {
		slog.Error("connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Shared store used by both the Kafka consumer and the scheduler.
	store := reminder.NewPostgresStore(pool)

	// Email sender.
	sender := email.NewSelligentSender(cfg.Selligent.BaseURL, cfg.Selligent.APIKey)

	// Kafka consumer group.
	saramaCfg := sarama.NewConfig()
	saramaCfg.Version = sarama.V2_6_0_0

	group, err := sarama.NewConsumerGroup(cfg.Kafka.Brokers, cfg.Kafka.Vouchers.ConsumerGroup, saramaCfg)
	if err != nil {
		slog.Error("create kafka consumer group", "error", err)
		os.Exit(1)
	}
	defer group.Close() //nolint:errcheck

	handler := consumer.NewHandler(store, cfg)
	cons := consumer.NewConsumer(handler)

	// Scheduler.
	sched := scheduler.New(store, sender, cfg.Scheduler, slog.Default())

	slog.Info("Customer Engagement Service starting",
		"config", *cfgPath,
		"kafka_brokers", cfg.Kafka.Brokers,
		"scheduler_interval_seconds", cfg.Scheduler.IntervalSeconds,
		"scheduler_workers", cfg.Scheduler.WorkerCount,
	)

	var wg sync.WaitGroup

	// Run the scheduler in the background.
	wg.Go(func() {
		slog.Info("scheduler started")
		sched.Start(ctx)
		slog.Info("scheduler stopped")
	})

	// Run the Kafka consumer group in the background.
	// Consume must be called in a loop because it returns after each rebalance.
	wg.Go(func() {
		slog.Info("kafka consumer started", "topic", cfg.Kafka.Vouchers.Topic)
		topics := []string{cfg.Kafka.Vouchers.Topic}
		for {
			if err := group.Consume(ctx, topics, cons); err != nil {
				slog.Error("kafka consumer error", "error", err)
			}
			if ctx.Err() != nil {
				return
			}
		}
	})

	<-ctx.Done()
	slog.Info("shutdown signal received, waiting for workers to finish")
	wg.Wait()
	slog.Info("shutdown complete")
}
