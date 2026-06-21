package telegrambot

import (
	"context"
	"fmt"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/Egooroh/beacon/internal/domain"
)

// ── Keyboards ─────────────────────────────────────────────────────────────────

func mainMenuKb() *tgbotapi.InlineKeyboardMarkup {
	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("➕ New Project", "newproject"),
			tgbotapi.NewInlineKeyboardButtonData("📋 My Projects", "myprojects"),
		),
	)
	return &kb
}

func langKb() *tgbotapi.InlineKeyboardMarkup {
	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Go", "lang:go"),
			tgbotapi.NewInlineKeyboardButtonData("Python", "lang:python"),
			tgbotapi.NewInlineKeyboardButtonData("Node.js", "lang:node"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("PHP", "lang:php"),
			tgbotapi.NewInlineKeyboardButtonData("C#", "lang:csharp"),
		),
	)
	return &kb
}

// ── Dispatcher ────────────────────────────────────────────────────────────────

func (b *Bot) handleUpdate(ctx context.Context, u *tgbotapi.Update) {
	switch {
	case u.CallbackQuery != nil:
		b.handleCallback(ctx, u.CallbackQuery)
	case u.Message != nil && u.Message.IsCommand():
		b.handleCommand(ctx, u.Message)
	case u.Message != nil:
		b.handleText(ctx, u.Message)
	}
}

func (b *Bot) handleCommand(_ context.Context, msg *tgbotapi.Message) {
	switch msg.Command() {
	case "start", "help":
		b.cmdStart(msg.Chat.ID)
	default:
		b.send(msg.Chat.ID, "Unknown command. Use /start.", nil)
	}
}

func (b *Bot) handleText(ctx context.Context, msg *tgbotapi.Message) {
	chatID := msg.Chat.ID
	st := b.state.get(chatID)
	if st.step == stepAwaitingProjectName {
		b.doCreateProject(ctx, chatID, strings.TrimSpace(msg.Text), st.lang)
		return
	}
	b.cmdStart(chatID)
}

func (b *Bot) handleCallback(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	b.answerCallback(cb.ID)
	chatID := cb.Message.Chat.ID

	parts := strings.SplitN(cb.Data, ":", 3)
	switch parts[0] {
	case "newproject":
		b.cbNewProject(chatID)
	case "myprojects":
		b.cbMyProjects(ctx, chatID)
	case "lang":
		if len(parts) == 2 {
			b.cbLangSelected(chatID, parts[1])
		}
	case "issues":
		if len(parts) == 2 {
			b.cbListIssues(ctx, chatID, parts[1])
		}
	case "status":
		if len(parts) == 3 {
			b.cbSetStatus(ctx, chatID, parts[1], parts[2])
		}
	}
}

// ── Command handlers ──────────────────────────────────────────────────────────

func (b *Bot) cmdStart(chatID int64) {
	b.state.clear(chatID)
	text := "<b>Beacon</b> — self-hosted error aggregator\n\n" +
		"Create a project and paste the snippet into your app.\n" +
		"All errors will be sent directly to this chat."
	b.send(chatID, text, mainMenuKb())
}

func (b *Bot) cbNewProject(chatID int64) {
	b.send(chatID, "Choose your project language:", langKb())
}

func (b *Bot) cbLangSelected(chatID int64, lang string) {
	if _, _, ok := buildSnippet(lang, "", ""); !ok {
		b.send(chatID, "Unknown language. Please try again.", langKb())
		return
	}
	b.state.set(chatID, &userState{step: stepAwaitingProjectName, lang: lang})
	b.send(chatID, "Enter your project name:", nil)
}

