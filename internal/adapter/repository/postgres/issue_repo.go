package pgstore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Egooroh/beacon/internal/domain"
)

// IssueRepository persists grouped incidents in PostgreSQL.
type IssueRepository struct {
	db *pgxpool.Pool
}

// NewIssueRepository creates an IssueRepository backed by the given pool.
func NewIssueRepository(db *pgxpool.Pool) *IssueRepository {
	return &IssueRepository{db: db}
}

// Upsert creates a new issue or increments its event count.
// created is true when the row is first inserted (events_count = 1).
func (r *IssueRepository) Upsert(ctx context.Context, ev *domain.Event) (*domain.Issue, bool, error) {
	const q = `
		INSERT INTO issues (project_id, fingerprint, title, level, status, events_count, first_seen_at, last_seen_at)
		VALUES ($1, $2, $3, $4, 'open', 1, $5, $5)
		ON CONFLICT (project_id, fingerprint) DO UPDATE SET
			events_count = issues.events_count + 1,
			last_seen_at = EXCLUDED.last_seen_at,
			level        = EXCLUDED.level
		RETURNING
			id, project_id, fingerprint, title, level, status,
			events_count, first_seen_at, last_seen_at, last_alert_at,
			(events_count = 1) AS created`

	now := time.Now()
	title := issueTitle(ev)

	var (
		issue     domain.Issue
		fpStr     string
		levelStr  string
		statusStr string
		created   bool
	)
	err := r.db.QueryRow(ctx, q,
		ev.ProjectID,
		string(ev.Fingerprint),
		title,
		string(ev.Level),
		now,
	).Scan(
		&issue.ID, &issue.ProjectID, &fpStr, &issue.Title,
		&levelStr, &statusStr,
		&issue.EventsCount, &issue.FirstSeenAt, &issue.LastSeenAt, &issue.LastAlertAt,
		&created,
	)
	if err != nil {
		return nil, false, fmt.Errorf("upsert issue: %w", err)
	}
	issue.Fingerprint = domain.Fingerprint(fpStr)
	issue.Level = domain.Level(levelStr)
	issue.Status = domain.IssueStatus(statusStr)
	return &issue, created, nil
}

// TopByCount returns the top-n most frequent open issues in a project since the given time.
func (r *IssueRepository) TopByCount(ctx context.Context, projectID string, since time.Time, limit int) ([]*domain.Issue, error) {
	const q = `
		SELECT id, project_id, fingerprint, title, level, status,
		       events_count, first_seen_at, last_seen_at, last_alert_at
		FROM issues
		WHERE project_id = $1 AND last_seen_at >= $2
		ORDER BY events_count DESC
		LIMIT $3`

	rows, err := r.db.Query(ctx, q, projectID, since, limit)
	if err != nil {
		return nil, fmt.Errorf("top by count: %w", err)
	}
	defer rows.Close()

	var issues []*domain.Issue
	for rows.Next() {
		var (
			iss       domain.Issue
			fpStr     string
			levelStr  string
			statusStr string
		)
		if err := rows.Scan(
			&iss.ID, &iss.ProjectID, &fpStr, &iss.Title,
			&levelStr, &statusStr,
			&iss.EventsCount, &iss.FirstSeenAt, &iss.LastSeenAt, &iss.LastAlertAt,
		); err != nil {
			return nil, fmt.Errorf("scan issue: %w", err)
		}
		iss.Fingerprint = domain.Fingerprint(fpStr)
		iss.Level = domain.Level(levelStr)
		iss.Status = domain.IssueStatus(statusStr)
		issues = append(issues, &iss)
	}
	return issues, rows.Err()
}

// CountInWindow returns the number of events for an issue within the given time window.
func (r *IssueRepository) CountInWindow(ctx context.Context, issueID string, window time.Duration) (int64, error) {
	const q = `SELECT COUNT(*) FROM events WHERE issue_id = $1 AND received_at > $2`
	since := time.Now().Add(-window)
	var count int64
	if err := r.db.QueryRow(ctx, q, issueID, since).Scan(&count); err != nil {
		return 0, fmt.Errorf("count in window: %w", err)
	}
	return count, nil
}

// UpdateStatus changes the lifecycle status of an issue.
func (r *IssueRepository) UpdateStatus(ctx context.Context, issueID string, s domain.IssueStatus) error {
	const q = `UPDATE issues SET status = $2 WHERE id = $1`
	if _, err := r.db.Exec(ctx, q, issueID, string(s)); err != nil {
		return fmt.Errorf("update status: %w", err)
	}
	return nil
}

