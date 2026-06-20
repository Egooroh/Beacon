package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Egooroh/beacon/internal/domain"
)

// ProjectCreator is the use case interface consumed by the project handler.
type ProjectCreator interface {
	Create(ctx context.Context, name string) (*domain.Project, string, error)
}

// ProjectHandler handles project management endpoints.
type ProjectHandler struct {
	uc ProjectCreator
}

// NewProjectHandler creates a ProjectHandler.
func NewProjectHandler(uc ProjectCreator) *ProjectHandler {
	return &ProjectHandler{uc: uc}
}

type createProjectRequest struct {
	Name string `json:"name"`
}

type createProjectResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	IngestToken string `json:"ingest_token"`
}

// Create handles POST /api/v1/projects (FR-19).
func (h *ProjectHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	p, rawToken, err := h.uc.Create(r.Context(), req.Name)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidInput) {
			http.Error(w, "project name is required", http.StatusUnprocessableEntity)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(createProjectResponse{
		ID:          p.ID,
		Name:        p.Name,
		IngestToken: rawToken,
	})
}
