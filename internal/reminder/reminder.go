// Package reminder defines the Reminder type and the interfaces for persisting
// and retrieving scheduled voucher reminder emails.
package reminder

import (
	"context"
	"time"
)

// Status represents the lifecycle state of a scheduled reminder.
type Status string

const (
	// StatusPending indicates the reminder is scheduled and not yet sent.
	StatusPending Status = "PENDING"
	// StatusSent indicates the reminder email has been successfully sent.
	StatusSent Status = "SENT"
	// StatusCancelled indicates the reminder was cancelled because the voucher was redeemed.
	StatusCancelled Status = "CANCELLED"
)

// Reminder represents a scheduled email reminder for an expiring voucher.
type Reminder struct {
	ID                 string    // ID of the reminder record
	ActivatedVoucherID string    // ID of the activated voucher
	HemaID             string    // Internal Hema identifier
	VoucherID          string    // ID of the voucher being reminded about
	ProgramID          string    // ID of the program the voucher belongs to
	Characteristic     string    // Characteristic of the voucher being reminded about
	SendAt             time.Time // Timestamp when the reminder is set to be sent
	Status             Status    // Status of the reminder (PENDING, SENT, CANCELLED)
	CreatedAt          time.Time // Timestamp when the reminder was created
	UpdatedAt          time.Time // Timestamp when the reminder was last updated
}

// Store is an interface implemented by a store that supports CRUD operations on reminders.
type Store interface {
	// Upsert inserts a new reminder or updates an existing PENDING one.
	// Reminders that are already SENT or CANCELLED are not overwritten.
	Upsert(ctx context.Context, reminder Reminder) error

	// Cancel marks a PENDING reminder as CANCELLED. Safe to call if the reminder
	// does not exist or is already SENT or CANCELLED.
	Cancel(ctx context.Context, activatedVoucherID string) error
}

// BatchStore is an interface implemented by a store that supports batch processing of reminders.
type BatchStore interface {
	// RunBatch selects up to limit PENDING reminders that need to be sent and let's
	// be processed by process.
	RunBatch(ctx context.Context, limit int, process func(Reminder) error) error
}
