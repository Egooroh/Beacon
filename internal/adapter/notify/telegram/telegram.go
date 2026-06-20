package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Egooroh/beacon/internal/domain"
)

const apiBase = "https://api.telegram.org"

// Notifier sends Beacon alerts to Telegram chats via Bot API.
type Notifier struct {
	token  string
	client *http.Client
}

// New creates a Telegram Notifier. token is the Bot API token from @BotFather.
func New(token string) *Notifier {
	return &Notifier{
		token: token,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// Platform implements alerting.Notifier.
func (n *Notifier) Platform() string { return "telegram" }

// Notify sends an alert message to the given Telegram chat_id.
func (n *Notifier) Notify(ctx context.Context, alert domain.Alert, chatID string) error {
	return n.Send(ctx, chatID, formatAlert(alert))
}

// Send delivers raw text to a Telegram chat. Used by the digest service.
func (n *Notifier) Send(ctx context.Context, chatID, text string) error {
	body, err := json.Marshal(map[string]string{
		"chat_id": chatID,
		"text":    text,
	})
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	url := fmt.Sprintf("%s/bot%s/sendMessage", apiBase, n.token)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("send message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram API returned %d", resp.StatusCode)
	}
	return nil
}

// formatAlert renders a plain-text notification message.
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
