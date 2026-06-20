package domain

import "time"

// IssueStatus is the lifecycle state of an issue.
type IssueStatus string

const (
	// StatusOpen is the default; the issue generates immediate alerts.
	StatusOpen IssueStatus = "open"
	// StatusResolved marks a closed issue; a new event causes a regression alert.
	StatusResolved IssueStatus = "resolved"
	// StatusMuted suppresses alerts while still counting events.
	StatusMuted IssueStatus = "muted"
	// StatusIgnored disables all processing for the issue.
	StatusIgnored IssueStatus = "ignored"
)

// Issue is a group of Events that share the same Fingerprint.
type Issue struct {
	ID          string
	ProjectID   string
	Fingerprint Fingerprint
	Title       string
	Level       Level
	Status      IssueStatus
	EventsCount int64
	FirstSeenAt time.Time
	LastSeenAt  time.Time
	LastAlertAt *time.Time // nil until the first alert is sent
}

// IsAlertable reports whether the issue should trigger immediate notifications.
func (i *Issue) IsAlertable() bool {
	return i.Status == StatusOpen
}
