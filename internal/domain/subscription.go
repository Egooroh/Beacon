package domain

import "time"

// Subscription binds a Project to a notification channel (Telegram chat or Slack channel).
type Subscription struct {
	ID        string
	ProjectID string
	Platform  string // "telegram" | "slack"
	ChatID    string
	CreatedAt time.Time
}
