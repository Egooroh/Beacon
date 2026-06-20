package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Egooroh/beacon/internal/domain"
)

// Subscriber creates and lists notification subscriptions.
type Subscriber interface {
	Subscribe(ctx context.Context, projectID, platform, chatID string) (*domain.Subscription, error)
	List(ctx context.Context, projectID string) ([]*domain.Subscription, error)
}

// SubscriptionHandler exposes subscription management via HTTP.
type SubscriptionHandler struct {
	uc Subscriber
}

// NewSubscriptionHandler creates a SubscriptionHandler.
func NewSubscriptionHandler(uc Subscriber) *SubscriptionHandler {
	return &SubscriptionHandler{uc: uc}
}

// Create handles POST /api/v1/projects/{id}/subscriptions.
func (h *SubscriptionHandler) Create(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")

	var body struct {
		Platform string `json:"platform"`
		ChatID   string `json:"chat_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
		return
	}

	sub, err := h.uc.Subscribe(r.Context(), projectID, body.Platform, body.ChatID)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrNotFound):
			http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
		case errors.Is(err, domain.ErrInvalidInput):
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		default:
			http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(toSubResponse(sub))
}

// List handles GET /api/v1/projects/{id}/subscriptions.
func (h *SubscriptionHandler) List(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")

	subs, err := h.uc.List(r.Context(), projectID)
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}

	resp := make([]subResponse, len(subs))
	for i, s := range subs {
		resp[i] = toSubResponse(s)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

type subResponse struct {
	ID        string `json:"id"`
	ProjectID string `json:"project_id"`
	Platform  string `json:"platform"`
	ChatID    string `json:"chat_id"`
}

func toSubResponse(s *domain.Subscription) subResponse {
	return subResponse{
		ID:        s.ID,
		ProjectID: s.ProjectID,
		Platform:  s.Platform,
		ChatID:    s.ChatID,
	}
}
