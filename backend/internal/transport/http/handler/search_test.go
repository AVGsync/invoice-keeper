package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/AVGsync/invoice-keeper/backend/internal/clock"
	"github.com/AVGsync/invoice-keeper/backend/internal/model"
	"github.com/AVGsync/invoice-keeper/backend/internal/transport/http/middleware"
)

// fakeSearch — подмена сервиса поиска для контрактных тестов хендлера.
type fakeSearch struct {
	occ      []model.Payment
	occTotal int
	rec      []model.RecurringSearchRow
	recTotal int
	gotOcc   model.OccurrenceFilter
	gotRec   model.RecurringFilter
}

func (f *fakeSearch) SearchOccurrences(_ context.Context, flt model.OccurrenceFilter) ([]model.Payment, int, error) {
	f.gotOcc = flt
	return f.occ, f.occTotal, nil
}

func (f *fakeSearch) SearchRecurring(_ context.Context, flt model.RecurringFilter) ([]model.RecurringSearchRow, int, error) {
	f.gotRec = flt
	return f.rec, f.recTotal, nil
}

func searchClk() clock.Clock {
	return clock.NewFixed(time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC), time.UTC)
}

func withSearchClaims(r *http.Request) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.ClaimsKey, &model.Claims{UserID: "u1", Role: "user"})
	return r.WithContext(ctx)
}

func TestSearchOccurrencesEnvelope(t *testing.T) {
	due := "2026-06-01"
	svc := &fakeSearch{
		occ: []model.Payment{
			{ID: "p1", UserID: "u1", Amount: 100, Currency: "USD", DueDate: due, RecurrentPaymentID: ptr("rp1")},
		},
		occTotal: 7,
	}
	h := NewSearchHandler(svc, searchClk())

	req := withSearchClaims(httptest.NewRequest(http.MethodGet, "/payment-occurrences/search?limit=5&offset=0", nil))
	rec := httptest.NewRecorder()
	h.SearchOccurrences()(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d", rec.Code)
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, k := range []string{"items", "total", "limit", "offset"} {
		if _, ok := body[k]; !ok {
			t.Fatalf("missing key %q in %s", k, rec.Body.String())
		}
	}
	var env struct {
		Items []struct {
			Status string `json:"status"`
		} `json:"items"`
		Total  int `json:"total"`
		Limit  int `json:"limit"`
		Offset int `json:"offset"`
	}
	json.Unmarshal(rec.Body.Bytes(), &env)
	if env.Total != 7 || env.Limit != 5 || env.Offset != 0 {
		t.Fatalf("envelope values: %#v", env)
	}
	if len(env.Items) != 1 || env.Items[0].Status != "overdue" {
		t.Fatalf("expected computed overdue status, got %#v", env.Items)
	}
}

func TestSearchOccurrencesEmptyIsArray(t *testing.T) {
	svc := &fakeSearch{occ: nil, occTotal: 0}
	h := NewSearchHandler(svc, searchClk())
	req := withSearchClaims(httptest.NewRequest(http.MethodGet, "/payment-occurrences/search", nil))
	rec := httptest.NewRecorder()
	h.SearchOccurrences()(rec, req)

	var env struct {
		Items json.RawMessage `json:"items"`
	}
	json.Unmarshal(rec.Body.Bytes(), &env)
	if string(env.Items) != "[]" {
		t.Fatalf("items must be [] not null, got %s", env.Items)
	}
}

func TestSearchOccurrencesBadParam(t *testing.T) {
	svc := &fakeSearch{}
	h := NewSearchHandler(svc, searchClk())
	req := withSearchClaims(httptest.NewRequest(http.MethodGet, "/payment-occurrences/search?limit=0", nil))
	rec := httptest.NewRecorder()
	h.SearchOccurrences()(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d", rec.Code)
	}
	if got := rec.Body.String(); got != "limit must be between 1 and 200\n" {
		t.Fatalf("error text passthrough: %q", got)
	}
}

func TestSearchOccurrencesNoClaims(t *testing.T) {
	svc := &fakeSearch{}
	h := NewSearchHandler(svc, searchClk())
	req := httptest.NewRequest(http.MethodGet, "/payment-occurrences/search", nil)
	rec := httptest.NewRecorder()
	h.SearchOccurrences()(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status: got %d", rec.Code)
	}
}

func TestSearchRecurringEnvelope(t *testing.T) {
	stopped := time.Now()
	nd := "2026-07-01"
	svc := &fakeSearch{
		rec: []model.RecurringSearchRow{
			{RecurrentPayment: model.RecurrentPayment{ID: "rp1", Amount: 500, Currency: "RUB"}, NextDueDate: &nd},
			{RecurrentPayment: model.RecurrentPayment{ID: "rp2", Amount: 500, Currency: "RUB", StoppedAt: &stopped}},
		},
		recTotal: 2,
	}
	h := NewSearchHandler(svc, searchClk())
	req := withSearchClaims(httptest.NewRequest(http.MethodGet, "/recurring-payments/search?include_stopped=true", nil))
	rec := httptest.NewRecorder()
	h.SearchRecurring()(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d (%s)", rec.Code, rec.Body.String())
	}
	var env struct {
		Items []struct {
			Status      string  `json:"status"`
			NextDueDate *string `json:"next_due_date"`
		} `json:"items"`
		Total int `json:"total"`
	}
	json.Unmarshal(rec.Body.Bytes(), &env)
	if env.Total != 2 || len(env.Items) != 2 {
		t.Fatalf("envelope: %#v", env)
	}
	if env.Items[0].Status != "active" || env.Items[0].NextDueDate == nil || *env.Items[0].NextDueDate != nd {
		t.Fatalf("active item: %#v", env.Items[0])
	}
	if env.Items[1].Status != "stopped" || env.Items[1].NextDueDate != nil {
		t.Fatalf("stopped item: %#v", env.Items[1])
	}
}

func ptr(s string) *string { return &s }
