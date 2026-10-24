package reminder

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/AVGsync/invoice-keeper/backend/internal/clock"
	"github.com/AVGsync/invoice-keeper/backend/internal/infrastructure/telegram"
	"github.com/AVGsync/invoice-keeper/backend/internal/model"
)

// Значения по умолчанию движка (§5).
const (
	defaultOverdueMaxDays = 3                     // §5.7: просрочку шлём только первые 3 дня
	defaultPerChatGap     = 3 * time.Second       // не чаще 1 сообщения в 3 с в группу (§5.4)
	defaultGlobalGap      = 40 * time.Millisecond // ~25 сообщений в секунду суммарно
	defaultSendTimeout    = 5 * time.Second
	defaultSendLimit      = 50
	defaultReconcileLimit = 200
	defaultCleanupEvery   = time.Hour
	maxSendAttempts       = 10 // после — состояние dead (§5.4)
)

// Config — параметры движка напоминаний.
type Config struct {
	LeadDays         int           // за сколько дней до срока напоминаем (>=3)
	Tick             time.Duration // период тика
	OverdueMaxDays   int           // сколько дней после срока ещё шлём overdue (0 => default)
	PerChatGap       time.Duration // пауза между сообщениями в один чат (0 => default; отрицательное отключает)
	GlobalGap        time.Duration // пауза между любыми сообщениями
	SendTimeout      time.Duration // таймаут одиночного sendMessage
	BotUsername      string        // username бота для Direct Link Mini App в кнопке «Открыть»
	MiniAppShortName string        // short_name Mini App (BotFather /newapp); пусто => основное Mini App бота
	Demo             bool          // режим заглушки (REMINDER_SOURCE=stub): помечаем сообщения [DEMO]
	SendLimit        int           // сколько строк outbox отправляем за тик
	ReconcileLimit   int           // сколько отправленных сообщений проверяем за тик
	CleanupEvery     time.Duration // как часто чистим outbox
}

func (c *Config) withDefaults() {
	if c.LeadDays <= 0 {
		c.LeadDays = 3
	}
	if c.Tick <= 0 {
		c.Tick = 30 * time.Second
	}
	if c.OverdueMaxDays == 0 {
		c.OverdueMaxDays = defaultOverdueMaxDays
	}
	if c.PerChatGap == 0 {
		c.PerChatGap = defaultPerChatGap
	}
	if c.GlobalGap == 0 {
		c.GlobalGap = defaultGlobalGap
	}
	if c.SendTimeout <= 0 {
		c.SendTimeout = defaultSendTimeout
	}
	if c.SendLimit <= 0 {
		c.SendLimit = defaultSendLimit
	}
	if c.ReconcileLimit <= 0 {
		c.ReconcileLimit = defaultReconcileLimit
	}
	if c.CleanupEvery <= 0 {
		c.CleanupEvery = defaultCleanupEvery
	}
}

// Loop — цикл напоминаний (одна горутина, один инстанс, §5.3): enqueue -> send
// -> reconcile -> cleanup. Первый тик сразу при старте, далее каждые Tick.
type Loop struct {
	src     OccurrenceSource
	notif   NotificationStore
	chats   TeamChatStore
	sender  BotSender
	clk     clock.Clock
	ensurer OccurrenceEnsurer // необязательный
	cfg     Config
	limiter *rateLimiter

	lastCleanup time.Time
}

// NewLoop собирает цикл. ensurer может быть nil.
func NewLoop(src OccurrenceSource, notif NotificationStore, chats TeamChatStore, sender BotSender, clk clock.Clock, ensurer OccurrenceEnsurer, cfg Config) *Loop {
	cfg.withDefaults()
	return &Loop{
		src: src, notif: notif, chats: chats, sender: sender, clk: clk, ensurer: ensurer, cfg: cfg,
		limiter: &rateLimiter{perChat: cfg.PerChatGap, global: cfg.GlobalGap, last: map[int64]time.Time{}},
	}
}