// UpdateLastAlertAt records when an alert was last sent for an issue.
func (r *IssueRepository) UpdateLastAlertAt(ctx context.Context, issueID string, t time.Time) error {
	const q = `UPDATE issues SET last_alert_at = $2 WHERE id = $1`
	if _, err := r.db.Exec(ctx, q, issueID, t); err != nil {
		return fmt.Errorf("update last_alert_at: %w", err)
	}
	return nil
}

// ListOpen returns up to limit open issues across all projects, newest-first.
func (r *IssueRepository) ListOpen(ctx context.Context, limit int) ([]*domain.Issue, error) {
	const q = `
		SELECT id, project_id, fingerprint, title, level, status,
		       events_count, first_seen_at, last_seen_at, last_alert_at
		FROM issues
		WHERE status = 'open'
		ORDER BY last_seen_at DESC
		LIMIT $1`

	rows, err := r.db.Query(ctx, q, limit)
	if err != nil {
		return nil, fmt.Errorf("list open issues: %w", err)
	}
	defer rows.Close()

	var issues []*domain.Issue
	for rows.Next() {
		var (
			iss       domain.Issue
			fpStr     string
			levelStr  string
			statusStr string
		)
		if err := rows.Scan(
			&iss.ID, &iss.ProjectID, &fpStr, &iss.Title,
			&levelStr, &statusStr,
			&iss.EventsCount, &iss.FirstSeenAt, &iss.LastSeenAt, &iss.LastAlertAt,
		); err != nil {
			return nil, fmt.Errorf("scan open issue: %w", err)
		}
		iss.Fingerprint = domain.Fingerprint(fpStr)
		iss.Level = domain.Level(levelStr)
		iss.Status = domain.IssueStatus(statusStr)
		issues = append(issues, &iss)
	}
	return issues, rows.Err()
}

// FindByID fetches a single issue by primary key.
// Returns domain.ErrNotFound when no issue matches.
func (r *IssueRepository) FindByID(ctx context.Context, id string) (*domain.Issue, error) {
	const q = `
		SELECT id, project_id, fingerprint, title, level, status,
		       events_count, first_seen_at, last_seen_at, last_alert_at
		FROM issues WHERE id = $1`

	var (
		iss       domain.Issue
		fpStr     string
		levelStr  string
		statusStr string
	)
	err := r.db.QueryRow(ctx, q, id).Scan(
		&iss.ID, &iss.ProjectID, &fpStr, &iss.Title,
		&levelStr, &statusStr,
		&iss.EventsCount, &iss.FirstSeenAt, &iss.LastSeenAt, &iss.LastAlertAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find issue: %w", err)
	}
	iss.Fingerprint = domain.Fingerprint(fpStr)
	iss.Level = domain.Level(levelStr)
	iss.Status = domain.IssueStatus(statusStr)
	return &iss, nil
}

// ListByProject returns paginated issues for a project, optionally filtered by status.
// An empty status string returns all statuses. Returns issues and total matching count.
func (r *IssueRepository) ListByProject(ctx context.Context, projectID, status string, limit, offset int) ([]*domain.Issue, int64, error) {
	const q = `
		SELECT id, project_id, fingerprint, title, level, status,
		       events_count, first_seen_at, last_seen_at, last_alert_at,
		       COUNT(*) OVER() AS total
		FROM issues
		WHERE project_id = $1 AND ($2 = '' OR status = $2)
		ORDER BY events_count DESC
		LIMIT $3 OFFSET $4`

	rows, err := r.db.Query(ctx, q, projectID, status, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list issues: %w", err)
	}
	defer rows.Close()

	var (
		issues []*domain.Issue
		total  int64
	)
	for rows.Next() {
		var (
			iss       domain.Issue
			fpStr     string
			levelStr  string
			statusStr string
		)
		if err := rows.Scan(
			&iss.ID, &iss.ProjectID, &fpStr, &iss.Title,
			&levelStr, &statusStr,
			&iss.EventsCount, &iss.FirstSeenAt, &iss.LastSeenAt, &iss.LastAlertAt,
			&total,
		); err != nil {
			return nil, 0, fmt.Errorf("scan issue: %w", err)
		}
		iss.Fingerprint = domain.Fingerprint(fpStr)
		iss.Level = domain.Level(levelStr)
		iss.Status = domain.IssueStatus(statusStr)
		issues = append(issues, &iss)
	}
	return issues, total, rows.Err()
}

// issueTitle derives a human-readable title from an event.
func issueTitle(ev *domain.Event) string {
	if ev.Exception != nil && ev.Exception.Type != "" {
		t := ev.Exception.Type
		if ev.Exception.Value != "" {
			t += ": " + ev.Exception.Value
		}
		return truncate(t, 255)
	}
	return truncate(ev.Message, 255)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
