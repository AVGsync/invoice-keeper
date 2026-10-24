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

// Контракт SQL-источника против реальных таблиц модуля B (§2.5). Пока таблиц B
// нет, тест помечается Skip: до интеграции (этап 4) гонять нечего.
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
		t.Skip("таблиц модуля B (recurring_payments/payment_occurrences) ещё нет")
	}

	db := &DB{db: raw}
	remindertest.RunSourceContract(t, func(t *testing.T, fx remindertest.Fixtures) reminder.OccurrenceSource {
		seedBFixtures(t, raw, fx)
		return db.TelegramSource()
	})
}

func bTablesPresent(t *testing.T, db *sql.DB) bool {
	t.Helper()
	var n int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM information_schema.tables
		WHERE table_name IN ('recurring_payments','payment_occurrences')`).Scan(&n)
	if err != nil {
		t.Fatalf("check tables: %v", err)
	}
	return n == 2
}

// seedBFixtures вставляет фикстуры в таблицы B по ПРЕДПОЛАГАЕМЫМ колонкам (§2.3).
// Если реальная схема B отличается, здесь и в telegram-source-rep.go правятся
// имена колонок — тест для того и нужен.
//
// Инварианты фикстур (исправляют прежние дефекты, finding):
//   - у каждого периода СВОЙ recurring_payment с валидным UUID (= id периода),
//     потому что title лежит на платеже и у периодов он разный;
//   - ответственные вставляются в users, чтобы FK assignee_id → users проходил;
//   - очистка ограничена UUID фикстур (не TRUNCATE), чтобы не трогать чужие данные.
func seedBFixtures(t *testing.T, db *sql.DB, fx remindertest.Fixtures) {
	t.Helper()
	ctx := context.Background()

	occIDs := make([]string, 0, len(fx.Occurrences))
	for _, o := range fx.Occurrences {
		occIDs = append(occIDs, o.ID)
	}

	// Чистим только строки фикстур (rp.id = o.id, см. ниже). Порядок: периоды → платежи.
	if _, err := db.ExecContext(ctx, `DELETE FROM payment_occurrences WHERE id = ANY($1)`, pq.Array(occIDs)); err != nil {
		t.Fatalf("clean occurrences: %v", err)
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM recurring_payments WHERE id = ANY($1)`, pq.Array(occIDs)); err != nil {
		t.Fatalf("clean payments: %v", err)
	}

	// Пользователи-ответственные: FK assignee_id → users должен выполняться.
	seenUser := map[string]bool{}
	for _, o := range fx.Occurrences {
		if o.AssigneeID == nil || seenUser[*o.AssigneeID] {
			continue
		}
		seenUser[*o.AssigneeID] = true
		login := "fx_" + (*o.AssigneeID)[:8]
		if _, err := db.ExecContext(ctx, `
			INSERT INTO users (id, full_name, login, password_hash, role)
			VALUES ($1, $2, $3, 'x', 'user')
			ON CONFLICT (id) DO NOTHING`,
			*o.AssigneeID, o.AssigneeName, login); err != nil {
			t.Fatalf("insert user fixture: %v", err)
		}
	}

	for _, o := range fx.Occurrences {
		var assignee any
		if o.AssigneeID != nil {
			assignee = *o.AssigneeID
		}
		var endedAt any
		if o.Ended {
			endedAt = o.DueDate.String() // непустая дата => ended
		}
		// rp.id = o.id — валидный UUID, уникальный на период (title у периодов разный).
		if _, err := db.ExecContext(ctx, `
			INSERT INTO recurring_payments (id, title, assignee_id, ended_at)
			VALUES ($1, $2, $3, $4::date)
			ON CONFLICT (id) DO NOTHING`,
			o.ID, o.Title, assignee, endedAt); err != nil {
			t.Fatalf("insert payment: %v", err)
		}
		var paid any
		if o.PaidDate != nil {
			paid = o.PaidDate.String()
		}
		if _, err := db.ExecContext(ctx, `
			INSERT INTO payment_occurrences (id, recurring_payment_id, amount, currency, due_date, paid_date)
			VALUES ($1, $2, $3::numeric, $4, $5::date, $6::date)`,
			o.ID, o.ID, o.Amount, o.Currency, o.DueDate.String(), paid); err != nil {
			t.Fatalf("insert occurrence: %v", err)
		}
	}
}
