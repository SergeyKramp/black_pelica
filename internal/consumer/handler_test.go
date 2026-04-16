package consumer_test

import (
	"context"
	"testing"
	"time"

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

// Given: a VoucherActivated event
// When: handling the event
// Then: a reminder is upserted with the correct fields and validUntil timestamp
func TestHandleActivated_UpsertWithCorrectFields(t *testing.T) {
	store := &mockStore{}
	handler := consumer.NewHandler(store)

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
	assert.Equal(t, validUntil, r.ValidUntil)
}

// Given: a VoucherRedeemed event
// When: handling the event
// Then: the corresponding reminder is cancelled
func TestHandleRedeemed_CancelsReminder(t *testing.T) {
	store := &mockStore{}
	handler := consumer.NewHandler(store)

	event := consumer.VoucherRedeemedEvent{
		Payload: consumer.VoucherRedeemedPayload{
			ActivatedVoucherID: "voucher-456",
		},
	}

	err := handler.HandleRedeemed(context.Background(), event)
	require.NoError(t, err)
	assert.Equal(t, []string{"voucher-456"}, store.cancelled)
}
