package consumer_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"hema/ces/internal/config"
	"hema/ces/internal/consumer"

	"github.com/IBM/sarama"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSession implements sarama.ConsumerGroupSession for testing.
type mockSession struct {
	ctx    context.Context
	marked []*sarama.ConsumerMessage
}

func (m *mockSession) Claims() map[string][]int32                              { return nil }
func (m *mockSession) MemberID() string                                        { return "" }
func (m *mockSession) GenerationID() int32                                     { return 0 }
func (m *mockSession) MarkOffset(_ string, _ int32, _ int64, _ string)        {}
func (m *mockSession) Commit()                                                 {}
func (m *mockSession) ResetOffset(_ string, _ int32, _ int64, _ string)       {}
func (m *mockSession) Context() context.Context                                { return m.ctx }
func (m *mockSession) MarkMessage(msg *sarama.ConsumerMessage, _ string) {
	m.marked = append(m.marked, msg)
}

// mockClaim implements sarama.ConsumerGroupClaim for testing.
// Send messages via the messages channel and close it to end ConsumeClaim.
type mockClaim struct {
	messages chan *sarama.ConsumerMessage
}

func (m *mockClaim) Topic() string                                    { return "test-topic" }
func (m *mockClaim) Partition() int32                                 { return 0 }
func (m *mockClaim) InitialOffset() int64                             { return 0 }
func (m *mockClaim) HighWaterMarkOffset() int64                       { return 0 }
func (m *mockClaim) Messages() <-chan *sarama.ConsumerMessage          { return m.messages }

// kafkaMsg builds a sarama.ConsumerMessage from any JSON-serialisable payload.
func kafkaMsg(t *testing.T, payload any) *sarama.ConsumerMessage {
	t.Helper()
	value, err := json.Marshal(payload)
	require.NoError(t, err)
	return &sarama.ConsumerMessage{Value: value}
}

// Given: a VoucherActivated Kafka message for a configured voucher type
// When: the consumer processes the message
// Then: a reminder is upserted and the message is marked
func TestConsumeClaim_VoucherActivated_UpsertsReminder(t *testing.T) {
	store := &mockStore{}
	cfg := &config.Config{ReminderOffsets: map[string]int{"HEMA": 3}}
	c := consumer.NewConsumer(consumer.NewHandler(store, cfg))

	session := &mockSession{ctx: context.Background()}
	claim := &mockClaim{messages: make(chan *sarama.ConsumerMessage, 1)}

	claim.messages <- kafkaMsg(t, map[string]any{
		"header":  map[string]any{"eventName": "VoucherActivated"},
		"payload": map[string]any{
			"hemaId":             "hema-1",
			"activatedVoucherId": "voucher-1",
			"voucherDetails": map[string]any{
				"programId":      "nl",
				"voucherId":      "278",
				"validUntil":     time.Now().Add(72 * time.Hour),
				"characteristic": "HEMA",
			},
		},
	})
	close(claim.messages)

	err := c.ConsumeClaim(session, claim)
	require.NoError(t, err)

	assert.Len(t, store.upserted, 1)
	assert.Equal(t, "voucher-1", store.upserted[0].ActivatedVoucherID)
	assert.Len(t, session.marked, 1)
}

// Given: a VoucherRedeemed Kafka message
// When: the consumer processes the message
// Then: the reminder is cancelled and the message is marked
func TestConsumeClaim_VoucherRedeemed_CancelsReminder(t *testing.T) {
	store := &mockStore{}
	cfg := &config.Config{}
	c := consumer.NewConsumer(consumer.NewHandler(store, cfg))

	session := &mockSession{ctx: context.Background()}
	claim := &mockClaim{messages: make(chan *sarama.ConsumerMessage, 1)}

	claim.messages <- kafkaMsg(t, map[string]any{
		"header":  map[string]any{"eventName": "VoucherRedeemed"},
		"payload": map[string]any{
			"hemaId":             "hema-1",
			"activatedVoucherId": "voucher-1",
		},
	})
	close(claim.messages)

	err := c.ConsumeClaim(session, claim)
	require.NoError(t, err)

	assert.Equal(t, []string{"voucher-1"}, store.cancelled)
	assert.Len(t, session.marked, 1)
}

// Given: a Kafka message with an unknown event name
// When: the consumer processes the message
// Then: the message is silently ignored and still marked (consumer does not stall)
func TestConsumeClaim_UnknownEvent_MarksMessageWithoutError(t *testing.T) {
	store := &mockStore{}
	c := consumer.NewConsumer(consumer.NewHandler(store, &config.Config{}))

	session := &mockSession{ctx: context.Background()}
	claim := &mockClaim{messages: make(chan *sarama.ConsumerMessage, 1)}

	claim.messages <- kafkaMsg(t, map[string]any{
		"header": map[string]any{"eventName": "SomeUnknownEvent"},
	})
	close(claim.messages)

	err := c.ConsumeClaim(session, claim)
	require.NoError(t, err)

	assert.Empty(t, store.upserted)
	assert.Empty(t, store.cancelled)
	assert.Len(t, session.marked, 1)
}

// Given: a Kafka message with malformed JSON
// When: the consumer processes the message
// Then: the message is still marked so the consumer does not stall
func TestConsumeClaim_MalformedJSON_StillMarksMessage(t *testing.T) {
	store := &mockStore{}
	c := consumer.NewConsumer(consumer.NewHandler(store, &config.Config{}))

	session := &mockSession{ctx: context.Background()}
	claim := &mockClaim{messages: make(chan *sarama.ConsumerMessage, 1)}

	claim.messages <- &sarama.ConsumerMessage{Value: []byte("not valid json {")}
	close(claim.messages)

	err := c.ConsumeClaim(session, claim)
	require.NoError(t, err)

	assert.Empty(t, store.upserted)
	assert.Len(t, session.marked, 1)
}