func (b *Bot) cbMyProjects(ctx context.Context, chatID int64) {
	projects, err := b.userProjects.ListProjectsByChatID(ctx, chatIDStr(chatID), "telegram")
	if err != nil {
		b.log.Error("list projects by chat", "chat_id", chatID, "error", err)
		b.send(chatID, "Failed to load projects. Please try again.", nil)
		return
	}
	if len(projects) == 0 {
		b.send(chatID, "You have no projects yet.\n\nPress <b>New Project</b> to create one.", mainMenuKb())
		return
	}

	var rows [][]tgbotapi.InlineKeyboardButton
	for _, p := range projects {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📁 "+p.Name, "issues:"+p.ID),
		))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("➕ New Project", "newproject"),
	))
	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	b.send(chatID, "<b>Your projects:</b>", &kb)
}

func (b *Bot) cbListIssues(ctx context.Context, chatID int64, projectID string) {
	issues, total, err := b.issues.List(ctx, projectID, string(domain.StatusOpen), 5, 0)
	if err != nil {
		b.log.Error("list issues for bot", "project_id", projectID, "error", err)
		b.send(chatID, "Failed to load issues. Please try again.", nil)
		return
	}
	if len(issues) == 0 {
		b.send(chatID, "✨ No open issues!", mainMenuKb())
		return
	}

	var sb strings.Builder
	_, _ = fmt.Fprintf(&sb, "<b>Open issues</b> (%d total)\n\n", total)
	for i, iss := range issues {
		_, _ = fmt.Fprintf(&sb, "%d. [%s] %s\n   Events: %d\n\n",
			i+1, iss.Level, escHTML(iss.Title), iss.EventsCount)
	}

	var rows [][]tgbotapi.InlineKeyboardButton
	for i, iss := range issues {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf("✅ Resolve #%d", i+1),
				"status:"+iss.ID+":resolved",
			),
			tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf("🔇 Mute #%d", i+1),
				"status:"+iss.ID+":muted",
			),
		))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("« Back", "myprojects"),
	))
	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	b.send(chatID, sb.String(), &kb)
}

func (b *Bot) cbSetStatus(ctx context.Context, chatID int64, issueID, status string) {
	if err := b.issues.SetStatus(ctx, issueID, domain.IssueStatus(status)); err != nil {
		b.log.Error("set issue status from bot", "issue_id", issueID, "status", status, "error", err)
		b.send(chatID, "Failed to update issue status.", nil)
		return
	}
	label := map[string]string{
		"resolved": "✅ Resolved",
		"muted":    "🔇 Muted",
		"open":     "🔓 Reopened",
	}[status]
	if label == "" {
		label = "Updated"
	}
	b.send(chatID, label, mainMenuKb())
}

// ── Project creation ──────────────────────────────────────────────────────────

func (b *Bot) doCreateProject(ctx context.Context, chatID int64, name, lang string) {
	b.state.clear(chatID)

	if name == "" {
		b.send(chatID, "Project name cannot be empty. Please try again:", nil)
		b.state.set(chatID, &userState{step: stepAwaitingProjectName, lang: lang})
		return
	}

	proj, token, err := b.projects.Create(ctx, name)
	if err != nil {
		b.log.Error("create project from bot", "name", name, "error", err)
		b.send(chatID, "Failed to create project. Please try again.", mainMenuKb())
		return
	}

	_, err = b.subs.Subscribe(ctx, proj.ID, "telegram", chatIDStr(chatID))
	if err != nil {
		b.log.Error("auto-subscribe after project create", "project_id", proj.ID, "error", err)
	}

	label, code, ok := buildSnippet(lang, b.publicURL, token)
	if !ok {
		code = fmt.Sprintf("URL: %s\nToken: %s", b.publicURL, token)
		label = "Integration"
	}

	text := fmt.Sprintf(
		"✅ <b>Project created!</b>\n\n"+
			"📛 <b>%s</b>\n"+
			"🔑 ID: <code>%s</code>\n\n"+
			"<b>%s integration:</b>\n<pre><code>%s</code></pre>\n\n"+
			"Errors will appear in this chat.",
		escHTML(proj.Name), proj.ID, label, escHTML(code),
	)
	b.send(chatID, text, mainMenuKb())
}

// escHTML escapes characters that have special meaning in Telegram HTML mode.
func escHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