// Run крутит тики до отмены ctx. Первый тик — сразу.
func (l *Loop) Run(ctx context.Context) {
	slog.Info("reminder loop: started", "tick", l.cfg.Tick, "lead_days", l.cfg.LeadDays)
	l.Tick(ctx)
	t := time.NewTicker(l.cfg.Tick)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("reminder loop: stopped")
			return
		case <-t.C:
			l.Tick(ctx)
		}
	}
}

// Tick выполняет один проход. Экспортирован для детерминированных тестов.
func (l *Loop) Tick(ctx context.Context) {
	today := l.clk.Today()
	if l.ensurer != nil {
		if err := l.ensurer.EnsureOccurrencesUntil(ctx, today.AddDays(l.cfg.LeadDays)); err != nil {
			slog.Error("reminder loop: ensure occurrences", "error", err)
		}
	}
	l.enqueue(ctx, today)
	l.send(ctx)
	l.reconcile(ctx, today)
	l.maybeCleanup(ctx)
}

// enqueue ставит в outbox напоминания по периодам в окне. Личку не делаем —
// только командный чат (решение №1), поэтому ставим, лишь когда он активен.
func (l *Loop) enqueue(ctx context.Context, today clock.Date) {
	if _, err := l.chats.GetActiveChat(ctx); err != nil {
		if !errors.Is(err, model.ErrTelegramNoTeamChat) {
			slog.Error("reminder loop: get active chat (enqueue)", "error", err)
		}
		return // нет командного чата — некому напоминать
	}

	from := today.AddDays(-l.cfg.OverdueMaxDays)
	to := today.AddDays(l.cfg.LeadDays)
	occs, err := l.src.DueBetween(ctx, from, to)
	if err != nil {
		slog.Error("reminder loop: due between", "error", err)
		return
	}

	items := make([]EnqueueItem, 0, len(occs))
	for _, o := range occs {
		diff := dayDiff(today, o.DueDate) // срок − сегодня
		var kind string
		switch {
		case diff >= 0 && diff <= l.cfg.LeadDays:
			kind = model.NotificationKindDueSoon
		case diff < 0 && -diff <= l.cfg.OverdueMaxDays:
			kind = model.NotificationKindOverdue
		default:
			continue
		}
		items = append(items, EnqueueItem{
			OccurrenceID: o.ID, DueDate: o.DueDate, Kind: kind, Recipient: model.NotificationRecipientTeam,
		})
	}
	if len(items) == 0 {
		return
	}
	if err := l.notif.Enqueue(ctx, items); err != nil {
		slog.Error("reminder loop: enqueue", "error", err)
	}
}

// send отправляет строки, которым пора.
func (l *Loop) send(ctx context.Context) {
	now := l.clk.Now()
	rows, err := l.notif.DueForSend(ctx, now, l.cfg.SendLimit)
	if err != nil {
		slog.Error("reminder loop: due for send", "error", err)
		return
	}
	for _, row := range rows {
		if ctx.Err() != nil {
			return
		}
		l.sendOne(ctx, row)
	}
}

