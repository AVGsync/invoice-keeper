package service

import (
	"context"
	"log/slog"

	"github.com/AVGsync/invoice-keeper/backend/internal/clock"
	"github.com/AVGsync/invoice-keeper/backend/internal/model"
)

// Серверный поиск платежей (трек B, пункт 4). Сервис только прокидывает фильтр в
// репозиторий, подставляя «сегодня» из общих часов, чтобы статусы overdue/pending
// и ближайший срок next_due совпадали с остальной системой.

// PaymentSearcher — поиск периодов в репозитории.
type PaymentSearcher interface {
	SearchPayments(ctx context.Context, f model.OccurrenceFilter, today string) ([]model.Payment, int, error)
}

// RecurringSearcher — поиск шаблонов в репозитории.
type RecurringSearcher interface {
	SearchRecurrentPayments(ctx context.Context, f model.RecurringFilter, today string) ([]model.RecurringSearchRow, int, error)
}

type PaymentSearchService struct {
	pay PaymentSearcher
	rec RecurringSearcher
	clk clock.Clock
}

func NewPaymentSearchService(pay PaymentSearcher, rec RecurringSearcher, clk clock.Clock) *PaymentSearchService {
	return &PaymentSearchService{pay: pay, rec: rec, clk: clk}
}

// SearchOccurrences возвращает страницу периодов и общее число по фильтру.
func (s *PaymentSearchService) SearchOccurrences(
	ctx context.Context,
	f model.OccurrenceFilter,
) ([]model.Payment, int, error) {
	payments, total, err := s.pay.SearchPayments(ctx, f, s.clk.Today().String())
	if err != nil {
		slog.Error("payment search service: search occurrences failed", "error", err)
		return nil, 0, err
	}
	return payments, total, nil
}

// SearchRecurring возвращает страницу шаблонов и общее число по фильтру.
func (s *PaymentSearchService) SearchRecurring(
	ctx context.Context,
	f model.RecurringFilter,
) ([]model.RecurringSearchRow, int, error) {
	rows, total, err := s.rec.SearchRecurrentPayments(ctx, f, s.clk.Today().String())
	if err != nil {
		slog.Error("payment search service: search recurring failed", "error", err)
		return nil, 0, err
	}
	return rows, total, nil
}
