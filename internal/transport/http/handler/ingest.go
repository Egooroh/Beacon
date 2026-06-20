package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/Egooroh/beacon/internal/domain"
	"github.com/Egooroh/beacon/internal/transport/http/middleware"
)

const maxIngestBodyBytes = 1 << 20 // 1 MiB

// Ingester is the use case interface consumed by the ingest handler.
type Ingester interface {
	Accept(ctx context.Context, rawToken string, payload []byte) error
}

// IngestHandler handles POST /api/v1/ingest.
type IngestHandler struct {
	uc Ingester
}

// NewIngestHandler creates an IngestHandler.
func NewIngestHandler(uc Ingester) *IngestHandler {
	return &IngestHandler{uc: uc}
}

// Handle accepts a raw event payload, returns 202 on success (FR-3).
func (h *IngestHandler) Handle(w http.ResponseWriter, r *http.Request) {
	token := middleware.TokenFromContext(r.Context())

	r.Body = http.MaxBytesReader(w, r.Body, maxIngestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "request body too large or unreadable", http.StatusBadRequest)
		return
	}
	if !json.Valid(body) {
		http.Error(w, "invalid JSON", http.StatusUnprocessableEntity)
		return
	}

	if err := h.uc.Accept(r.Context(), token, body); err != nil {
		switch {
		case errors.Is(err, domain.ErrUnauthorized):
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		case errors.Is(err, domain.ErrInvalidInput):
			http.Error(w, "unprocessable entity", http.StatusUnprocessableEntity)
		default:
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "queued"})
}
