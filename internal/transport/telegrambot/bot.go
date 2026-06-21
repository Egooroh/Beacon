// Package telegrambot implements a Telegram bot transport adapter for Beacon.
// Users interact with the bot to create projects and receive error alerts.
package telegrambot

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/Egooroh/beacon/internal/domain"
)

// ProjectCreator creates new Beacon projects.
type ProjectCreator interface {
	Create(ctx context.Context, name string) (*domain.Project, string, error)
}

// Subscriber creates notification subscriptions for projects.
type Subscriber interface {
	Subscribe(ctx context.Context, projectID, platform, chatID string) (*domain.Subscription, error)
}

// IssueManager lists and transitions issues.
type IssueManager interface {
	List(ctx context.Context, projectID, status string, limit, offset int) ([]*domain.Issue, int64, error)
	SetStatus(ctx context.Context, issueID string, status domain.IssueStatus) error
}

// UserProjectLister finds projects a Telegram chat is subscribed to.
type UserProjectLister interface {
	ListProjectsByChatID(ctx context.Context, chatID, platform string) ([]*domain.Project, error)
}

// Bot is the Telegram bot transport adapter for Beacon.
type Bot struct {
	api          *tgbotapi.BotAPI
	projects     ProjectCreator
	subs         Subscriber
	issues       IssueManager
	userProjects UserProjectLister
	state        *stateStore
	log          *slog.Logger
	publicURL    string
}

// New creates a Bot. publicURL is used in code snippets shown to users
// (e.g. "https://beacon.example.com"); leave empty to show a placeholder.
func New(
	token string,
	projects ProjectCreator,
	subs Subscriber,
	issues IssueManager,
	userProjects UserProjectLister,
	log *slog.Logger,
	publicURL string,
) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("create telegram bot: %w", err)
	}
	url := publicURL
	if url == "" {
		url = "https://your-beacon.example.com"
	}
	return &Bot{
		api:          api,
		projects:     projects,
		subs:         subs,
		issues:       issues,
		userProjects: userProjects,
		state:        newStateStore(),
		log:          log,
		publicURL:    url,
	}, nil
}

// Run starts long-polling and blocks until ctx is cancelled.
func (b *Bot) Run(ctx context.Context) {
	cfg := tgbotapi.NewUpdate(0)
	cfg.Timeout = 30
	updates := b.api.GetUpdatesChan(cfg)

	b.log.Info("telegram bot started", "username", b.api.Self.UserName)

	for {
		select {
		case <-ctx.Done():
			b.api.StopReceivingUpdates()
			return
		case update, ok := <-updates:
			if !ok {
				return
			}
			b.handleUpdate(ctx, update)
		}
	}
}

func (b *Bot) send(chatID int64, text string, kb *tgbotapi.InlineKeyboardMarkup) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeHTML
	msg.DisableWebPagePreview = true
	if kb != nil {
		msg.ReplyMarkup = kb
	}
	if _, err := b.api.Send(msg); err != nil {
		b.log.Error("send telegram message", "chat_id", chatID, "error", err)
	}
}

func (b *Bot) answerCallback(id string) {
	cb := tgbotapi.NewCallback(id, "")
	if _, err := b.api.Request(cb); err != nil {
		b.log.Error("answer callback", "error", err)
	}
}

func chatIDStr(id int64) string { return strconv.FormatInt(id, 10) }
