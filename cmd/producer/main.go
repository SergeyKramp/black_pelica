// Package main is a development tool that produces mock VoucherActivated and
// VoucherRedeemed events to Kafka on a fixed interval. It is not part of the
// production service — use it to drive the CES server locally without needing
// a real upstream system.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/IBM/sarama"
)

const topic = "Hema.Loyalty.Voucher"

var characteristics = []string{"HEMA", "PREMIUM", "GIFT"}

func main() {
	broker := flag.String("broker", "localhost:9092", "Kafka broker address")
	interval := flag.Duration("interval", 5*time.Second, "interval between produced events")
	flag.Parse()

	cfg := sarama.NewConfig()
	cfg.Version = sarama.V2_6_0_0
	cfg.Producer.Return.Successes = true

	producer, err := sarama.NewSyncProducer([]string{*broker}, cfg)
	if err != nil {
		slog.Error("create producer", "error", err)
		os.Exit(1)
	}
	defer producer.Close()

	slog.Info("mock producer started", "broker", *broker, "topic", topic, "interval", *interval)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	counter := 0
	// Keep track of a few recent activated voucher IDs so we can redeem them.
	var redeemable []string

	for {
		select {
		case <-stop:
			slog.Info("producer stopped")
			return
		case <-ticker.C:
			counter++
			voucherID := fmt.Sprintf("voucher-%d", counter)
			characteristic := characteristics[rand.IntN(len(characteristics))]

			// Every third event, redeem a previously activated voucher.
			if counter%3 == 0 && len(redeemable) > 0 {
				id := redeemable[0]
				redeemable = redeemable[1:]
				if err := produce(producer, redeemedEvent(id)); err != nil {
					slog.Error("produce redeemed event", "error", err)
				} else {
					slog.Info("produced VoucherRedeemed", "activatedVoucherId", id)
				}
			}

			if err := produce(producer, activatedEvent(voucherID, characteristic)); err != nil {
				slog.Error("produce activated event", "error", err)
			} else {
				slog.Info("produced VoucherActivated",
					"activatedVoucherId", voucherID,
					"characteristic", characteristic,
				)
				redeemable = append(redeemable, voucherID)
			}
		}
	}
}

// activatedEvent builds a VoucherActivated message with a validity window that
// expires 1 minute from now, so any non-zero reminder offset will produce a
// send_at in the past and the scheduler will pick it up immediately.
func activatedEvent(activatedVoucherID, characteristic string) any {
	return map[string]any{
		"header": map[string]any{
			"messageId": activatedVoucherID + "-activated",
			"eventName": "VoucherActivated",
		},
		"payload": map[string]any{
			"hemaId":             "hema-" + activatedVoucherID,
			"activatedVoucherId": activatedVoucherID,
			"voucherDetails": map[string]any{
				"programId":      "nl",
				"voucherId":      "278",
				"validUntil":     time.Now().Add(time.Minute),
				"characteristic": characteristic,
			},
		},
	}
}

// redeemedEvent builds a VoucherRedeemed message for the given activated voucher.
func redeemedEvent(activatedVoucherID string) any {
	return map[string]any{
		"header": map[string]any{
			"messageId": activatedVoucherID + "-redeemed",
			"eventName": "VoucherRedeemed",
		},
		"payload": map[string]any{
			"hemaId":             "hema-" + activatedVoucherID,
			"activatedVoucherId": activatedVoucherID,
		},
	}
}

// produce serialises msg as JSON and sends it to the topic.
func produce(p sarama.SyncProducer, msg any) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, _, err = p.SendMessage(&sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.ByteEncoder(body),
	})
	return err
}
