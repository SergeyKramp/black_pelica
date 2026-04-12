package main

import (
	"context"
	"log/slog"
	"os/signal"
	"syscall"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	slog.Info("Customer Engagement Service starting")

	// TODO: initialise config, DB, Kafka consumer, scheduler

	<-ctx.Done()
	slog.Info("shutting down")
}
