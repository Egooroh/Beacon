package pgstore

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Egooroh/beacon/internal/domain"
)

// SubscriptionRepository persists notification subscriptions in PostgreSQL.
type SubscriptionRepository struct {
	db *pgxpool.Pool
}

// NewSubscriptionRepository creates a SubscriptionRepository backed by the given pool.
func NewSubscriptionRepository(db *pgxpool.Pool) *SubscriptionRepository {
	return &SubscriptionRepository{db: db}
}

// Create inserts a new subscription and returns it with the DB-assigned ID.
func (r *SubscriptionRepository) Create(ctx context.Context, projectID, platform, chatID string) (*domain.Subscription, error) {
	const q = `
		INSERT INTO subscriptions (project_id, platform, chat_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (project_id, platform, chat_id) DO UPDATE SET created_at = subscriptions.created_at
		RETURNING id, project_id, platform, chat_id, created_at`

	sub := &domain.Subscription{}
	err := r.db.QueryRow(ctx, q, projectID, platform, chatID).
		Scan(&sub.ID, &sub.ProjectID, &sub.Platform, &sub.ChatID, &sub.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create subscription: %w", err)
	}
	return sub, nil
}

// ListProjectsWithSubscriptions returns distinct project IDs that have at least one subscription.
func (r *SubscriptionRepository) ListProjectsWithSubscriptions(ctx context.Context) ([]string, error) {
	const q = `SELECT DISTINCT project_id::text FROM subscriptions ORDER BY project_id`

	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list subscribed projects: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan project id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ListProjectsByChatID returns all projects where the given chat has a subscription.
func (r *SubscriptionRepository) ListProjectsByChatID(ctx context.Context, chatID, platform string) ([]*domain.Project, error) {
	const q = `
		SELECT p.id, p.name, p.token_hash, p.created_at
		FROM projects p
		JOIN subscriptions s ON s.project_id = p.id
		WHERE s.chat_id = $1 AND s.platform = $2
		ORDER BY s.created_at DESC`

	rows, err := r.db.Query(ctx, q, chatID, platform)
	if err != nil {
		return nil, fmt.Errorf("list projects by chat: %w", err)
	}
	defer rows.Close()

	var projects []*domain.Project
	for rows.Next() {
		p := &domain.Project{}
		if err := rows.Scan(&p.ID, &p.Name, &p.TokenHash, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

// ListByProject returns all subscriptions for the given project.
func (r *SubscriptionRepository) ListByProject(ctx context.Context, projectID string) ([]*domain.Subscription, error) {
	const q = `
		SELECT id, project_id, platform, chat_id, created_at
		FROM subscriptions
		WHERE project_id = $1
		ORDER BY created_at`

	rows, err := r.db.Query(ctx, q, projectID)
	if err != nil {
		return nil, fmt.Errorf("list subscriptions: %w", err)
	}
	defer rows.Close()

	var subs []*domain.Subscription
	for rows.Next() {
		sub := &domain.Subscription{}
		if err := rows.Scan(&sub.ID, &sub.ProjectID, &sub.Platform, &sub.ChatID, &sub.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan subscription: %w", err)
		}
		subs = append(subs, sub)
	}
	return subs, rows.Err()
}
