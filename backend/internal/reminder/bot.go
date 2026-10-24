package reminder

import (
	"context"
	"errors"
	"html"
	"log/slog"
	"sort"
	"strings"

	"github.com/AVGsync/invoice-keeper/backend/internal/clock"
	"github.com/AVGsync/invoice-keeper/backend/internal/infrastructure/telegram"
	"github.com/AVGsync/invoice-keeper/backend/internal/model"
)

// BotService — операции бота поверх движка: кнопка «Оплачено» и read-only
// команды /today, /week, /overdue. Отдаётся в telegrambot.Service. Авторизацию
// по from.id делает вызывающий; сюда приходит уже разрешённый actorUserID (§6.3).
type BotService struct {
	src    OccurrenceSource
	payer  Payer
	notif  NotificationStore
	sender BotSender
	clk    clock.Clock
	cfg    Config
}

// NewBotService собирает сервис бота движка.
func NewBotService(src OccurrenceSource, payer Payer, notif NotificationStore, sender BotSender, clk clock.Clock, cfg Config) *BotService {
	cfg.withDefaults()
	return &BotService{src: src, payer: payer, notif: notif, sender: sender, clk: clk, cfg: cfg}
}

// MarkPaidCallback обрабатывает нажатие «Оплачено». occHex — id периода из
// callback_data (p:<uuid hex>). Возвращает текст для answerCallbackQuery и флаг
// alert. При успехе правит все отправленные сообщения периода в «Оплачено».
func (b *BotService) MarkPaidCallback(ctx context.Context, occHex, actorUserID, actorName string) (text string, alert bool) {
	occID := hexToUUID(occHex)
	if occID == "" {
		return "Кнопка устарела.", false
	}

	err := b.payer.MarkPaid(ctx, occID, b.clk.Today(), actorUserID)
	switch {
	case err == nil:
		b.editOccurrencePaid(ctx, occID, actorName)
		return "Отмечено", false
	case errors.Is(err, ErrAlreadyPaid):
		b.editOccurrencePaid(ctx, occID, actorName)
		return "Уже оплачено", false
	case errors.Is(err, ErrForbidden):
		return "Отметить может ответственный или администратор.", true
	case errors.Is(err, ErrPaymentEnded):
		return "Платёж завершён.", true
	case errors.Is(err, ErrOccurrenceNotFound):
		return "Период не найден.", true
	default:
		slog.Error("reminder bot: mark paid", "error", err, "occurrence_id", occID)
		return "Ошибка, попробуйте позже.", true
	}
}

// editOccurrencePaid правит все открытые/отправленные сообщения периода в
// «Оплачено» и убирает кнопки.
func (b *BotService) editOccurrencePaid(ctx context.Context, occID, actorName string) {
	occs, err := b.src.GetByIDs(ctx, []string{occID})
	if err != nil || len(occs) == 0 {
		return
	}
	rows, err := b.notif.OpenByOccurrence(ctx, occID)
	if err != nil {
		slog.Error("reminder bot: open by occurrence", "error", err, "occurrence_id", occID)
		return
	}
	now := b.clk.Now()
	text := paidText(occs[0], actorName)
	for _, row := range rows {
		if row.ChatID == nil || row.MessageID == nil {
			continue
		}
		cctx, cancel := context.WithTimeout(ctx, b.cfg.SendTimeout)
		_, eerr := b.sender.EditMessageText(cctx, telegram.EditMessageTextParams{
			ChatID: *row.ChatID, MessageID: *row.MessageID, Text: text, ParseMode: "HTML", ReplyMarkup: emptyMarkup(),
		})
		cancel()
		if eerr != nil {
			var apiErr *telegram.APIError
			if !(errors.As(eerr, &apiErr) && apiErr.IsMessageNotModified()) {
				slog.Error("reminder bot: edit paid message", "error", eerr, "id", row.ID)
				continue
			}
		}
		if serr := b.notif.SetShownStatus(ctx, row.ID, model.ShownStatusPaid, now); serr != nil {
			slog.Error("reminder bot: set shown paid", "error", serr, "id", row.ID)
		}
	}
}

// ResetTeamQueue сбрасывает отложенные (failed) командные строки на немедленную
// отправку — например, после нового /bind (§5.4).
func (b *BotService) ResetTeamQueue(ctx context.Context) {
	if err := b.notif.ResetFailedByRecipient(ctx, model.NotificationRecipientTeam, b.clk.Now()); err != nil {
		slog.Error("reminder bot: reset team queue", "error", err)
	}
}

// Today, Week, Overdue — текстовые сводки для команд бота (read-only).
func (b *BotService) Today(ctx context.Context) string {
	today := b.clk.Today()
	occs, err := b.src.DueBetween(ctx, today, today)
	return b.renderList(err, "Сегодня платежей нет.", "Платежи на сегодня:", occs, today)
}

func (b *BotService) Week(ctx context.Context) string {
	today := b.clk.Today()
	occs, err := b.src.DueBetween(ctx, today, today.AddDays(7))
	return b.renderList(err, "На ближайшую неделю платежей нет.", "Платежи на 7 дней:", occs, today)
}

func (b *BotService) Overdue(ctx context.Context) string {
	today := b.clk.Today()
	occs, err := b.src.DueBetween(ctx, today.AddDays(-3650), today.AddDays(-1))
	return b.renderList(err, "Просроченных платежей нет.", "Просроченные платежи:", occs, today)
}

func (b *BotService) renderList(err error, empty, header string, occs []model.ReminderOccurrence, today clock.Date) string {
	if err != nil {
		slog.Error("reminder bot: list occurrences", "error", err)
		return "Не удалось получить список. Попробуйте позже."
	}
	if len(occs) == 0 {
		return empty
	}
	sort.Slice(occs, func(i, j int) bool { return occs[i].DueDate.Before(occs[j].DueDate) })
	var sb strings.Builder
	sb.WriteString(header)
	sb.WriteString("\n")
	for _, o := range occs {
		sb.WriteString("• <b>")
		sb.WriteString(html.EscapeString(truncate(o.Title, maxTitleLen)))
		sb.WriteString("</b> — ")
		sb.WriteString(html.EscapeString(FormatMoney(o.Amount, o.Currency)))
		sb.WriteString(" — ")
		sb.WriteString(formatDate(o.DueDate))
		sb.WriteString(" (")
		sb.WriteString(html.EscapeString(relativeDay(today, o.DueDate)))
		sb.WriteString(")\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}
