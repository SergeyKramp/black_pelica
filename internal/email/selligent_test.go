package email_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"hema/ces/internal/email"
	"hema/ces/internal/reminder"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func reminders(n int) []reminder.Reminder {
	result := make([]reminder.Reminder, n)
	for i := range result {
		result[i] = reminder.Reminder{
			ActivatedVoucherID: fmt.Sprintf("voucher-%d", i+1),
			HemaID:             fmt.Sprintf("hema-%d", i+1),
			VoucherID:          "278",
			Characteristic:     "HEMA",
			SendAt:             time.Now(),
		}
	}
	return result
}

// Given: a batch of due reminders and a reachable Selligent endpoint
// When: sending the reminders
// Then: a POST is made to the bulk endpoint with all reminders and their idempotency keys
func TestSendReminders_Success_SendsCorrectBulkPayload(t *testing.T) {
	var received map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &received)
		assert.Equal(t, "/email/v1/messages/send", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sender := email.NewSelligentSender(srv.URL, "test-api-key")
	batch := reminders(3)

	err := sender.SendReminders(context.Background(), batch)
	require.NoError(t, err)

	msgs := received["messages"].([]any)
	require.Len(t, msgs, 3)
	first := msgs[0].(map[string]any)
	assert.Equal(t, "hema-1", first["hemaId"])
	assert.Equal(t, "278", first["voucherId"])
	assert.Equal(t, "HEMA", first["characteristic"])
}

// Given: a batch of due reminders and a reachable Selligent endpoint
// When: sending the reminders
// Then: the Authorization header carries the configured API key
func TestSendReminders_Success_SetsAuthorizationHeader(t *testing.T) {
	var authHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sender := email.NewSelligentSender(srv.URL, "my-secret-key")
	err := sender.SendReminders(context.Background(), reminders(1))
	require.NoError(t, err)

	assert.Equal(t, "Bearer my-secret-key", authHeader)
}

// Given: Selligent returns a non-2xx status code
// When: sending the reminders
// Then: an error is returned
func TestSendReminders_NonSuccessStatus_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	sender := email.NewSelligentSender(srv.URL, "test-api-key")
	err := sender.SendReminders(context.Background(), reminders(1))
	require.Error(t, err)
}

// Given: Selligent is unreachable
// When: sending the reminders
// Then: an error is returned
func TestSendReminders_Unreachable_ReturnsError(t *testing.T) {
	sender := email.NewSelligentSender("http://localhost:0", "test-api-key")
	err := sender.SendReminders(context.Background(), reminders(1))
	require.Error(t, err)
}
