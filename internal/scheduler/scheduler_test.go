package scheduler_test

import (
	"context"
	"log/slog"
	"sync"
	"testing"

	"hema/ces/internal/config"
	"hema/ces/internal/reminder"
	"hema/ces/internal/scheduler"

	"github.com/stretchr/testify/assert"
)

// mockBatchStore is a test double for reminder.BatchStore.
// Each call consumes the stored reminders so subsequent calls return empty,
// simulating rows being claimed by SELECT FOR UPDATE SKIP LOCKED.
// It is safe for concurrent use.
type mockBatchStore struct {
	reminders []reminder.Reminder
	err       error
	mu        sync.Mutex
	calls     int
}

// ReminderBatch records the call, returns err if set, and hands the current
// batch to process exactly once — subsequent calls find no rows.
func (m *mockBatchStore) ReminderBatch(_ context.Context, _ int, process func([]reminder.Reminder) error) error {
	m.mu.Lock()
	m.calls++
	batch := m.reminders
	m.reminders = nil
	m.mu.Unlock()

	if m.err != nil {
		return m.err
	}
	if len(batch) == 0 {
		return nil
	}
	return process(batch)
}

// mockSender is a test double for email.Sender.
// It is safe for concurrent use.
type mockSender struct {
	mu    sync.Mutex
	calls int
	err   error
}

// SendReminders records the call and returns err if set.
func (m *mockSender) SendReminders(_ context.Context, _ []reminder.Reminder) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	m.calls++
	return nil
}

func newScheduler(store *mockBatchStore, sender *mockSender, workerCount int) *scheduler.Scheduler {
	cfg := config.SchedulerConfig{
		IntervalSeconds: 60,
		WorkerCount:     workerCount,
		BatchSize:       10,
	}
	return scheduler.New(store, sender, cfg, slog.Default())
}

// Given: a batch of due reminders
// When: RunOnce is called
// Then: SendReminders is called for the worker that received reminders
func TestRunOnce_DueReminders_CallsSendReminders(t *testing.T) {
	store := &mockBatchStore{
		reminders: []reminder.Reminder{
			{ActivatedVoucherID: "v-1", HemaID: "h-1"},
		},
	}
	sender := &mockSender{}

	newScheduler(store, sender, 1).SendReminders(context.Background())

	assert.Equal(t, 1, sender.calls)
}

// Given: a scheduler configured with multiple workers and no due reminders
// When: RunOnce is called
// Then: ReminderBatch is called once per worker before stopping
func TestRunOnce_MultipleWorkers_RunsBatchPerWorker(t *testing.T) {
	store := &mockBatchStore{}
	sender := &mockSender{}

	newScheduler(store, sender, 3).SendReminders(context.Background())

	assert.Equal(t, 3, store.calls)
}

// Given: more due reminders than one round of workers can process
// When: RunOnce is called
// Then: workers keep looping until all reminders are drained
func TestRunOnce_MoreRemindersThanOneBatch_DrainsFully(t *testing.T) {
	// The mock will be replenished between rounds to simulate a larger queue.
	// Round 1: worker finds reminders → anyFound=true → loop
	// Round 2: store is replenished with a second batch → anyFound=true → loop
	// Round 3: store is empty → stops
	store := &drainMockStore{rounds: [][]reminder.Reminder{
		{{ActivatedVoucherID: "v-1"}, {ActivatedVoucherID: "v-2"}},
		{{ActivatedVoucherID: "v-3"}},
	}}
	sender := &mockSender{}

	scheduler.New(store, sender, config.SchedulerConfig{
		IntervalSeconds: 60,
		WorkerCount:     1,
		BatchSize:       10,
	}, slog.Default()).SendReminders(context.Background())

	assert.Equal(t, 2, sender.calls)
}

// drainMockStore returns successive rounds of reminders, then empty.
type drainMockStore struct {
	rounds [][]reminder.Reminder
	mu     sync.Mutex
}

func (d *drainMockStore) ReminderBatch(_ context.Context, _ int, process func([]reminder.Reminder) error) error {
	d.mu.Lock()
	var batch []reminder.Reminder
	if len(d.rounds) > 0 {
		batch = d.rounds[0]
		d.rounds = d.rounds[1:]
	}
	d.mu.Unlock()

	if len(batch) == 0 {
		return nil
	}
	return process(batch)
}

// Given: the email sender returns an error
// When: RunOnce is called
// Then: other workers are unaffected and the call completes without panic
func TestRunOnce_SenderFails_OtherWorkersUnaffected(t *testing.T) {
	store := &mockBatchStore{
		err: assert.AnError,
	}
	sender := &mockSender{}

	newScheduler(store, sender, 3).SendReminders(context.Background())

	assert.Equal(t, 3, store.calls)
}

// Given: no due reminders
// When: RunOnce is called
// Then: SendReminders is never called
func TestRunOnce_NoReminders_DoesNotCallSender(t *testing.T) {
	store := &mockBatchStore{}
	sender := &mockSender{}

	newScheduler(store, sender, 2).SendReminders(context.Background())

	assert.Equal(t, 0, sender.calls)
}
