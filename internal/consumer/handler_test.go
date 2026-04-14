package consumer_test

import (
	"context"
	"testing"
	"time"

	"hema/ces/internal/config"
	"hema/ces/internal/consumer"
	"hema/ces/internal/reminder"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockStore struct {
	upserted  []reminder.Reminder
	cancelled []string
}

func (m *mockStore) Upsert(_ context.Context, r reminder.Reminder) error {
	m.upserted = append(m.upserted, r)
	return nil
}

func (m *mockStore) Cancel(_ context.Context, id string) error {
	m.cancelled = append(m.cancelled, id)
	return nil
}

// Given: a VoucherActivated event for a configured voucher type
// When: handling the event
// Then: a reminder is scheduled with send_at = validUntil - offset
func TestHandleActivated_ConfiguredType_SchedulesReminder(t *testing.T) {
	store := &mockStore{}
	cfg := &config.Config{ReminderOffsets: map[string]int{"HEMA": 3}}
	handler := consumer.NewHandler(store, cfg)

	validUntil := time.Date(2025, 6, 29, 10, 27, 23, 0, time.UTC)
	event := consumer.VoucherActivatedEvent{
		Payload: consumer.VoucherActivatedPayload{
			HemaID:             "hema-123",
			ActivatedVoucherID: "voucher-456",
			VoucherDetails: consumer.VoucherDetails{
				ProgramID:      "nl",
				VoucherID:      "278",
				ValidUntil:     validUntil,
				Characteristic: "HEMA",
			},
		},
	}

	err := handler.HandleActivated(context.Background(), event)
	require.NoError(t, err)

	require.Len(t, store.upserted, 1)
	r := store.upserted[0]
	assert.Equal(t, "voucher-456", r.ActivatedVoucherID)
	assert.Equal(t, "hema-123", r.HemaID)
	assert.Equal(t, "HEMA", r.Characteristic)
	assert.Equal(t, validUntil.Add(-3*24*time.Hour), r.SendAt)
}

// Given: a VoucherActivated event for a voucher type with a zero offset
// When: handling the event
// Then: no reminder is scheduled
func TestHandleActivated_ZeroOffset_DoesNotSchedule(t *testing.T) {
	store := &mockStore{}
	cfg := &config.Config{ReminderOffsets: map[string]int{"GIFT": 0}}
	handler := consumer.NewHandler(store, cfg)

	event := consumer.VoucherActivatedEvent{
		Payload: consumer.VoucherActivatedPayload{
			ActivatedVoucherID: "voucher-789",
			VoucherDetails:     consumer.VoucherDetails{Characteristic: "GIFT"},
		},
	}

	err := handler.HandleActivated(context.Background(), event)
	require.NoError(t, err)
	assert.Empty(t, store.upserted)
}

// Given: a VoucherActivated event for an unconfigured voucher type
// When: handling the event
// Then: no reminder is scheduled
func TestHandleActivated_UnknownType_DoesNotSchedule(t *testing.T) {
	store := &mockStore{}
	cfg := &config.Config{ReminderOffsets: map[string]int{"HEMA": 3}}
	handler := consumer.NewHandler(store, cfg)

	event := consumer.VoucherActivatedEvent{
		Payload: consumer.VoucherActivatedPayload{
			ActivatedVoucherID: "voucher-789",
			VoucherDetails:     consumer.VoucherDetails{Characteristic: "UNKNOWN"},
		},
	}

	err := handler.HandleActivated(context.Background(), event)
	require.NoError(t, err)
	assert.Empty(t, store.upserted)
}

// Given: a VoucherRedeemed event
// When: handling the event
// Then: the corresponding reminder is cancelled
func TestHandleRedeemed_CancelsReminder(t *testing.T) {
	store := &mockStore{}
	handler := consumer.NewHandler(store, &config.Config{})

	event := consumer.VoucherRedeemedEvent{
		Payload: consumer.VoucherRedeemedPayload{
			ActivatedVoucherID: "voucher-456",
		},
	}

	err := handler.HandleRedeemed(context.Background(), event)
	require.NoError(t, err)
	assert.Equal(t, []string{"voucher-456"}, store.cancelled)
}