func (l *Loop) sendOne(ctx context.Context, row model.TelegramNotification) {
	now := l.clk.Now()
	today := l.clk.Today()

	// Перечитываем период прямо перед отправкой (§5.4).
	occs, err := l.src.GetByIDs(ctx, []string{row.OccurrenceID})
	if err != nil {
		slog.Error("reminder loop: reread occurrence", "error", err, "occurrence_id", row.OccurrenceID)
		return
	}
	if len(occs) == 0 {
		// Период исчез (жёсткое удаление) — отправлять нечего.
		l.cancel(ctx, row.ID, now)
		return
	}
	occ := occs[0]

	switch {
	case occ.Paid, occ.Ended:
		l.cancel(ctx, row.ID, now)
		return
	case !occ.DueDate.Equal(row.DueDate):
		// Срок перенесён: это напоминание устарело, по новому сроку будет своё.
		l.cancel(ctx, row.ID, now)
		return
	case row.Kind == model.NotificationKindDueSoon && today.After(occ.DueDate):
		// Опоздали с due_soon: по §5.7 не досылаем, overdue поедет отдельной строкой.
		l.cancel(ctx, row.ID, now)
		return
	case row.Kind == model.NotificationKindOverdue && dayDiff(occ.DueDate, today) > l.cfg.OverdueMaxDays:
		// Слишком старая просрочка: overdue шлём только первые OverdueMaxDays дней
		// после срока (§5.7). Ограничение действует и при отправке, а не только при
		// постановке: иначе строка overdue, зависшая из-за «no team chat»/429, после
		// долгого простоя и нового /bind ушла бы пачкой устаревших сообщений.
		l.cancel(ctx, row.ID, now)
		return
	}

	chat, err := l.chats.GetActiveChat(ctx)
	if err != nil {
		if errors.Is(err, model.ErrTelegramNoTeamChat) {
			l.fail(ctx, row.ID, now.Add(15*time.Minute), false, "no team chat", now)
			return
		}
		slog.Error("reminder loop: get active chat (send)", "error", err)
		l.fail(ctx, row.ID, now.Add(time.Minute), false, "get active chat failed", now)
		return
	}

	text := l.renderText(occ, today, row.Kind)
	markup := reminderKeyboard(occ.ID, l.cfg)
	chatID := chat.TGChatID

	l.limiter.wait(ctx, chatID)
	msg, sendErr := l.sendMessage(ctx, chatID, text, markup)

	// Группа превратилась в супергруппу: меняем chat_id и один раз повторяем.
	var apiErr *telegram.APIError
	if errors.As(sendErr, &apiErr) && apiErr.IsMigrate() {
		newID := apiErr.MigrateToChatID
		if uerr := l.chats.UpdateChatID(ctx, chatID, newID); uerr != nil {
			slog.Error("reminder loop: migrate chat id", "error", uerr)
		}
		chatID = newID
		l.limiter.wait(ctx, chatID)
		msg, sendErr = l.sendMessage(ctx, chatID, text, markup)
	}

	if sendErr == nil {
		if err := l.notif.MarkSent(ctx, row.ID, chatID, msg.MessageID, now); err != nil {
			slog.Error("reminder loop: mark sent", "error", err, "id", row.ID)
		}
		return
	}
	l.handleSendError(ctx, row, chatID, sendErr, now)
}

func (l *Loop) sendMessage(ctx context.Context, chatID int64, text string, markup *telegram.InlineKeyboardMarkup) (telegram.Message, error) {
	cctx, cancel := context.WithTimeout(ctx, l.cfg.SendTimeout)
	defer cancel()
	return l.sender.SendMessage(cctx, telegram.SendMessageParams{
		ChatID: chatID, Text: text, ParseMode: "HTML", ReplyMarkup: markup,
	})
}

// handleSendError раскладывает ошибку отправки по таблице §5.4.
func (l *Loop) handleSendError(ctx context.Context, row model.TelegramNotification, chatID int64, sendErr error, now time.Time) {
	var apiErr *telegram.APIError
	if errors.As(sendErr, &apiErr) {
		switch {
		case apiErr.IsRetryAfter():
			d := time.Duration(apiErr.RetryAfter) * time.Second
			if d <= 0 {
				d = time.Second
			}
			l.fail(ctx, row.ID, now.Add(d), false, "flood control 429", now)
			l.limiter.penalize(chatID, now.Add(d))
		case apiErr.IsChatNotFound() || apiErr.IsBotBlocked():
			// Командный чат недоступен: деактивируем, ждём нового /bind (§5.4).
			if err := l.chats.DeactivateChat(ctx, chatID, apiErr.Description); err != nil &&
				!errors.Is(err, model.ErrTelegramNoTeamChat) {
				slog.Error("reminder loop: deactivate chat", "error", err)
			}
			slog.Error("reminder loop: team chat unavailable", "chat_id", chatID, "desc", apiErr.Description)
			l.fail(ctx, row.ID, now.Add(15*time.Minute), false, apiErr.Description, now)
		case apiErr.Code >= 500 || apiErr.Code == 0:
			l.backoff(ctx, row, apiErr.Error(), now)
		default: // прочие 400
			l.dead(ctx, row.ID, apiErr.Error(), now)
		}
		return
	}
	// Сеть/таймаут — как 5xx.
	l.backoff(ctx, row, sendErr.Error(), now)
}

