package reminder

import "errors"

// Sentinel-ошибки домена оплаты. Их возвращает порт Payer: и in-memory заглушка
// (stub.go), и будущий адаптер к сервису B (apiserver/telegram.go маппит ошибки
// B в эти). Движок и бот решают по ним, что показать пользователю (§2.2, §6.3).
var (
	// ErrOccurrenceNotFound — периода с таким id нет.
	ErrOccurrenceNotFound = errors.New("reminder: occurrence not found")
	// ErrAlreadyPaid — период уже отмечен оплаченным.
	ErrAlreadyPaid = errors.New("reminder: occurrence already paid")
	// ErrPaymentEnded — платёж завершён или удалён, отметить нельзя.
	ErrPaymentEnded = errors.New("reminder: payment ended")
	// ErrForbidden — у пользователя нет прав отметить оплату (не ответственный и не админ).
	ErrForbidden = errors.New("reminder: forbidden")
)
