package service

import (
	"context"
	"testing"
	"time"

	"github.com/AVGsync/invoice-keeper/backend/internal/audit"
	"github.com/AVGsync/invoice-keeper/backend/internal/clock"
	"github.com/AVGsync/invoice-keeper/backend/internal/model"
	"github.com/AVGsync/invoice-keeper/backend/internal/model/request"
)

type fakeUserRepo struct {
	byLogin    model.User
	byLoginErr error
	byID       model.User
	byIDErr    error
}

func (f *fakeUserRepo) GetUserByLogin(string, context.Context) (model.User, error) {
	return f.byLogin, f.byLoginErr
}
func (f *fakeUserRepo) AddRefreshToken(string, time.Time, time.Time, *time.Time, context.Context) error {
	return nil
}
func (f *fakeUserRepo) GetRefreshTokenByHash(string, context.Context) (model.RefreshToken, error) {
	return model.RefreshToken{}, nil
}
func (f *fakeUserRepo) GetUserByID(string, context.Context) (model.User, error) {
	return f.byID, f.byIDErr
}
func (f *fakeUserRepo) RevokeRefreshToken(string, context.Context) error  { return nil }
func (f *fakeUserRepo) CreateUser(model.User, context.Context) error      { return nil }
func (f *fakeUserRepo) GetAllUsers(context.Context) ([]model.User, error) { return nil, nil }
func (f *fakeUserRepo) DeleteUserByID(string, context.Context) error      { return nil }
func (f *fakeUserRepo) UpdateUser(model.User, context.Context) error      { return nil }

type fakeHasher struct{ ok bool }

func (f fakeHasher) HashPassword(string) (string, error)     { return "hash", nil }
func (f fakeHasher) CheckPassword(string, string) bool       { return f.ok }
func (f fakeHasher) HashRefreshToken(string) (string, error) { return "rhash", nil }
func (f fakeHasher) CheckRefreshToken(string, string) bool   { return true }

type fakeJWT struct{ refreshUser string }

func (f fakeJWT) GenerateAccess(string, string) (string, error)  { return "access", nil }
func (f fakeJWT) GenerateRefresh(string, string) (string, error) { return "refresh", nil }
func (f fakeJWT) ValidateRefresh(string) (*model.Claims, error) {
	return &model.Claims{UserID: f.refreshUser, Role: "user"}, nil
}

func userSvc(repo *fakeUserRepo, ok bool) (*UserService, *recSpy) {
	rec := &recSpy{}
	s := NewUserService(repo, fakeHasher{ok: ok}, fakeJWT{refreshUser: "u1"}, time.Hour, time.Hour)
	s.SetAuditRecorder(rec)
	return s, rec
}

func TestAuditLoginSuccess(t *testing.T) {
	repo := &fakeUserRepo{byLogin: model.User{ID: "u1", Login: "olga", Role: "user", PasswordHash: "h"}}
	s, rec := userSvc(repo, true)
	if _, _, err := s.AuthenticateUser(request.LoginRequest{Login: "olga", Password: "p"}, context.Background()); err != nil {
		t.Fatal(err)
	}
	e := rec.one(t)
	if e.Action != audit.ActionAuthLogin || e.Outcome != "" || e.ActorID != "u1" {
		t.Errorf("unexpected: %+v", e)
	}
	if e.Payload["login"] != "olga" {
		t.Errorf("login payload wrong: %+v", e.Payload)
	}
}

func TestAuditLoginFailure(t *testing.T) {
	repo := &fakeUserRepo{byLogin: model.User{ID: "u1", Login: "olga", PasswordHash: "h"}}
	s, rec := userSvc(repo, false) // пароль не сойдётся
	if _, _, err := s.AuthenticateUser(request.LoginRequest{Login: "olga", Password: "bad"}, context.Background()); err == nil {
		t.Fatal("expected error")
	}
	e := rec.one(t)
	if e.Action != audit.ActionAuthLogin || e.Outcome != audit.OutcomeFailure {
		t.Fatalf("unexpected: %+v", e)
	}
	if e.ActorID != "" || e.Payload["reason"] != "invalid_credentials" {
		t.Errorf("failure entry wrong: %+v", e)
	}
}

