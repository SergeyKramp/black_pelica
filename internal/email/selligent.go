package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"hema/ces/internal/reminder"
)

// SelligentSender sends voucher reminder emails via the Selligent HTTP API.
// This doens't follow the actual Selligent API, just a simplified version for the purpose of this assignment.
type SelligentSender struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewSelligentSender creates a SelligentSender that posts to baseURL
// and authenticates using apiKey.
func NewSelligentSender(baseURL, apiKey string) *SelligentSender {
	return &SelligentSender{
		baseURL:    baseURL,
		apiKey:     apiKey,
		httpClient: &http.Client{},
	}
}

// reminderMessage is a single entry in the bulk send request body.
type reminderMessage struct {
	HemaID         string `json:"hemaId"`
	VoucherID      string `json:"voucherId"`
	Characteristic string `json:"characteristic"`
}

// bulkSendRequest is the JSON body for the Selligent bulk send endpoint.
type bulkSendRequest struct {
	Messages []reminderMessage `json:"messages"`
}

// SendReminders sends a batch of reminder emails in a single request to the
// Selligent bulk send endpoint. It returns an error if the request fails or
// Selligent responds with a non-2xx status.
func (s *SelligentSender) SendReminders(ctx context.Context, reminders []reminder.Reminder) error {
	messages := make([]reminderMessage, len(reminders))
	for i, r := range reminders {
		messages[i] = reminderMessage{
			HemaID:         r.HemaID,
			VoucherID:      r.VoucherID,
			Characteristic: r.Characteristic,
		}
	}

	body, err := json.Marshal(bulkSendRequest{Messages: messages})
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/email/v1/messages/send", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.apiKey)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send reminder emails: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("selligent returned %d for batch of %d reminders", resp.StatusCode, len(reminders))
	}
	return nil
}
