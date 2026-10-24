package apiserver

import (
	"github.com/AVGsync/invoice-keeper/backend/internal/transport/http/handler"
	"github.com/AVGsync/invoice-keeper/backend/internal/transport/http/middleware"

	"github.com/go-chi/chi/v5"
)

// mountPayments монтирует модуль регулярных платежей и их периодов (владелец — второй бэкендер).
func (s *APIServer) mountPayments(r chi.Router, mw *middleware.Middleware, paymentHandler *handler.PaymentHandler, searchHandler *handler.SearchHandler, receiptHandler *handler.ReceiptHandler) {
	r.Route("/payments", func(r chi.Router) {
		r.Use(mw.MaxBytes(defaultMaxRequestBody))

		r.Get("/summary", paymentHandler.GetPaymentsSummary())
	})

	r.Route("/recurring-payments", func(r chi.Router) {
		r.Use(mw.MaxBytes(defaultMaxRequestBody))

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
		r.Group(func(r chi.Router) {
			r.Use(mw.MaxBytes(defaultMaxRequestBody))

			r.Get("/search", searchHandler.SearchOccurrences())
			r.Get("/", paymentHandler.GetPaymentOccurrences())
			r.Get("/{id}", paymentHandler.GetPaymentOccurrence())
			r.Patch("/{id}", paymentHandler.PatchPaymentOccurrence())

			r.Get("/{id}/receipts", receiptHandler.List())
			r.Post("/{id}/receipts/link", receiptHandler.AddLink())
		})

		// Для загрузки файла лимит тела больше: размер файла плюс запас на служебные поля.
		r.With(mw.MaxBytes(int64(s.config.ReceiptsMaxMB)<<20+defaultMaxRequestBody)).
			Post("/{id}/receipts", receiptHandler.Upload())
	})

	r.Route("/receipts", func(r chi.Router) {
		r.Use(mw.MaxBytes(defaultMaxRequestBody))

		r.Get("/{id}/access", receiptHandler.Access())
		r.Delete("/{id}", receiptHandler.Delete())
	})
}

// mountNotifications монтирует настройки email-уведомлений текущего пользователя.
func (s *APIServer) mountNotifications(r chi.Router, mw *middleware.Middleware, notificationsHandler *handler.NotificationsHandler, testMailHandler *handler.TestMailHandler) {
	r.With(mw.IsAdmin, mw.MaxBytes(defaultMaxRequestBody)).Post("/notifications/test-email", testMailHandler.Send())

	r.Route("/notifications/settings", func(r chi.Router) {
		r.Use(mw.MaxBytes(defaultMaxRequestBody))

		r.Get("/", notificationsHandler.GetSettings())
		r.Put("/", notificationsHandler.UpdateSettings())
		r.Patch("/", notificationsHandler.PatchSettings())
		r.Post("/", notificationsHandler.CreateSettings())
		r.Delete("/", notificationsHandler.DeleteSettings())
	})
}
