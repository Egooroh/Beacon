package pgstore

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Egooroh/beacon/internal/domain"
)

// ProjectRepository accesses projects in PostgreSQL.
type ProjectRepository struct {
	db *pgxpool.Pool
}

// NewProjectRepository creates a ProjectRepository backed by the given pool.
func NewProjectRepository(db *pgxpool.Pool) *ProjectRepository {
	return &ProjectRepository{db: db}
}

// FindByTokenHash looks up a project by its stored token hash.
// Returns domain.ErrNotFound when no project matches the hash.
func (r *ProjectRepository) FindByTokenHash(ctx context.Context, tokenHash string) (*domain.Project, error) {
	const q = `SELECT id, name, token_hash, created_at FROM projects WHERE token_hash = $1`

	var p domain.Project
	err := r.db.QueryRow(ctx, q, tokenHash).Scan(&p.ID, &p.Name, &p.TokenHash, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find project by token hash: %w", err)
	}
	return &p, nil
}

// FindByID looks up a project by its primary key.
// Returns domain.ErrNotFound when no project matches.
func (r *ProjectRepository) FindByID(ctx context.Context, id string) (*domain.Project, error) {
	const q = `SELECT id, name, token_hash, created_at FROM projects WHERE id = $1`

	var p domain.Project
	err := r.db.QueryRow(ctx, q, id).Scan(&p.ID, &p.Name, &p.TokenHash, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find project by id: %w", err)
	}
	return &p, nil
}

// Create inserts a new project and returns it with the DB-assigned ID and created_at.
func (r *ProjectRepository) Create(ctx context.Context, name, tokenHash string) (*domain.Project, error) {
	const q = `
		INSERT INTO projects (name, token_hash)
		VALUES ($1, $2)
		RETURNING id, created_at`

	p := &domain.Project{Name: name, TokenHash: tokenHash}
	if err := r.db.QueryRow(ctx, q, name, tokenHash).Scan(&p.ID, &p.CreatedAt); err != nil {
		return nil, fmt.Errorf("create project: %w", err)
	}
	return p, nil
}
