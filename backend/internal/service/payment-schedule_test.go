package service

import (
	"testing"

	"github.com/AVGsync/invoice-keeper/backend/internal/clock"
	"github.com/AVGsync/invoice-keeper/backend/internal/model"
)

func datesToStrings(ds []clock.Date) []string {
	out := make([]string, 0, len(ds))
	for _, d := range ds {
		out = append(out, d.String())
	}
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestDueDatesMonthlyClampsToLastDay(t *testing.T) {
	rp := model.RecurrentPayment{Frequency: "monthly", Interval: 1, StartDate: "2025-01-31"}
	after, _ := clock.ParseDate("2024-12-31")
	until, _ := clock.ParseDate("2025-03-31")
	got := datesToStrings(DueDates(rp, after, until))
	want := []string{"2025-01-31", "2025-02-28", "2025-03-31"}
	if !equalStrings(got, want) {
		t.Errorf("monthly clamp: got %v want %v", got, want)
	}
}

func TestDueDatesMonthlyLeapFebruary(t *testing.T) {
	rp := model.RecurrentPayment{Frequency: "monthly", Interval: 1, StartDate: "2024-01-31"}
	after, _ := clock.ParseDate("2023-12-31")
	until, _ := clock.ParseDate("2024-03-31")
	got := datesToStrings(DueDates(rp, after, until))
	want := []string{"2024-01-31", "2024-02-29", "2024-03-31"}
	if !equalStrings(got, want) {
		t.Errorf("leap february: got %v want %v", got, want)
	}
}

func TestDueDatesMonthlyInterval2(t *testing.T) {
	rp := model.RecurrentPayment{Frequency: "monthly", Interval: 2, StartDate: "2025-01-15"}
	after, _ := clock.ParseDate("2024-12-31")
	until, _ := clock.ParseDate("2025-06-30")
	got := datesToStrings(DueDates(rp, after, until))
	want := []string{"2025-01-15", "2025-03-15", "2025-05-15"}
	if !equalStrings(got, want) {
		t.Errorf("interval 2: got %v want %v", got, want)
	}
}

func TestDueDatesWeekly(t *testing.T) {
	rp := model.RecurrentPayment{Frequency: "weekly", Interval: 2, StartDate: "2025-01-01"}
	after, _ := clock.ParseDate("2024-12-31")
	until, _ := clock.ParseDate("2025-02-01")
	got := datesToStrings(DueDates(rp, after, until))
	want := []string{"2025-01-01", "2025-01-15", "2025-01-29"}
	if !equalStrings(got, want) {
		t.Errorf("weekly: got %v want %v", got, want)
	}
}

func TestDueDatesHorizonBoundaryInclusive(t *testing.T) {
	rp := model.RecurrentPayment{Frequency: "monthly", Interval: 1, StartDate: "2025-01-10"}
	after, _ := clock.ParseDate("2025-01-09")
	until, _ := clock.ParseDate("2025-02-10")
	got := datesToStrings(DueDates(rp, after, until))
	want := []string{"2025-01-10", "2025-02-10"} // until включительно
	if !equalStrings(got, want) {
		t.Errorf("horizon boundary: got %v want %v", got, want)
	}
}

func TestDueDatesAfterExcludesEarlier(t *testing.T) {
	rp := model.RecurrentPayment{Frequency: "monthly", Interval: 1, StartDate: "2025-01-10"}
	after, _ := clock.ParseDate("2025-01-10") // строго после 10 янв
	until, _ := clock.ParseDate("2025-03-10")
	got := datesToStrings(DueDates(rp, after, until))
	want := []string{"2025-02-10", "2025-03-10"}
	if !equalStrings(got, want) {
		t.Errorf("after exclusive: got %v want %v", got, want)
	}
}

func ptrStr(s string) *string { return &s }

func TestEffectiveStatus(t *testing.T) {
	today := clock.Date{Y: 2026, M: 10, D: 24}
	cases := []struct {
		name         string
		p            model.Payment
		wantStatus   string
		wantPaidLate bool
	}{
		{"pending", model.Payment{DueDate: "2026-10-25"}, "pending", false},
		{"pending today", model.Payment{DueDate: "2026-10-24"}, "pending", false},
		{"overdue", model.Payment{DueDate: "2026-10-23"}, "overdue", false},
		{"paid on time", model.Payment{DueDate: "2026-10-24", PaidDate: ptrStr("2026-10-24")}, "paid", false},
		{"paid late", model.Payment{DueDate: "2026-10-20", PaidDate: ptrStr("2026-10-24")}, "paid", true},
		{"paid early", model.Payment{DueDate: "2026-10-30", PaidDate: ptrStr("2026-10-24")}, "paid", false},
		// Остановленный/удалённый шаблон (Ended): неоплаченный прошедший период —
		// "ended", не "overdue" (NL_QUERIES.md §6.4 п.5); оплата остаётся "paid".
		{"ended unpaid past", model.Payment{DueDate: "2026-10-23", Ended: true}, model.ShownStatusEnded, false},
		{"ended unpaid future", model.Payment{DueDate: "2026-10-30", Ended: true}, model.ShownStatusEnded, false},
		{"ended but paid", model.Payment{DueDate: "2026-10-23", PaidDate: ptrStr("2026-10-23"), Ended: true}, "paid", false},
	}
	for _, c := range cases {
		status, paidLate := EffectiveStatus(c.p, today)
		if status != c.wantStatus || paidLate != c.wantPaidLate {
			t.Errorf("%s: got (%s,%v) want (%s,%v)", c.name, status, paidLate, c.wantStatus, c.wantPaidLate)
		}
	}
}
