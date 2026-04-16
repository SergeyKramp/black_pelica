package consumer

import (
	"context"

	"hema/ces/internal/reminder"
)

// Handler processes voucher lifecycle events and translates them into
// reminder scheduling operations against the reminder store.
type Handler struct {
	store reminder.Store
}

// NewHandler creates a Handler that writes to store.
func NewHandler(store reminder.Store) *Handler {
	return &Handler{store: store}
}

// HandleActivated schedules a reminder for the voucher in the event.
func (h *Handler) HandleActivated(ctx context.Context, event VoucherActivatedEvent) error {
	p := event.Payload
	r := reminder.Reminder{
		ActivatedVoucherID: p.ActivatedVoucherID,
		HemaID:             p.HemaID,
		VoucherID:          p.VoucherDetails.VoucherID,
		ProgramID:          p.VoucherDetails.ProgramID,
		Characteristic:     p.VoucherDetails.Characteristic,
		ValidUntil:         p.VoucherDetails.ValidUntil,
	}
	return h.store.Upsert(ctx, r)
}

// HandleRedeemed cancels the pending reminder for the voucher in the event.
func (h *Handler) HandleRedeemed(ctx context.Context, event VoucherRedeemedEvent) error {
	return h.store.Cancel(ctx, event.Payload.ActivatedVoucherID)
}
