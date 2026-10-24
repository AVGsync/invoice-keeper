//go:build integration

package postgres

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/AVGsync/invoice-keeper/backend/internal/reminder"
	"github.com/AVGsync/invoice-keeper/backend/internal/reminder/remindertest"
	"github.com/lib/pq"
)

// Контракт SQL-источника против реальных таблиц модуля B (§2.5).
//
// ВНИМАНИЕ: тест ПИШЕТ и УДАЛЯЕТ строки фикстур в БД по адресу TEST_DATABASE_URL.
// Удаляются только строки с фиксированными UUID фикстур (не TRUNCATE), поэтому
// на выделенной тестовой базе (в CI — invoice_keeper_test) это безопасно. НЕ
// указывайте сюда рабочую БД. Запуск:
//
//	TEST_DATABASE_URL=postgres://.../invoice_keeper_test?sslmode=disable \
//	    go test -tags integration ./internal/repository/postgres/
func TestTelegramSourceContract(t *testing.T) {
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
	if !bTablesPresent(t, raw) {
		t.Skip("таблиц модуля B (recurrent_payments/payments/user_recurrent_payments) ещё нет")
	}

	db := &DB{db: raw}
	remindertest.RunSourceContract(t, func(t *testing.T, fx remindertest.Fixtures) reminder.OccurrenceSource {
		seedBFixtures(t, raw, fx)
		return db.TelegramSource()
	}, remindertest.AssigneeRequired())
}

func bTablesPresent(t *testing.T, db *sql.DB) bool {
	t.Helper()
	var n int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM information_schema.tables
		WHERE table_name IN ('recurrent_payments','payments','user_recurrent_payments')`).Scan(&n)
	if err != nil {
		t.Fatalf("check tables: %v", err)
	}
	return n == 3
}

// seedBFixtures вставляет фикстуры в реальные таблицы B (миграции 000006–000010).
//
// Инварианты:
//   - у каждого периода свой recurrent_payment (id = id периода), monthly/interval 1,
//     day_offset = день due, start_date = due, stopped_at = NOW() у завершённого;
//   - payments.user_id NOT NULL, поэтому у периода без ответственного ставим Other;
//     отдельная строка user_recurrent_payments делает его видимым как assignee;
//   - amount переводится в минимальные единицы (×100);
//   - paid_date/paid_at/status согласованы с CHECK (paid_date<=>paid_at, status);
//   - очистка ограничена UUID фикстур; порядок: payments → user_recurrent_payments → recurrent_payments.
func seedBFixtures(t *testing.T, db *sql.DB, fx remindertest.Fixtures) {
	t.Helper()
	ctx := context.Background()

	occIDs := make([]string, 0, len(fx.Occurrences))
	for _, o := range fx.Occurrences {
		occIDs = append(occIDs, o.ID)
	}

	if _, err := db.ExecContext(ctx, `DELETE FROM payments WHERE id = ANY($1)`, pq.Array(occIDs)); err != nil {
		t.Fatalf("clean payments: %v", err)
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM user_recurrent_payments WHERE recurrent_payment_id = ANY($1)`, pq.Array(occIDs)); err != nil {
		t.Fatalf("clean user_recurrent_payments: %v", err)
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM recurrent_payments WHERE id = ANY($1)`, pq.Array(occIDs)); err != nil {
		t.Fatalf("clean recurrent_payments: %v", err)
	}

	// Пользователи: ответственный, Other и админ (для payer-теста).
	for _, u := range []struct{ id, name string }{
		{fx.Assignee, "Иван Петров"},
		{fx.Other, "Пётр Сидоров"},
		{fx.Admin, "Админ"},
	} {
		login := "fx_" + u.id[:8]
		if _, err := db.ExecContext(ctx, `
			INSERT INTO users (id, full_name, login, password_hash, role)
			VALUES ($1, $2, $3, 'x', 'user')
			ON CONFLICT (id) DO NOTHING`, u.id, u.name, login); err != nil {
			t.Fatalf("insert user fixture: %v", err)
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
			t.Fatalf("insert user_recurrent_payments: %v", err)
		}

		var paidDate any
		var paidAt any
		status := "pending"
		if o.PaidDate != nil {
			paidDate = o.PaidDate.String()
			paidAt = o.PaidDate.String() // timestamptz примет дату
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