func TestAuditLogout(t *testing.T) {
	repo := &fakeUserRepo{}
	s, rec := userSvc(repo, true)
	if err := s.RevokeRefreshToken(context.Background(), "token"); err != nil {
		t.Fatal(err)
	}
	e := rec.one(t)
	if e.Action != audit.ActionAuthLogout || e.ActorID != "u1" {
		t.Errorf("unexpected: %+v", e)
	}
}

func TestAuditUserCreate(t *testing.T) {
	repo := &fakeUserRepo{byLogin: model.User{ID: "u-new"}}
	s, rec := userSvc(repo, true)
	if err := s.CreateUser(request.CreateUserRequest{FullName: "Иван Петров", Login: "ivan", Password: "password1"}, context.Background()); err != nil {
		t.Fatal(err)
	}
	e := rec.one(t)
	if e.Action != audit.ActionUserCreate || e.EntityID != "u-new" {
		t.Errorf("unexpected: %+v", e)
	}
	if e.Payload["login"] != "ivan" || e.Payload["full_name"] != "Иван Петров" {
		t.Errorf("payload wrong: %+v", e.Payload)
	}
}

func TestAuditUserUpdate(t *testing.T) {
	repo := &fakeUserRepo{byID: model.User{ID: "u1", Login: "old", FullName: "Старый", Role: "user"}}
	s, rec := userSvc(repo, true)
	newName := "Новый"
	if err := s.UpdateUserByID(context.Background(), "u1", request.UserUpdateRequest{FullName: &newName}); err != nil {
		t.Fatal(err)
	}
	e := rec.one(t)
	if e.Action != audit.ActionUserUpdate {
		t.Fatalf("action = %q", e.Action)
	}
	if e.Changes["full_name"].From != "Старый" || e.Changes["full_name"].To != "Новый" {
		t.Errorf("full_name change wrong: %+v", e.Changes)
	}
	if _, ok := e.Changes["login"]; ok {
		t.Errorf("login must not be in changes (unchanged)")
	}
}

func TestAuditUserDelete(t *testing.T) {
	repo := &fakeUserRepo{byID: model.User{ID: "u1", Login: "olga", FullName: "Ольга", Role: "user"}}
	s, rec := userSvc(repo, true)
	if err := s.DeleteUserByID("u1", context.Background()); err != nil {
		t.Fatal(err)
	}
	e := rec.one(t)
	if e.Action != audit.ActionUserDelete || e.Payload["login"] != "olga" {
		t.Errorf("unexpected: %+v", e)
	}
}

// recSpy — фейковый рекордер: собирает записи для проверок.
type recSpy struct{ entries []audit.Entry }

func (r *recSpy) Record(_ context.Context, e audit.Entry) { r.entries = append(r.entries, e) }

func (r *recSpy) one(t *testing.T) audit.Entry {
	t.Helper()
	if len(r.entries) != 1 {
		t.Fatalf("want exactly 1 audit entry, got %d: %+v", len(r.entries), r.entries)
	}
	return r.entries[0]
}

func strptr(s string) *string { return &s }

type fakePersonaRepo struct {
	persona model.Persona
}

func (f *fakePersonaRepo) GetAllPersonas(context.Context) ([]model.Persona, error) { return nil, nil }
func (f *fakePersonaRepo) GetPersonaByID(string, context.Context) (model.Persona, error) {
	return f.persona, nil
}
func (f *fakePersonaRepo) CreatePersona(p model.Persona, _ context.Context) (model.Persona, error) {
	p.ID = "p-new"
	return p, nil
}
func (f *fakePersonaRepo) UpdatePersona(model.Persona, context.Context) error { return nil }
func (f *fakePersonaRepo) DeletePersona(string, context.Context) error        { return nil }

func TestAuditPersonaCreate(t *testing.T) {
	rec := &recSpy{}
	s := NewPersonaService(&fakePersonaRepo{})
	s.SetAuditRecorder(rec)
	if _, err := s.CreatePersona(request.CreatePersonaRequest{Name: "Бухгалтер", Description: strptr("desc")}, context.Background()); err != nil {
		t.Fatal(err)
	}
	e := rec.one(t)
	if e.Action != audit.ActionPersonaCreate || e.EntityID != "p-new" {
		t.Errorf("unexpected entry: %+v", e)
	}
	if e.Payload["name"] != "Бухгалтер" {
		t.Errorf("payload name wrong: %+v", e.Payload)
	}
}

