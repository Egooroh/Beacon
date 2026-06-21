package handler

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/Egooroh/beacon/internal/domain"
)

// SentryIngester saves a pre-parsed event for a specific project.
type SentryIngester interface {
	AcceptForProject(ctx context.Context, projectID string, event *domain.Event) error
}

// SentryParser converts a raw Sentry webhook body to a domain Event.
type SentryParser interface {
	Parse(raw []byte) (*domain.Event, error)
}

// SentryWebhookHandler handles POST /api/v1/projects/{id}/webhook/sentry.
// It validates the HMAC-SHA256 signature (when secret is configured) and
// feeds the parsed event into the ingest pipeline.
type SentryWebhookHandler struct {
	ingester SentryIngester
	parser   SentryParser
	secret   string
}

// NewSentryWebhookHandler creates a SentryWebhookHandler.
// secret is the BEACON_SENTRY_SECRET value; empty disables HMAC validation.
func NewSentryWebhookHandler(ingester SentryIngester, parser SentryParser, secret string) *SentryWebhookHandler {
	return &SentryWebhookHandler{ingester: ingester, parser: parser, secret: secret}
}

// Handle processes a Sentry error webhook.
func (h *SentryWebhookHandler) Handle(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "read body failed", http.StatusBadRequest)
		return
	}

	if h.secret != "" {
		sig := r.Header.Get("Sentry-Hook-Signature")
		if !validateSentryHMAC(body, sig, h.secret) {
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}
	}

	event, err := h.parser.Parse(body)
	if err != nil {
		http.Error(w, "unprocessable entity", http.StatusUnprocessableEntity)
		return
	}

	projectID := chi.URLParam(r, "id")
	if err := h.ingester.AcceptForProject(r.Context(), projectID, event); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte(`{"status":"queued"}`))
}

// validateSentryHMAC verifies the Sentry-Hook-Signature header value.
// Expected format: "sha256=<hex_signature>".
func validateSentryHMAC(body []byte, header, secret string) bool {
	const prefix = "sha256="
	if !strings.HasPrefix(header, prefix) {
		return false
	}
	sig, err := hex.DecodeString(header[len(prefix):])
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hmac.Equal(sig, mac.Sum(nil))
}
