// Package consumer handles consuming voucher lifecycle events from Kafka
// and translating them into reminder scheduling operations.
package consumer

import "time"

// EventHeader contains metadata present on all Kafka events.
type EventHeader struct {
	MessageID string `json:"messageId"`
	EventName string `json:"eventName"`
}

// VoucherActivatedEvent is emitted when a customer activates a voucher using loyalty points.
// It carries the voucher details needed to schedule a reminder email.
type VoucherActivatedEvent struct {
	Header  EventHeader             `json:"header"`
	Payload VoucherActivatedPayload `json:"payload"`
}

// VoucherActivatedPayload contains the customer and voucher details from a VoucherActivated event.
type VoucherActivatedPayload struct {
	HemaID             string         `json:"hemaId"`
	ActivatedVoucherID string         `json:"activatedVoucherId"`
	VoucherDetails     VoucherDetails `json:"voucherDetails"`
}

// VoucherDetails contains the voucher-specific fields from a VoucherActivated event.
type VoucherDetails struct {
	ProgramID      string    `json:"programId"`
	VoucherID      string    `json:"voucherId"`
	ValidUntil     time.Time `json:"validUntil"`
	Characteristic string    `json:"characteristic"`
}

// VoucherRedeemedEvent is emitted when a customer redeems (spends) a voucher.
// It is used to cancel any pending reminder for that voucher.
type VoucherRedeemedEvent struct {
	Header  EventHeader            `json:"header"`
	Payload VoucherRedeemedPayload `json:"payload"`
}

// VoucherRedeemedPayload contains the identifiers needed to cancel a scheduled reminder.
type VoucherRedeemedPayload struct {
	HemaID             string `json:"hemaId"`
	ActivatedVoucherID string `json:"activatedVoucherId"`
}
