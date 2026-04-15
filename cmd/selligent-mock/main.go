// Package main is a development tool that mimics the Selligent bulk email API.
// It accepts POST requests to /email/v1/messages/send, logs the received
// messages, and always returns 200 OK. Use it locally instead of pointing the
// CES server at the real Selligent endpoint.
package main

import (
	"encoding/json"
	"flag"
	"log/slog"
	"net/http"
	"os"
)

func main() {
	addr := flag.String("addr", ":8080", "address to listen on")
	flag.Parse()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /email/v1/messages/send", handleSend)

	slog.Info("selligent mock listening", "addr", *addr)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}

// handleSend logs each message in the received batch and returns 200 OK.
func handleSend(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Messages []map[string]any `json:"messages"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		slog.Error("decode request", "error", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	slog.Info("received email batch", "count", len(body.Messages))
	for i, msg := range body.Messages {
		slog.Info("  message",
			"index", i+1,
			"hemaId", msg["hemaId"],
			"voucherId", msg["voucherId"],
			"characteristic", msg["characteristic"],
		)
	}

	w.WriteHeader(http.StatusOK)
}
