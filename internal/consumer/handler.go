package consumer

import (
	"context"
	"time"

	"hema/ces/internal/config"
	"hema/ces/internal/reminder"
)

// Handler processes voucher lifecycle events and translates them into
// reminder scheduling operations against the reminder store.
type Handler struct {
	store reminder.Store
	cfg   *config.Config
}

// NewHandler creates a Handler that writes to store and uses cfg to look up
// per-voucher-type reminder offsets.
func NewHandler(store reminder.Store, cfg *config.Config) *Handler {
	return &Handler{store: store, cfg: cfg}
}

// HandleActivated schedules a reminder for the voucher in the event.
// If no reminder offset is configured for the voucher's characteristic, it is a no-op.
func (h *Handler) HandleActivated(ctx context.Context, event VoucherActivatedEvent) error {
	p := event.Payload
	days, ok := h.cfg.ReminderOffset(p.VoucherDetails.Characteristic)
	if !ok {
		return nil
	}

	r := reminder.Reminder{
		ActivatedVoucherID: p.ActivatedVoucherID,
		HemaID:             p.HemaID,
		VoucherID:          p.VoucherDetails.VoucherID,
		ProgramID:          p.VoucherDetails.ProgramID,
		Characteristic:     p.VoucherDetails.Characteristic,
		SendAt:             p.VoucherDetails.ValidUntil.Add(-time.Duration(days) * 24 * time.Hour),
	}

	return h.store.Upsert(ctx, r)
}

// HandleRedeemed cancels the pending reminder for the voucher in the event.
func (h *Handler) HandleRedeemed(ctx context.Context, event VoucherRedeemedEvent) error {
	return h.store.Cancel(ctx, event.Payload.ActivatedVoucherID)
}
