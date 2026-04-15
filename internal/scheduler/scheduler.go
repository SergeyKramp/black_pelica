// Package scheduler polls the database for due reminders and dispatches them
// via the email sender. Multiple concurrent workers claim disjoint sets of rows
// using SELECT FOR UPDATE SKIP LOCKED so the scheduler can safely run as several
// replicas without sending duplicate emails.
package scheduler

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"hema/ces/internal/config"
	"hema/ces/internal/email"
	"hema/ces/internal/reminder"
)

// Scheduler polls for due reminders and sends them via the email sender.
type Scheduler struct {
	store  reminder.BatchStore
	sender email.Sender
	cfg    config.SchedulerConfig
	logger *slog.Logger
}

// New creates a Scheduler that fetches reminders from store and sends them
// via sender according to the timing and concurrency settings in cfg.
func New(store reminder.BatchStore, sender email.Sender, cfg config.SchedulerConfig, logger *slog.Logger) *Scheduler {
	return &Scheduler{store: store, sender: sender, cfg: cfg, logger: logger}
}

// SendReminders drains all due reminders by repeatedly running cfg.WorkerCount
// workers until a full round finds nothing left to process. Each worker claims
// up to cfg.BatchSize rows via SELECT FOR UPDATE SKIP LOCKED so workers always
// operate on disjoint sets. Errors are logged and do not stop other workers.
func (s *Scheduler) SendReminders(ctx context.Context) {
	for {
		var (
			wg                     sync.WaitGroup
			mu                     sync.Mutex
			moreRemindersToProcess bool
		)
		for range s.cfg.WorkerCount {
			wg.Go(func() {
				found := false
				err := s.store.ReminderBatch(ctx, s.cfg.BatchSize, func(reminders []reminder.Reminder) error {
					found = true
					return s.sender.SendReminders(ctx, reminders)
				})
				if err != nil {
					s.logger.ErrorContext(ctx, "reminder batch failed", "error", err)
				}
				if found {
					mu.Lock()
					moreRemindersToProcess = true
					mu.Unlock()
				}
			})
		}
		wg.Wait()
		if !moreRemindersToProcess {
			// Stop once all reminders have been processed
			return
		}
	}
}

// Start runs the scheduler on a fixed interval until ctx is cancelled.
func (s *Scheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(s.cfg.IntervalSeconds) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.SendReminders(ctx)
		}
	}
}