// backoff — повтор 30s·2^attempts, не больше часа; после maxSendAttempts — dead.
func (l *Loop) backoff(ctx context.Context, row model.TelegramNotification, lastErr string, now time.Time) {
	if row.Attempts+1 >= maxSendAttempts {
		l.dead(ctx, row.ID, lastErr, now)
		return
	}
	d := 30 * time.Second << uint(row.Attempts)
	if d > time.Hour || d <= 0 {
		d = time.Hour
	}
	l.fail(ctx, row.ID, now.Add(d), true, lastErr, now)
}

func (l *Loop) cancel(ctx context.Context, id string, now time.Time) {
	if err := l.notif.Cancel(ctx, id, now); err != nil {
		slog.Error("reminder loop: cancel", "error", err, "id", id)
	}
}

func (l *Loop) fail(ctx context.Context, id string, next time.Time, incr bool, lastErr string, now time.Time) {
	if err := l.notif.MarkFailed(ctx, id, next, incr, lastErr, now); err != nil {
		slog.Error("reminder loop: mark failed", "error", err, "id", id)
	}
}

func (l *Loop) dead(ctx context.Context, id, lastErr string, now time.Time) {
	if err := l.notif.MarkDead(ctx, id, lastErr, now); err != nil {
		slog.Error("reminder loop: mark dead", "error", err, "id", id)
	}
}

// reconcile правит уже отправленные сообщения при смене статуса периода (§5.6).
func (l *Loop) reconcile(ctx context.Context, today clock.Date) {
	rows, err := l.notif.OpenForReconcile(ctx, today.AddDays(-7), l.cfg.ReconcileLimit)
	if err != nil {
		slog.Error("reminder loop: open for reconcile", "error", err)
		return
	}
	if len(rows) == 0 {
		return
	}

	seen := map[string]struct{}{}
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		if _, ok := seen[r.OccurrenceID]; ok {
			continue
		}
		seen[r.OccurrenceID] = struct{}{}
		ids = append(ids, r.OccurrenceID)
	}
	occs, err := l.src.GetByIDs(ctx, ids)
	if err != nil {
		slog.Error("reminder loop: reconcile get by ids", "error", err)
		return
	}
	byID := make(map[string]model.ReminderOccurrence, len(occs))
	for _, o := range occs {
		byID[o.ID] = o
	}

	now := l.clk.Now()
	for _, row := range rows {
		occ, ok := byID[row.OccurrenceID]
		if !ok {
			continue
		}
		switch {
		case occ.Paid && row.ShownStatus != model.ShownStatusPaid:
			l.editShown(ctx, row, paidText(occ, ""), emptyMarkup(), model.ShownStatusPaid, now)
		case occ.Ended && row.ShownStatus != model.ShownStatusEnded:
			l.editShown(ctx, row, endedText(occ), emptyMarkup(), model.ShownStatusEnded, now)
		case !occ.Paid && !occ.Ended && row.ShownStatus == model.ShownStatusPaid:
			// B снял отметку об оплате — возвращаем кнопку (§5.6).
			l.editShown(ctx, row, l.renderText(occ, today, row.Kind), reminderKeyboard(occ.ID, l.cfg), model.ShownStatusOpen, now)
		}
	}
}

