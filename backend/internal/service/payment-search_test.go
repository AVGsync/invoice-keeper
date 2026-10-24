package service

import (
	"context"
	"testing"
	"time"

	"github.com/AVGsync/invoice-keeper/backend/internal/clock"
	"github.com/AVGsync/invoice-keeper/backend/internal/model"
)

type stubSearcher struct {
	gotToday   string
	payments   []model.Payment
	recurring  []model.RecurringSearchRow
	total      int
	totalRec   int
	gotOccFilt model.OccurrenceFilter
	gotRecFilt model.RecurringFilter
}

func (s *stubSearcher) SearchPayments(_ context.Context, f model.OccurrenceFilter, today string) ([]model.Payment, int, error) {
	s.gotToday = today
	s.gotOccFilt = f
	return s.payments, s.total, nil
}

func (s *stubSearcher) SearchRecurrentPayments(_ context.Context, f model.RecurringFilter, today string) ([]model.RecurringSearchRow, int, error) {
	s.gotToday = today
	s.gotRecFilt = f
	return s.recurring, s.totalRec, nil
}

func TestPaymentSearchServicePassesToday(t *testing.T) {
	clk := clock.NewFixed(time.Date(2026, 3, 9, 10, 0, 0, 0, time.UTC), time.UTC)
	stub := &stubSearcher{payments: []model.Payment{{ID: "p1"}}, total: 1}
	svc := NewPaymentSearchService(stub, stub, clk)

	got, total, err := svc.SearchOccurrences(context.Background(), model.OccurrenceFilter{Limit: 50})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stub.gotToday != "2026-03-09" {
		t.Fatalf("today passed to repo: got %q", stub.gotToday)
	}
	if total != 1 || len(got) != 1 {
		t.Fatalf("passthrough: got %d items total %d", len(got), total)
	}

	stub.gotToday = ""
	if _, _, err := svc.SearchRecurring(context.Background(), model.RecurringFilter{Limit: 50}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stub.gotToday != "2026-03-09" {
		t.Fatalf("today passed to recurring repo: got %q", stub.gotToday)
	}
}
