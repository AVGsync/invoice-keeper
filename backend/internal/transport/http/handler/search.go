package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/AVGsync/invoice-keeper/backend/internal/clock"
	"github.com/AVGsync/invoice-keeper/backend/internal/model"
	"github.com/AVGsync/invoice-keeper/backend/internal/model/request"
	"github.com/AVGsync/invoice-keeper/backend/internal/model/response"
	"github.com/AVGsync/invoice-keeper/backend/internal/service"
	"github.com/AVGsync/invoice-keeper/backend/internal/transport/http/middleware"
)

// SearchUseCase — серверный поиск платежей.
type SearchUseCase interface {
	SearchOccurrences(ctx context.Context, f model.OccurrenceFilter) ([]model.Payment, int, error)
	SearchRecurring(ctx context.Context, f model.RecurringFilter) ([]model.RecurringSearchRow, int, error)
}

type SearchHandler struct {
	svc SearchUseCase
	clk clock.Clock
}

func NewSearchHandler(svc SearchUseCase, clk clock.Clock) *SearchHandler {
	return &SearchHandler{svc: svc, clk: clk}
}

// SearchOccurrences godoc
// @Summary      Поиск периодов платежей
// @Description  Серверный поиск с фильтрами, сортировкой и пагинацией. Права как у GET /payment-occurrences.
// @Description  amount_min/amount_max — в минимальных единицах и только вместе с ровно одной currency.
// @Tags         Payments
// @Security     BearerAuth
// @Produce      json
// @Param        q query string false "Подстрока описания или ФИО ответственного"
// @Param        status query []string false "Статусы" collectionFormat(csv) Enums(overdue,pending,paid,ended)
// @Param        currency query []string false "Валюты" collectionFormat(csv) Enums(RUB,USD,EUR)
// @Param        assignee query string false "me — только периоды текущего пользователя" Enums(me)
// @Param        assignee_id query []string false "UUID ответственных (взаимоисключимо с assignee=me)" collectionFormat(csv)
// @Param        persona_id query []string false "UUID персон ответственного" collectionFormat(csv)
// @Param        recurrent_payment_id query string false "UUID шаблона"
// @Param        due_from query string false "Нижняя граница due_date (YYYY-MM-DD)"
// @Param        due_to query string false "Верхняя граница due_date (YYYY-MM-DD)"
// @Param        paid_from query string false "Нижняя граница paid_date (YYYY-MM-DD)"
// @Param        paid_to query string false "Верхняя граница paid_date (YYYY-MM-DD)"
// @Param        amount_min query integer false "Минимальная сумма (минимальные единицы, требует ровно одну currency)"
// @Param        amount_max query integer false "Максимальная сумма (минимальные единицы, требует ровно одну currency)"
// @Param        sort query string false "Поле сортировки" Enums(due_date,amount,created_at,description,paid_date)
// @Param        order query string false "Направление" Enums(asc,desc)
// @Param        limit query integer false "1..200, по умолчанию 50"
// @Param        offset query integer false "0..100000, по умолчанию 0"
// @Success      200 {object} response.PaymentSearchResponse
// @Failure      400 {string} string "Некорректный параметр (текст ошибки)"
// @Failure      403 {string} string "Не удалось получить данные авторизации"
// @Failure      500 {string} string "Не удалось выполнить поиск"
// @Router       /payment-occurrences/search [get]
func (h *SearchHandler) SearchOccurrences() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := middleware.ClaimsFromContext(r.Context())
		if !ok {
			http.Error(w, "failed get claims", http.StatusForbidden)
			return
		}

		filter, err := request.ParseOccurrenceSearch(r.URL.Query(), claims.UserID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		payments, total, err := h.svc.SearchOccurrences(r.Context(), filter)
		if err != nil {
			http.Error(w, "failed to search payments", http.StatusInternalServerError)
			return
		}

		today := h.clk.Today()
		items := make([]response.PaymentResponse, 0, len(payments))
		for _, p := range payments {
			status, paidLate := service.EffectiveStatus(p, today)
			items = append(items, response.NewPaymentResponse(p, status, paidLate))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response.PaymentSearchResponse{
			Items:  items,
			Total:  total,
			Limit:  filter.Limit,
			Offset: filter.Offset,
		})
	}
}

// SearchRecurring godoc
// @Summary      Поиск регулярных платежей
// @Description  Серверный поиск шаблонов с фильтрами, сортировкой и пагинацией. Права как у GET /recurring-payments.
// @Description  amount_min/amount_max — в минимальных единицах и только вместе с ровно одной currency.
// @Tags         Recurring Payments
// @Security     BearerAuth
// @Produce      json
// @Param        q query string false "Подстрока описания или ФИО ответственного"
// @Param        currency query []string false "Валюты" collectionFormat(csv) Enums(RUB,USD,EUR)
// @Param        frequency query []string false "Периодичность" collectionFormat(csv) Enums(weekly,monthly)
// @Param        assignee query string false "me — только шаблоны текущего пользователя" Enums(me)
// @Param        assignee_id query []string false "UUID ответственных (взаимоисключимо с assignee=me)" collectionFormat(csv)
// @Param        persona_id query []string false "UUID персон ответственного" collectionFormat(csv)
// @Param        status query []string false "Статус шаблона" collectionFormat(csv) Enums(active,stopped)
// @Param        include_stopped query boolean false "true ≡ status=active,stopped; по умолчанию только активные"
// @Param        amount_min query integer false "Минимальная сумма (минимальные единицы, требует ровно одну currency)"
// @Param        amount_max query integer false "Максимальная сумма (минимальные единицы, требует ровно одну currency)"
// @Param        start_from query string false "Нижняя граница start_date (YYYY-MM-DD)"
// @Param        start_to query string false "Верхняя граница start_date (YYYY-MM-DD)"
// @Param        next_due_from query string false "Нижняя граница ближайшего срока (YYYY-MM-DD)"
// @Param        next_due_to query string false "Верхняя граница ближайшего срока (YYYY-MM-DD)"
// @Param        sort query string false "Поле сортировки" Enums(description,amount,created_at,next_due,start_date)
// @Param        order query string false "Направление" Enums(asc,desc)
// @Param        limit query integer false "1..200, по умолчанию 50"
// @Param        offset query integer false "0..100000, по умолчанию 0"
// @Success      200 {object} response.RecurringPaymentSearchResponse
// @Failure      400 {string} string "Некорректный параметр (текст ошибки)"
// @Failure      403 {string} string "Не удалось получить данные авторизации"
// @Failure      500 {string} string "Не удалось выполнить поиск"
// @Router       /recurring-payments/search [get]
func (h *SearchHandler) SearchRecurring() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := middleware.ClaimsFromContext(r.Context())
		if !ok {
			http.Error(w, "failed get claims", http.StatusForbidden)
			return
		}

		filter, err := request.ParseRecurringSearch(r.URL.Query(), claims.UserID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		rows, total, err := h.svc.SearchRecurring(r.Context(), filter)
		if err != nil {
			http.Error(w, "failed to search recurring payments", http.StatusInternalServerError)
			return
		}

		items := make([]response.RecurringPaymentItem, 0, len(rows))
		for _, row := range rows {
			items = append(items, response.NewRecurringPaymentItem(row))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response.RecurringPaymentSearchResponse{
			Items:  items,
			Total:  total,
			Limit:  filter.Limit,
			Offset: filter.Offset,
		})
	}
}