func TestAuditPersonaUpdate(t *testing.T) {
	rec := &recSpy{}
	s := NewPersonaService(&fakePersonaRepo{persona: model.Persona{ID: "p1", Name: "Old"}})
	s.SetAuditRecorder(rec)
	if _, err := s.UpdatePersona("p1", request.UpdatePersonaRequest{Name: strptr("New")}, context.Background()); err != nil {
		t.Fatal(err)
	}
	e := rec.one(t)
	if e.Action != audit.ActionPersonaUpdate {
		t.Fatalf("action = %q", e.Action)
	}
	if e.Changes["name"].From != "Old" || e.Changes["name"].To != "New" {
		t.Errorf("name change wrong: %+v", e.Changes)
	}
}

func TestAuditPersonaDelete(t *testing.T) {
	rec := &recSpy{}
	s := NewPersonaService(&fakePersonaRepo{persona: model.Persona{ID: "p1", Name: "Z"}})
	s.SetAuditRecorder(rec)
	if err := s.DeletePersona("p1", context.Background()); err != nil {
		t.Fatal(err)
	}
	e := rec.one(t)
	if e.Action != audit.ActionPersonaDelete || e.Payload["name"] != "Z" {
		t.Errorf("unexpected: %+v", e)
	}
}

type fakePaymentRepo struct {
	payment model.Payment
	markErr error
}

func (f *fakePaymentRepo) CreatePayment(model.Payment, context.Context) (string, error) {
	return "", nil
}
func (f *fakePaymentRepo) GetPaymentByID(string, context.Context) (model.Payment, error) {
	return f.payment, nil
}
func (f *fakePaymentRepo) GetPaymentsByUserID(string, context.Context) ([]model.Payment, error) {
	return nil, nil
}
func (f *fakePaymentRepo) GetAllPayments(context.Context) ([]model.Payment, error) { return nil, nil }
func (f *fakePaymentRepo) UpdatePayment(model.Payment, context.Context) error      { return nil }
func (f *fakePaymentRepo) MarkPaymentAsPaid(string, string, context.Context) error { return f.markErr }
func (f *fakePaymentRepo) ChangePaidDate(string, string, context.Context) error    { return nil }
func (f *fakePaymentRepo) MarkPaymentAsUnpaid(string, string, context.Context) error {
	return nil
}
func (f *fakePaymentRepo) DeletePayment(string, context.Context) error { return nil }

func paymentSvc(repo *fakePaymentRepo) (*PaymentService, *recSpy) {
	rec := &recSpy{}
	clk := clock.NewFixed(time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC), time.UTC)
	s := NewPaymentService(repo, clk)
	s.SetAuditRecorder(rec)
	return s, rec
}

func TestAuditOccurrenceConfirmPaidLate(t *testing.T) {
	repo := &fakePaymentRepo{payment: model.Payment{ID: "o1", UserID: "u", DueDate: "2026-10-01", Amount: 100, Currency: "USD"}}
	s, rec := paymentSvc(repo)
	paid, _ := clock.ParseDate("2026-10-24")
	if err := s.MarkOccurrencePaid(context.Background(), "o1", paid, PaymentActor{IsAdmin: true}); err != nil {
		t.Fatal(err)
	}
	e := rec.one(t)
	if e.Action != audit.ActionOccurrenceConfirm {
		t.Fatalf("action = %q", e.Action)
	}
	if e.Payload["paid_late"] != true {
		t.Errorf("expected paid_late true: %+v", e.Payload)
	}
}

func TestAuditOccurrenceCancelPaid(t *testing.T) {
	repo := &fakePaymentRepo{payment: model.Payment{ID: "o1", UserID: "u", DueDate: "2026-10-01", PaidDate: strptr("2026-10-10")}}
	s, rec := paymentSvc(repo)
	if err := s.MarkOccurrenceUnpaid(context.Background(), "o1", PaymentActor{IsAdmin: true}); err != nil {
		t.Fatal(err)
	}
	e := rec.one(t)
	if e.Action != audit.ActionOccurrenceCancel {
		t.Fatalf("action = %q", e.Action)
	}
	if e.Changes["paid_date"].From != "2026-10-10" || e.Changes["paid_date"].To != nil {
		t.Errorf("paid_date change wrong: %+v", e.Changes)
	}
}