func (l *Loop) editShown(ctx context.Context, row model.TelegramNotification, text string, markup *telegram.InlineKeyboardMarkup, shown string, now time.Time) {
	if row.ChatID == nil || row.MessageID == nil {
		return
	}
	// Правки тоже идут через лимитер: иначе пачка reconcile в один групповой чат
	// легко упирается в 429 (§5.4, §5.6).
	l.limiter.wait(ctx, *row.ChatID)
	cctx, cancel := context.WithTimeout(ctx, l.cfg.SendTimeout)
	_, err := l.sender.EditMessageText(cctx, telegram.EditMessageTextParams{
		ChatID: *row.ChatID, MessageID: *row.MessageID, Text: text, ParseMode: "HTML", ReplyMarkup: markup,
	})
	cancel()
	if err != nil {
		var apiErr *telegram.APIError
		switch {
		case errors.As(err, &apiErr) && apiErr.IsMessageNotModified():
			// нечего менять — всё равно фиксируем показанный статус
		case errors.As(err, &apiErr) && apiErr.IsEditUnrecoverable():
			// Сообщение больше не отредактировать (удалено админом, недоступно,
			// чат потерян). Всё равно фиксируем целевой shown_status, иначе строка
			// вечно всплывает в OpenForReconcile (сорт по updated_at) и забивает
			// очередь, повторяя editMessageText тысячи раз в сутки (§5.6).
			slog.Warn("reminder loop: edit unrecoverable, fixing shown status anyway",
				"id", row.ID, "desc", apiErr.Description)
		default:
			slog.Error("reminder loop: edit message", "error", err, "id", row.ID)
			return
		}
	}
	if err := l.notif.SetShownStatus(ctx, row.ID, shown, now); err != nil {
		slog.Error("reminder loop: set shown status", "error", err, "id", row.ID)
	}
}

// renderText собирает текст напоминания и в режиме демо-заглушки помечает его
// префиксом [DEMO] (решение №7, риск §11).
func (l *Loop) renderText(occ model.ReminderOccurrence, today clock.Date, kind string) string {
	text := reminderText(occ, today, kind)
	if l.cfg.Demo {
		return demoPrefix + text
	}
	return text
}

func (l *Loop) maybeCleanup(ctx context.Context) {
	now := l.clk.Now()
	if !l.lastCleanup.IsZero() && now.Sub(l.lastCleanup) < l.cfg.CleanupEvery {
		return
	}
	l.lastCleanup = now
	// Удаляем давно завершённые строки outbox (dead/cancelled старше 30 дней).
	if err := l.notif.Cleanup(ctx, now.AddDate(0, 0, -30)); err != nil {
		slog.Error("reminder loop: cleanup", "error", err)
	}
	// Чистим ядро бота: апдейты старше 7 дней и коды привязки старше суток (§5.3 шаг 5).
	if err := l.chats.CleanupStale(ctx, now.AddDate(0, 0, -7), now.AddDate(0, 0, -1)); err != nil {
		slog.Error("reminder loop: cleanup stale", "error", err)
	}
}

// emptyMarkup — пустая клавиатура: editMessageText с ней убирает кнопки.
func emptyMarkup() *telegram.InlineKeyboardMarkup {
	return &telegram.InlineKeyboardMarkup{InlineKeyboard: [][]telegram.InlineKeyboardButton{}}
}

// rateLimiter — глобальная пауза между любыми сообщениями и пауза на чат (§5.4).
// Нулевые/отрицательные интервалы отключают лимит (используется в тестах).
type rateLimiter struct {
	mu         sync.Mutex
	perChat    time.Duration
	global     time.Duration
	last       map[int64]time.Time
	lastGlobal time.Time
}

func (r *rateLimiter) wait(ctx context.Context, chatID int64) {
	r.mu.Lock()
	now := time.Now()
	var sleep time.Duration
	if r.global > 0 {
		if gap := r.global - now.Sub(r.lastGlobal); gap > sleep {
			sleep = gap
		}
	}
	if r.perChat > 0 {
		if t, ok := r.last[chatID]; ok {
			if gap := r.perChat - now.Sub(t); gap > sleep {
				sleep = gap
			}
		}
	}
	fire := now.Add(sleep)
	r.lastGlobal = fire
	r.last[chatID] = fire
	r.mu.Unlock()

	if sleep <= 0 {
		return
	}
	t := time.NewTimer(sleep)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

// penalize притормаживает чат до указанного момента после 429.
func (r *rateLimiter) penalize(chatID int64, until time.Time) {
	r.mu.Lock()
	r.last[chatID] = until
	r.mu.Unlock()
}
