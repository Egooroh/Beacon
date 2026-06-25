package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/Egooroh/beacon/internal/domain"
)

// IssueManager lists issues and updates their lifecycle status.
type IssueManager interface {
	List(ctx context.Context, projectID, status string, limit, offset int) ([]*domain.Issue, int64, error)
	SetStatus(ctx context.Context, issueID string, s domain.IssueStatus) error
}

// IssueHandler exposes issue management via HTTP.
type IssueHandler struct {
	uc IssueManager
}

// NewIssueHandler creates an IssueHandler.
func NewIssueHandler(uc IssueManager) *IssueHandler {
	return &IssueHandler{uc: uc}
}

// List handles GET /api/v1/projects/{id}/issues
func (h *IssueHandler) List(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	q := r.URL.Query()

	status := q.Get("status")
	limit := parseIntParam(q.Get("limit"), 20)
	offset := parseIntParam(q.Get("offset"), 0)

	issues, total, err := h.uc.List(r.Context(), projectID, status, limit, offset)
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}

	type item struct {
		ID          string `json:"id"`
		Fingerprint string `json:"fingerprint"`
		Title       string `json:"title"`
		Level       string `json:"level"`
		Status      string `json:"status"`
		EventsCount int64  `json:"events_count"`
		FirstSeenAt string `json:"first_seen_at"`
		LastSeenAt  string `json:"last_seen_at"`
	}
	items := make([]item, len(issues))
	for i, iss := range issues {
		items[i] = item{
			ID:          iss.ID,
			Fingerprint: string(iss.Fingerprint),
			Title:       iss.Title,
			Level:       string(iss.Level),
			Status:      string(iss.Status),
			EventsCount: iss.EventsCount,
			FirstSeenAt: iss.FirstSeenAt.UTC().Format("2006-01-02T15:04:05Z"),
			LastSeenAt:  iss.LastSeenAt.UTC().Format("2006-01-02T15:04:05Z"),
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		Items  []item `json:"items"`
		Total  int64  `json:"total"`
		Limit  int    `json:"limit"`
		Offset int    `json:"offset"`
	}{Items: items, Total: total, Limit: limit, Offset: offset})
}

// SetStatus handles PATCH /api/v1/issues/{id}/status
func (h *IssueHandler) SetStatus(w http.ResponseWriter, r *http.Request) {
	issueID := chi.URLParam(r, "id")

	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
		return
	}

	if err := h.uc.SetStatus(r.Context(), issueID, domain.IssueStatus(body.Status)); err != nil {
		switch {
		case errors.Is(err, domain.ErrNotFound):
			http.Error(w, `{"error":"issue not found"}`, http.StatusNotFound)
		case errors.Is(err, domain.ErrInvalidInput):
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		default:
			http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func parseIntParam(s string, def int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil || v < 0 {
		return def
	}
	return v
}