func TestAuditOccurrenceCancelNoop(t *testing.T) {
	repo := &fakePaymentRepo{payment: model.Payment{ID: "o1", UserID: "u", DueDate: "2026-10-01"}}
	s, rec := paymentSvc(repo)
	if err := s.MarkOccurrenceUnpaid(context.Background(), "o1", PaymentActor{IsAdmin: true}); err != nil {
		t.Fatal(err)
	}
	e := rec.one(t)
	if e.Payload["noop"] != true || e.Changes != nil {
		t.Errorf("expected noop payload, no changes: %+v", e)
	}
}

func TestAuditOccurrenceUpdate(t *testing.T) {
	repo := &fakePaymentRepo{payment: model.Payment{ID: "o1", UserID: "u", DueDate: "2026-10-01", Amount: 100, Currency: "USD", Description: "a"}}
	s, rec := paymentSvc(repo)
	amount := int64(200)
	if err := s.UpdatePayment("o1", request.UpdatePaymentRequest{Amount: &amount}, context.Background()); err != nil {
		t.Fatal(err)
	}
	e := rec.one(t)
	if e.Action != audit.ActionOccurrenceUpdate {
		t.Fatalf("action = %q", e.Action)
	}
	if e.Changes["amount"].From != int64(100) || e.Changes["amount"].To != int64(200) {
		t.Errorf("amount change wrong: %+v", e.Changes)
	}
}

type fakeNotifRepo struct {
	settings model.UserNotificationSettings
	exists   bool
}

func (f *fakeNotifRepo) GetByUserID(string, context.Context) (model.UserNotificationSettings, error) {
	return f.settings, nil
}
func (f *fakeNotifRepo) Create(s model.UserNotificationSettings, _ context.Context) error {
	f.settings = s
	return nil
}
func (f *fakeNotifRepo) Update(s model.UserNotificationSettings, _ context.Context) error {
	f.settings = s
	return nil
}
func (f *fakeNotifRepo) Delete(string, context.Context) error { return nil }

func validNotifReq() request.UpdateUserNotificationSettingsRequest {
	return request.UpdateUserNotificationSettingsRequest{
		Email:         "olga@example.com",
		EmailEnabled:  true,
		DaysBeforeDue: 3,
		SendTime:      "09:00",
		Timezone:      "UTC",
	}
}

func TestAuditNotificationCreate(t *testing.T) {
	rec := &recSpy{}
	s := NewNotificationsService(&fakeNotifRepo{})
	s.SetAuditRecorder(rec)
	if err := s.Create("u1", validNotifReq(), context.Background()); err != nil {
		t.Fatal(err)
	}
	e := rec.one(t)
	if e.Action != audit.ActionNotificationSettingsCreate || e.EntityID != "u1" {
		t.Errorf("unexpected: %+v", e)
	}
	if e.Changes["email"].To != "olga@example.com" {
		t.Errorf("email change missing (masked only in Logger): %+v", e.Changes)
	}
}

func TestAuditNotificationPatch(t *testing.T) {
	rec := &recSpy{}
	repo := &fakeNotifRepo{settings: model.UserNotificationSettings{UserID: "u1", Email: "old@e.com", DaysBeforeDue: 1, SendTime: "08:00", Timezone: "UTC", EmailEnabled: false}}
	s := NewNotificationsService(repo)
	s.SetAuditRecorder(rec)
	enabled := true
	if err := s.Patch("u1", request.PatchUserNotificationSettingsRequest{EmailEnabled: &enabled}, context.Background()); err != nil {
		t.Fatal(err)
	}
	e := rec.one(t)
	if e.Action != audit.ActionNotificationSettingsUpdate {
		t.Fatalf("action = %q", e.Action)
	}
	if e.Changes["email_enabled"].From != false || e.Changes["email_enabled"].To != true {
		t.Errorf("email_enabled change wrong: %+v", e.Changes)
	}
}
