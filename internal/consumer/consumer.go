package consumer

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/IBM/sarama"
)

// Consumer implements sarama.ConsumerGroupHandler and dispatches incoming
// Kafka messages to the appropriate Handler method based on event name.
type Consumer struct {
	handler *Handler
}

// NewConsumer creates a Consumer that dispatches events to handler.
func NewConsumer(handler *Handler) *Consumer {
	return &Consumer{handler: handler}
}

// Setup implements sarama.ConsumerGroupHandler.
func (c *Consumer) Setup(_ sarama.ConsumerGroupSession) error { return nil }

// Cleanup implements sarama.ConsumerGroupHandler.
func (c *Consumer) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }

// ConsumeClaim processes messages from a single partition until the claim channel closes.
// Each message is dispatched to the appropriate handler based on its event name.
func (c *Consumer) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		if err := c.process(session.Context(), msg); err != nil {
			slog.Error("failed to process kafka message",
				"topic", msg.Topic,
				"partition", msg.Partition,
				"offset", msg.Offset,
				"error", err,
			)
		}
		session.MarkMessage(msg, "")
	}
	return nil
}

// process unmarshals the event envelope to determine the event name, then
// fully unmarshals and dispatches to the correct handler method.
func (c *Consumer) process(ctx context.Context, msg *sarama.ConsumerMessage) error {
	var envelope struct {
		Header struct {
			EventName string `json:"eventName"`
		} `json:"header"`
	}
	if err := json.Unmarshal(msg.Value, &envelope); err != nil {
		return err
	}

	switch envelope.Header.EventName {
	case "VoucherActivated":
		var event VoucherActivatedEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			return err
		}
		return c.handler.HandleActivated(ctx, event)
	case "VoucherRedeemed":
		var event VoucherRedeemedEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			return err
		}
		return c.handler.HandleRedeemed(ctx, event)
	default:
		slog.Debug("ignoring unknown event", "eventName", envelope.Header.EventName)
		return nil
	}
}
