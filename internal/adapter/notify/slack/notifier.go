package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Egooroh/beacon/internal/domain"
)

const apiBase = "https://slack.com/api/chat.postMessage"

// Notifier sends Beacon alerts and digests to Slack channels via the Web API.
// Implements both alerting.Notifier and digest.Notifier.
type Notifier struct {
	token  string
	client *http.Client
}

// New creates a Slack Notifier. token is a Bot OAuth token (xoxb-...).
func New(token string) *Notifier {
	return &Notifier{
		token:  token,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// Platform implements alerting.Notifier and digest.Notifier.
func (n *Notifier) Platform() string { return "slack" }

// Notify formats a domain.Alert and sends it to the given Slack channel.
// Implements alerting.Notifier.
func (n *Notifier) Notify(ctx context.Context, alert domain.Alert, chatID string) error {
	return n.Send(ctx, chatID, formatAlert(alert))
}

// Send delivers raw text to a Slack channel. Used by the digest service.
// Implements digest.Notifier.
func (n *Notifier) Send(ctx context.Context, chatID, text string) error {
	body, err := json.Marshal(map[string]string{
		"channel": chatID,
		"text":    text,
	})
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBase, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+n.token)

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("send message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack API returned %d", resp.StatusCode)
	}
	return nil
}

func formatAlert(a domain.Alert) string {
	projectName := ""
	if a.Project != nil {
		projectName = a.Project.Name
	}

	var prefix string
	switch a.Type {
	case domain.AlertNewIssue:
		prefix = "[NEW]"
	case domain.AlertRegression:
		prefix = "[REGRESSION]"
	case domain.AlertSpike:
		prefix = "[SPIKE]"
	default:
		prefix = "[ALERT]"
	}

	return fmt.Sprintf(
		"%s %s\nProject: %s | Level: %s\nFirst seen: %s",
		prefix,
		a.Issue.Title,
		projectName,
		a.Issue.Level,
		a.Issue.FirstSeenAt.UTC().Format(time.RFC3339),
	)
}
