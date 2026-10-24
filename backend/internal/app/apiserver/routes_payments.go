package apiserver

import (
	"github.com/AVGsync/invoice-keeper/backend/internal/transport/http/handler"
	"github.com/AVGsync/invoice-keeper/backend/internal/transport/http/middleware"

	"github.com/go-chi/chi/v5"
)

// mountPayments монтирует модуль регулярных платежей и их периодов (владелец —
// второй бэкендер). Вызывается внутри защищённой группы (Auth, LimitByUser,
// IsUserOrAdmin).
func (s *APIServer) mountPayments(r chi.Router, mw *middleware.Middleware, paymentHandler *handler.PaymentHandler, searchHandler *handler.SearchHandler) {
	r.Route("/payments", func(r chi.Router) {
		r.Use(mw.MaxBytes(defaultMaxRequestBody))

		r.Get("/summary", paymentHandler.GetPaymentsSummary())
	})

	// Шаблоны регулярных платежей: чтение — любой авторизованный (команда),
	// запись (создание/изменение/остановка) — только администратор (D10).
	r.Route("/recurring-payments", func(r chi.Router) {
		r.Use(mw.MaxBytes(defaultMaxRequestBody))

		// Статический /search приоритетнее /{id}; права как у списка.
		r.Get("/search", searchHandler.SearchRecurring())
		r.Get("/", paymentHandler.GetRecurringPayments())

		r.Group(func(r chi.Router) {
			r.Use(mw.IsAdmin)
			r.Post("/", paymentHandler.CreateRecurringPayment())
			r.Patch("/{id}", paymentHandler.UpdateRecurringPayment())
			r.Delete("/{id}", paymentHandler.StopRecurringPayment())
		})
	})

	r.Route("/payment-occurrences", func(r chi.Router) {
		r.Use(mw.MaxBytes(defaultMaxRequestBody))

		// Статический /search приоритетнее /{id}; права как у списка.
		r.Get("/search", searchHandler.SearchOccurrences())
		r.Get("/", paymentHandler.GetPaymentOccurrences())
		r.Get("/{id}", paymentHandler.GetPaymentOccurrence())
		r.Patch("/{id}", paymentHandler.PatchPaymentOccurrence())
	})
}

// mountNotifications монтирует настройки email-уведомлений текущего пользователя.
func (s *APIServer) mountNotifications(r chi.Router, mw *middleware.Middleware, notificationsHandler *handler.NotificationsHandler) {
	r.Route("/notifications/settings", func(r chi.Router) {
		r.Use(mw.MaxBytes(defaultMaxRequestBody))

		r.Get("/", notificationsHandler.GetSettings())
		r.Put("/", notificationsHandler.UpdateSettings())
		r.Patch("/", notificationsHandler.PatchSettings())
		r.Post("/", notificationsHandler.CreateSettings())
		r.Delete("/", notificationsHandler.DeleteSettings())
	})
}
