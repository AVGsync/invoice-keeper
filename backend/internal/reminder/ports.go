// Package reminder — движок напоминаний об оплате: окно due−N дней, outbox с
// доставкой at-least-once, правка отправленных сообщений и кнопка «Оплачено».
// По решению пользователя №1 напоминания уходят только в командный чат; личку
// ответственного не делаем. Интерфейсы объявляет потребитель (бот), реализации
// данных B лежат за портами OccurrenceSource/Payer (docs/TELEGRAM_BOT.md §2.2, §5).
package reminder

import (
	"context"
	"time"

	"github.com/AVGsync/invoice-keeper/backend/internal/clock"
	"github.com/AVGsync/invoice-keeper/backend/internal/infrastructure/telegram"
	"github.com/AVGsync/invoice-keeper/backend/internal/model"
)

// OccurrenceSource читает периоды модуля B. Реализации: in-memory заглушка
// (MemoryStore) и SQL-адаптер postgres.TelegramSourceRepository.
type OccurrenceSource interface {
	// DueBetween возвращает неоплаченные периоды активных платежей с due_date в
	// [from, to] включительно (даты в APP_TIMEZONE). Нужен диапазон, включающий
	// и недавнюю просрочку: движок зовёт DueBetween(today−overdueMax, today+lead).
	DueBetween(ctx context.Context, from, to clock.Date) ([]model.ReminderOccurrence, error)
	// GetByIDs возвращает текущее состояние периодов, включая оплаченные и
	// завершённые — для перепроверки перед отправкой и правки сообщений.
	GetByIDs(ctx context.Context, ids []string) ([]model.ReminderOccurrence, error)
}

// Payer пишет в домен B. Реализует сервис B (адаптер в apiserver/telegram.go) и
// заглушка MemoryStore. Политику «кто может отметить» (ответственный или админ,
// решение пользователя №4) проверяет сама реализация.
type Payer interface {
	// MarkPaid идемпотентен. Ошибки: ErrOccurrenceNotFound, ErrAlreadyPaid,
	// ErrPaymentEnded, ErrForbidden.
	MarkPaid(ctx context.Context, occurrenceID string, paidDate clock.Date, actorUserID string) error
}

// OccurrenceEnsurer — необязательный порт: если B создаёт периоды лениво, движок
// просит гарантировать строки до горизонта перед выборкой (§2.2, [В-B 2]).
type OccurrenceEnsurer interface {
	EnsureOccurrencesUntil(ctx context.Context, until clock.Date) error
}

// EnqueueItem — строка для постановки в outbox.
type EnqueueItem struct {
	OccurrenceID string
	DueDate      clock.Date
	Kind         string
	Recipient    string
}

// NotificationStore — outbox напоминаний (таблица telegram_notifications).
// Реализация — postgres.TelegramNotificationRepository; в тестах — in-memory.
type NotificationStore interface {
	// Enqueue ставит строки с дедупом по (occurrence_id, due_date, kind,
	// recipient); строку в состоянии cancelled возвращает в pending (§5.3).
	Enqueue(ctx context.Context, items []EnqueueItem) error
	// DueForSend возвращает строки pending/failed, которым пора (next_attempt_at
	// <= now), в порядке next_attempt_at, не больше limit.
	DueForSend(ctx context.Context, now time.Time, limit int) ([]model.TelegramNotification, error)
	MarkSent(ctx context.Context, id string, chatID, messageID int64, now time.Time) error
	// MarkFailed переводит строку в failed с next_attempt_at; incrementAttempt=false
	// для 429 (retry_after не считается попыткой, §5.4).
	MarkFailed(ctx context.Context, id string, nextAttemptAt time.Time, incrementAttempt bool, lastErr string, now time.Time) error
	MarkDead(ctx context.Context, id, lastErr string, now time.Time) error
	Cancel(ctx context.Context, id string, now time.Time) error
	// OpenForReconcile возвращает отправленные открытые/оплаченные сообщения со
	// сроком не раньше dueOnOrAfter — кандидаты на правку (§5.6).
	OpenForReconcile(ctx context.Context, dueOnOrAfter clock.Date, limit int) ([]model.TelegramNotification, error)
	// OpenByOccurrence возвращает отправленные (shown_status open|paid) строки
	// периода — для правки по кнопке «Оплачено» (§6.3).
	OpenByOccurrence(ctx context.Context, occurrenceID string) ([]model.TelegramNotification, error)
	SetShownStatus(ctx context.Context, id, shownStatus string, now time.Time) error
	// ResetFailedByRecipient сбрасывает next_attempt_at=now у failed строк
	// адресата — например, после /bind нового командного чата (§5.4).
	ResetFailedByRecipient(ctx context.Context, recipient string, now time.Time) error
	// Cleanup удаляет мусор outbox (мёртвые/отменённые строки старше ttl).
	Cleanup(ctx context.Context, before time.Time) error
}

// TeamChatStore отдаёт активный командный чат и правит его при migrate/удалении.
// Реализует postgres.TelegramRepository.
type TeamChatStore interface {
	GetActiveChat(ctx context.Context) (model.TelegramChat, error) // model.ErrTelegramNoTeamChat, если нет
	DeactivateChat(ctx context.Context, tgChatID int64, reason string) error
	UpdateChatID(ctx context.Context, oldID, newID int64) error
	// CleanupStale удаляет обработанные апдейты и истёкшие коды привязки (§5.3 шаг 5).
	CleanupStale(ctx context.Context, updatesBefore, codesBefore time.Time) error
}

// LinkResolver находит привязку по Telegram id — для авторизации кнопки
// «Оплачено» по from.id (§6.3). Реализует postgres.TelegramRepository.
type LinkResolver interface {
	GetLinkByTGUserID(ctx context.Context, tgUserID int64) (model.TelegramLink, error)
}

// BotSender — часть клиента Bot API, нужная движку. Реализуют *telegram.Client
// и *telegram.FakeClient.
type BotSender interface {
	SendMessage(ctx context.Context, p telegram.SendMessageParams) (telegram.Message, error)
	EditMessageText(ctx context.Context, p telegram.EditMessageTextParams) (telegram.Message, error)
}
