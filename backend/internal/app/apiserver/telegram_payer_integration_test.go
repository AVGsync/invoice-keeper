//go:build integration

package apiserver

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/AVGsync/invoice-keeper/backend/internal/clock"
	"github.com/AVGsync/invoice-keeper/backend/internal/reminder"
	"github.com/AVGsync/invoice-keeper/backend/internal/reminder/remindertest"
	"github.com/AVGsync/invoice-keeper/backend/internal/repository/postgres"
	"github.com/AVGsync/invoice-keeper/backend/internal/service"
	"github.com/lib/pq"
)

// RunPayerContract против paymentsPayer на реальной БД (§G11). Та же фикстура,
// что у источника, плюс админ. Запуск:
//
//	TEST_DATABASE_URL=postgres://.../invoice_keeper_test?sslmode=disable \
//	    go test -tags integration ./internal/app/apiserver/
func TestPaymentsPayerContract(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL не задан")
	}
	raw, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer raw.Close()
	if err := raw.Ping(); err != nil {
		t.Fatalf("ping: %v", err)
	}

	var n int
	if err := raw.QueryRow(`SELECT COUNT(*) FROM information_schema.tables
		WHERE table_name IN ('recurrent_payments','payments','user_recurrent_payments')`).Scan(&n); err != nil {
		t.Fatalf("check tables: %v", err)
	}
	if n != 3 {
		t.Skip("таблиц модуля B ещё нет")
	}

	db := postgres.NewFromDB(raw)
	ts, _ := time.Parse("2006-01-02", "2026-10-24")
	clk := clock.NewFixed(ts, time.UTC)

	remindertest.RunPayerContract(t, func(t *testing.T, fx remindertest.Fixtures) reminder.Payer {
		seedPayerFixtures(t, raw, fx)
		admins := map[string]bool{fx.Admin: true}
		return paymentsPayer{
			payments: service.NewPaymentService(db.Payment(), clk),
			source:   db.TelegramSource(),
			isAdmin:  func(_ context.Context, id string) bool { return admins[id] },
		}
	})
}

func seedPayerFixtures(t *testing.T, db *sql.DB, fx remindertest.Fixtures) {
	t.Helper()
	ctx := context.Background()

	occIDs := make([]string, 0, len(fx.Occurrences))
	for _, o := range fx.Occurrences {
		occIDs = append(occIDs, o.ID)
	}
	_, _ = db.ExecContext(ctx, `DELETE FROM payments WHERE id = ANY($1)`, pq.Array(occIDs))
	_, _ = db.ExecContext(ctx, `DELETE FROM user_recurrent_payments WHERE recurrent_payment_id = ANY($1)`, pq.Array(occIDs))
	_, _ = db.ExecContext(ctx, `DELETE FROM recurrent_payments WHERE id = ANY($1)`, pq.Array(occIDs))

	for _, u := range []struct{ id, name string }{
		{fx.Assignee, "Иван Петров"}, {fx.Other, "Пётр Сидоров"}, {fx.Admin, "Админ"},
	} {
		if _, err := db.ExecContext(ctx, `
			INSERT INTO users (id, full_name, login, password_hash, role)
			VALUES ($1, $2, $3, 'x', 'user') ON CONFLICT (id) DO NOTHING`,
			u.id, u.name, "fx_"+u.id[len(u.id)-12:]); err != nil { // хвост UUID уникален
			t.Fatalf("insert user: %v", err)
		}
	}

	for _, o := range fx.Occurrences {
		userID := fx.Other
		if o.AssigneeID != nil {
			userID = *o.AssigneeID
		}
		var stoppedAt any
		if o.Ended {
			stoppedAt = o.DueDate.String()
		}
		if _, err := db.ExecContext(ctx, `
			INSERT INTO recurrent_payments (id, amount, currency, frequency, "interval", description, day_offset, start_date, stopped_at)
			VALUES ($1, (($2::numeric)*100)::bigint, $3, 'monthly', 1, $4, $5, $6::date, $7::timestamptz)
			ON CONFLICT (id) DO NOTHING`,
			o.ID, o.Amount, o.Currency, o.Title, o.DueDate.D, o.DueDate.String(), stoppedAt); err != nil {
			t.Fatalf("insert recurrent payment: %v", err)
		}
		if _, err := db.ExecContext(ctx, `
			INSERT INTO user_recurrent_payments (user_id, recurrent_payment_id)
			VALUES ($1, $2) ON CONFLICT DO NOTHING`, userID, o.ID); err != nil {
			t.Fatalf("insert assignment: %v", err)
		}
		var paidDate, paidAt any
		status := "pending"
		if o.PaidDate != nil {
			paidDate = o.PaidDate.String()
			paidAt = o.PaidDate.String()
			status = "paid"
		}
		if _, err := db.ExecContext(ctx, `
			INSERT INTO payments (id, user_id, recurrent_payment_id, amount, currency, description, due_date, paid_date, paid_at, status)
			VALUES ($1, $2, $3, (($4::numeric)*100)::bigint, $5, $6, $7::date, $8::date, $9::timestamptz, $10)`,
			o.ID, userID, o.ID, o.Amount, o.Currency, o.Title, o.DueDate.String(), paidDate, paidAt, status); err != nil {
			t.Fatalf("insert payment: %v", err)
		}
	}
}
