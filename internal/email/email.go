// Package email provides the interface and implementation for sending
// voucher reminder emails via the Selligent email marketing platform.
package email

import (
	"context"

	"hema/ces/internal/reminder"
)

// Sender sends voucher reminder emails. Implementations must be safe for concurrent use.
type Sender interface {
	// SendReminders sends reminder emails for a batch of reminders
	SendReminders(ctx context.Context, reminders []reminder.Reminder) error
}
