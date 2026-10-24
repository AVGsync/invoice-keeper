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
	"github.com/AVGsync/invoice-keeper/backend/internal/model/response"
	"github.com/AVGsync/invoice-keeper/backend/internal/transport/http/middleware"
)

type fakeAuditSvc struct {
	gotFilter model.AuditFilter
	entries   []model.AuditEntry
	total     int
}

func (f *fakeAuditSvc) List(_ context.Context, flt model.AuditFilter) ([]model.AuditEntry, int, error) {
	f.gotFilter = flt
	return f.entries, f.total, nil
}

func auditTestHandler(svc AuditUseCase) *AuditHandler {
	clk := clock.NewFixed(time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC), time.UTC)
	return NewAuditHandler(svc, clk)
}

func withAuditClaims(r *http.Request, userID, role string) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.ClaimsKey, &model.Claims{UserID: userID, Role: role})
	return r.WithContext(ctx)
}

func TestMyActivityOverridesActor(t *testing.T) {
	svc := &fakeAuditSvc{total: 0}
	h := auditTestHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/me/activity?actor_id=99999999-9999-4999-8999-999999999999", nil)
	r = withAuditClaims(r, "me-user-id", "user")
	w := httptest.NewRecorder()
	h.MyActivity()(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if len(svc.gotFilter.ActorIDs) != 1 || svc.gotFilter.ActorIDs[0] != "me-user-id" {
		t.Errorf("actor not overridden to JWT user: %v", svc.gotFilter.ActorIDs)
	}
}

func TestAuditLogUsesActorParam(t *testing.T) {
	svc := &fakeAuditSvc{}
	h := auditTestHandler(svc)
	r := httptest.NewRequest(http.MethodGet, "/audit-log?actor_id=11111111-1111-4111-8111-111111111111&action=recurring.*", nil)
	r = withAuditClaims(r, "admin-id", "admin")
	w := httptest.NewRecorder()
	h.AuditLog()(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if len(svc.gotFilter.ActorIDs) != 1 || svc.gotFilter.ActorIDs[0] != "11111111-1111-4111-8111-111111111111" {
		t.Errorf("actor param not used: %v", svc.gotFilter.ActorIDs)
	}
}

func TestAuditLogBadParam(t *testing.T) {
	h := auditTestHandler(&fakeAuditSvc{})
	r := httptest.NewRequest(http.MethodGet, "/audit-log?channel=sms", nil)
	r = withAuditClaims(r, "admin-id", "admin")
	w := httptest.NewRecorder()
	h.AuditLog()(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestAuditItemMapping(t *testing.T) {
	actorID := "a-1"
	role := "admin"
	name := "Ольга"
	etype := "recurring_payment"
	eid := "rp-1"
	svc := &fakeAuditSvc{
		total: 1,
		entries: []model.AuditEntry{{
			ID:         7,
			CreatedAt:  time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC),
			ActorID:    &actorID,
			ActorRole:  &role,
			ActorName:  &name,
			Channel:    "web",
			Action:     "recurring.update",
			Outcome:    "success",
			EntityType: &etype,
			EntityID:   &eid,
			Changes:    []byte(`{"amount":{"from":100,"to":200}}`),
		}},
	}
	h := auditTestHandler(svc)
	r := httptest.NewRequest(http.MethodGet, "/audit-log", nil)
	r = withAuditClaims(r, "admin-id", "admin")
	w := httptest.NewRecorder()
	h.AuditLog()(w, r)

	var resp response.AuditLogResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Total != 1 || len(resp.Items) != 1 {
		t.Fatalf("items = %+v", resp)
	}
	it := resp.Items[0]
	if it.Actor == nil || it.Actor.ID != "a-1" || *it.Actor.FullName != "Ольга" {
		t.Errorf("actor mapping wrong: %+v", it.Actor)
	}
	if it.ActionLabel != "Изменён регулярный платёж" {
		t.Errorf("label = %q", it.ActionLabel)
	}
	if string(it.Changes) != `{"amount":{"from":100,"to":200}}` {
		t.Errorf("changes = %s", it.Changes)
	}
	if string(it.Payload) != "null" {
		t.Errorf("nil payload should be null, got %s", it.Payload)
	}
}

func TestAuditActions(t *testing.T) {
	h := auditTestHandler(&fakeAuditSvc{})
	r := httptest.NewRequest(http.MethodGet, "/audit-log/actions", nil)
	w := httptest.NewRecorder()
	h.Actions()(w, r)
	var resp response.AuditActionsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Items) == 0 {
		t.Errorf("catalogue must not be empty")
	}
}
