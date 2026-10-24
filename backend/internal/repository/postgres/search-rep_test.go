//go:build integration

package postgres

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/AVGsync/invoice-keeper/backend/internal/clock"
	"github.com/AVGsync/invoice-keeper/backend/internal/model"
	"github.com/AVGsync/invoice-keeper/backend/internal/service"
	"github.com/lib/pq"
)

// Контракт серверного поиска (трек B) против реальных таблиц платежей.
//
// ВНИМАНИЕ: тест ПИШЕТ и УДАЛЯЕТ строки фикстур в БД по адресу TEST_DATABASE_URL.
// Удаляются только строки с префиксом UUID фикстур B (bbbbbbbb-…) и логинами
// srch_* — не TRUNCATE. НЕ указывайте сюда рабочую БД. Запуск:
//
//	TEST_DATABASE_URL=postgres://.../invoice_keeper_test?sslmode=disable \
//	    go test -tags integration ./internal/repository/postgres/

const (
	searchToday = "2026-06-15"

	fxPersona = "bbbbbbbb-0000-4000-8000-000000000001"
	fxUserA   = "bbbbbbbb-0000-4000-8000-000000000011"
	fxUserB   = "bbbbbbbb-0000-4000-8000-000000000012"

	fxRP1 = "bbbbbbbb-0000-4000-8000-000000000021" // active monthly USD, Мария (A)
	fxRP2 = "bbbbbbbb-0000-4000-8000-000000000022" // active weekly RUB, Иван (B)
	fxRP3 = "bbbbbbbb-0000-4000-8000-000000000023" // stopped monthly USD, Мария (A)

	fxPOverdue    = "bbbbbbbb-0000-4000-8000-000000000031" // USD RP1 overdue
	fxPPending    = "bbbbbbbb-0000-4000-8000-000000000032" // USD RP1 pending
	fxPPaidLate   = "bbbbbbbb-0000-4000-8000-000000000033" // RUB RP2 paid (late)
	fxPEnded      = "bbbbbbbb-0000-4000-8000-000000000034" // USD RP3 ended
	fxPPaidOnTime = "bbbbbbbb-0000-4000-8000-000000000035" // RUB RP2 paid
	fxPPendingRUB = "bbbbbbbb-0000-4000-8000-000000000036" // RUB RP2 pending
)

func searchDB(t *testing.T) *DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL не задан")
	}
	raw, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := raw.Ping(); err != nil {
		raw.Close()
		t.Fatalf("ping: %v", err)
	}
	t.Cleanup(func() { raw.Close() })
	return &DB{db: raw}
}

func cleanSearchFixtures(t *testing.T, db *sql.DB) {
	t.Helper()
	ctx := context.Background()
	payIDs := pq.Array([]string{fxPOverdue, fxPPending, fxPPaidLate, fxPEnded, fxPPaidOnTime, fxPPendingRUB})
	rpIDs := pq.Array([]string{fxRP1, fxRP2, fxRP3})
	userIDs := pq.Array([]string{fxUserA, fxUserB})
	for _, stmt := range []struct {
		sql  string
		args []any
	}{
		{`DELETE FROM payments WHERE id = ANY($1)`, []any{payIDs}},
		{`DELETE FROM user_recurrent_payments WHERE recurrent_payment_id = ANY($1)`, []any{rpIDs}},
		{`DELETE FROM recurrent_payments WHERE id = ANY($1)`, []any{rpIDs}},
		{`DELETE FROM users WHERE id = ANY($1)`, []any{userIDs}},
		{`DELETE FROM personas WHERE id = $1`, []any{fxPersona}},
	} {
		if _, err := db.ExecContext(ctx, stmt.sql, stmt.args...); err != nil {
			t.Fatalf("clean fixtures (%s): %v", stmt.sql, err)
		}
	}
}

func seedSearchFixtures(t *testing.T, db *sql.DB) {
	t.Helper()
	ctx := context.Background()
	cleanSearchFixtures(t, db)

	exec := func(q string, args ...any) {
		if _, err := db.ExecContext(ctx, q, args...); err != nil {
			t.Fatalf("seed (%s): %v", q, err)
		}
	}

	exec(`INSERT INTO personas (id, name) VALUES ($1, 'Поисковая персона')`, fxPersona)
	exec(`INSERT INTO users (id, full_name, login, password_hash, role, persona_id)
		VALUES ($1, 'Мария Поиск', 'srch_a', 'x', 'user', $2)`, fxUserA, fxPersona)
	exec(`INSERT INTO users (id, full_name, login, password_hash, role)
		VALUES ($1, 'Иван Другой', 'srch_b', 'x', 'user')`, fxUserB)

	// Шаблоны. RP3 остановлен.
	exec(`INSERT INTO recurrent_payments (id, amount, currency, frequency, "interval", description, day_offset, start_date)
		VALUES ($1, 100000, 'USD', 'monthly', 1, 'Netflix', 1, '2026-01-01'::date)`, fxRP1)
	exec(`INSERT INTO recurrent_payments (id, amount, currency, frequency, "interval", description, day_offset, start_date)
		VALUES ($1, 50000, 'RUB', 'weekly', 1, 'Office Rent', 1, '2026-01-01'::date)`, fxRP2)
	exec(`INSERT INTO recurrent_payments (id, amount, currency, frequency, "interval", description, day_offset, start_date, stopped_at)
		VALUES ($1, 20000, 'USD', 'monthly', 1, 'Old Service', 1, '2026-01-01'::date, NOW())`, fxRP3)

	exec(`INSERT INTO user_recurrent_payments (user_id, recurrent_payment_id) VALUES ($1, $2)`, fxUserA, fxRP1)
	exec(`INSERT INTO user_recurrent_payments (user_id, recurrent_payment_id) VALUES ($1, $2)`, fxUserB, fxRP2)
	exec(`INSERT INTO user_recurrent_payments (user_id, recurrent_payment_id) VALUES ($1, $2)`, fxUserA, fxRP3)

	// Периоды: 4 статуса в 2 валютах. due относительно today=2026-06-15.
	// overdue USD
	exec(`INSERT INTO payments (id, user_id, recurrent_payment_id, amount, currency, description, due_date, status)
		VALUES ($1, $2, $3, 100000, 'USD', '50% скидка', '2026-06-01'::date, 'overdue')`, fxPOverdue, fxUserA, fxRP1)
	// pending USD
	exec(`INSERT INTO payments (id, user_id, recurrent_payment_id, amount, currency, description, due_date, status)
		VALUES ($1, $2, $3, 120000, 'USD', 'Netflix июль', '2026-06-30'::date, 'pending')`, fxPPending, fxUserA, fxRP1)
	// paid late RUB
	exec(`INSERT INTO payments (id, user_id, recurrent_payment_id, amount, currency, description, due_date, paid_date, paid_at, status)
		VALUES ($1, $2, $3, 50000, 'RUB', 'Office Rent май', '2026-05-01'::date, '2026-05-10'::date, NOW(), 'paid')`, fxPPaidLate, fxUserB, fxRP2)
	// ended USD (шаблон остановлен), неоплачен, due в прошлом
	exec(`INSERT INTO payments (id, user_id, recurrent_payment_id, amount, currency, description, due_date, status)
		VALUES ($1, $2, $3, 20000, 'USD', 'Old Service', '2026-06-10'::date, 'pending')`, fxPEnded, fxUserA, fxRP3)
	// paid on time RUB
	exec(`INSERT INTO payments (id, user_id, recurrent_payment_id, amount, currency, description, due_date, paid_date, paid_at, status)
		VALUES ($1, $2, $3, 50000, 'RUB', 'Office Rent июнь', '2026-06-10'::date, '2026-06-10'::date, NOW(), 'paid')`, fxPPaidOnTime, fxUserB, fxRP2)
	// pending RUB
	exec(`INSERT INTO payments (id, user_id, recurrent_payment_id, amount, currency, description, due_date, status)
		VALUES ($1, $2, $3, 50000, 'RUB', 'Office Rent июль', '2026-07-01'::date, 'pending')`, fxPPendingRUB, fxUserB, fxRP2)

	t.Cleanup(func() { cleanSearchFixtures(t, db) })
}

// ids собирает id найденных периодов в множество для проверки состава.
func occurrenceIDs(payments []model.Payment) map[string]bool {
	set := make(map[string]bool, len(payments))
	for _, p := range payments {
		set[p.ID] = true
	}
	return set
}

func TestSearchPaymentsFilters(t *testing.T) {
	db := searchDB(t)
	seedSearchFixtures(t, db.db)
	repo := db.Payment()
	ctx := context.Background()
	today, _ := clock.ParseDate(searchToday)

	base := func() model.OccurrenceFilter {
		return model.OccurrenceFilter{Sort: "due_date", Order: "asc", Limit: 50, Offset: 0}
	}

	t.Run("status overdue", func(t *testing.T) {
		f := base()
		f.Statuses = []string{"overdue"}
		got, total, err := repo.SearchPayments(ctx, f, searchToday)
		if err != nil {
			t.Fatal(err)
		}
		set := occurrenceIDs(got)
		if total != 1 || !set[fxPOverdue] {
			t.Fatalf("overdue: total=%d ids=%v", total, set)
		}
		// Паритет эффективного статуса.
		for _, p := range got {
			if st, _ := service.EffectiveStatus(p, today); st != "overdue" {
				t.Fatalf("effective status parity: %s = %s", p.ID, st)
			}
		}
	})

	t.Run("status pending both currencies", func(t *testing.T) {
		f := base()
		f.Statuses = []string{"pending"}
		got, total, err := repo.SearchPayments(ctx, f, searchToday)
		if err != nil {
			t.Fatal(err)
		}
		set := occurrenceIDs(got)
		if total != 2 || !set[fxPPending] || !set[fxPPendingRUB] {
			t.Fatalf("pending: total=%d ids=%v", total, set)
		}
	})

	t.Run("status ended", func(t *testing.T) {
		f := base()
		f.Statuses = []string{"ended"}
		got, total, err := repo.SearchPayments(ctx, f, searchToday)
		if err != nil {
			t.Fatal(err)
		}
		set := occurrenceIDs(got)
		if total != 1 || !set[fxPEnded] {
			t.Fatalf("ended: total=%d ids=%v", total, set)
		}
		for _, p := range got {
			if st, _ := service.EffectiveStatus(p, today); st != model.ShownStatusEnded {
				t.Fatalf("ended parity: %s = %s", p.ID, st)
			}
		}
	})

	t.Run("status paid", func(t *testing.T) {
		f := base()
		f.Statuses = []string{"paid"}
		_, total, err := repo.SearchPayments(ctx, f, searchToday)
		if err != nil {
			t.Fatal(err)
		}
		if total != 2 {
			t.Fatalf("paid: total=%d", total)
		}
	})

	t.Run("currency USD", func(t *testing.T) {
		f := base()
		f.Currencies = []string{"USD"}
		got, total, err := repo.SearchPayments(ctx, f, searchToday)
		if err != nil {
			t.Fatal(err)
		}
		set := occurrenceIDs(got)
		if total != 3 || !set[fxPOverdue] || !set[fxPPending] || !set[fxPEnded] {
			t.Fatalf("USD: total=%d ids=%v", total, set)
		}
	})

	t.Run("assignee_id A", func(t *testing.T) {
		f := base()
		f.AssigneeIDs = []string{fxUserA}
		_, total, err := repo.SearchPayments(ctx, f, searchToday)
		if err != nil {
			t.Fatal(err)
		}
		if total != 3 {
			t.Fatalf("assignee A: total=%d", total)
		}
	})

	t.Run("persona filter", func(t *testing.T) {
		f := base()
		f.PersonaIDs = []string{fxPersona}
		_, total, err := repo.SearchPayments(ctx, f, searchToday)
		if err != nil {
			t.Fatal(err)
		}
		if total != 3 {
			t.Fatalf("persona: total=%d", total)
		}
	})

	t.Run("amount range single currency", func(t *testing.T) {
		f := base()
		f.Currencies = []string{"USD"}
		min := int64(110000)
		f.AmountMin = &min
		_, total, err := repo.SearchPayments(ctx, f, searchToday)
		if err != nil {
			t.Fatal(err)
		}
		if total != 1 { // только pending USD 120000
			t.Fatalf("amount_min: total=%d", total)
		}
	})

	t.Run("q literal percent escaped", func(t *testing.T) {
		// '50%' в описании есть, но '%' в запросе экранируется → как литерал.
		f := base()
		f.Query = "50%"
		_, total, err := repo.SearchPayments(ctx, f, searchToday)
		if err != nil {
			t.Fatal(err)
		}
		if total != 1 {
			t.Fatalf("q '50%%' literal: total=%d", total)
		}
		// 'Off%' не должно совпасть ни с чем (нет литерального '%' в Office).
		f.Query = "Off%"
		_, total, err = repo.SearchPayments(ctx, f, searchToday)
		if err != nil {
			t.Fatal(err)
		}
		if total != 0 {
			t.Fatalf("q 'Off%%' must not match: total=%d", total)
		}
	})

	t.Run("q by assignee name", func(t *testing.T) {
		f := base()
		f.Query = "Мария"
		_, total, err := repo.SearchPayments(ctx, f, searchToday)
		if err != nil {
			t.Fatal(err)
		}
		if total != 3 { // все периоды Марии (A)
			t.Fatalf("q by name: total=%d", total)
		}
	})

	t.Run("sort amount asc/desc", func(t *testing.T) {
		f := base()
		f.Currencies = []string{"USD"}
		f.Sort = "amount"
		f.Order = "asc"
		asc, _, err := repo.SearchPayments(ctx, f, searchToday)
		if err != nil {
			t.Fatal(err)
		}
		f.Order = "desc"
		desc, _, err := repo.SearchPayments(ctx, f, searchToday)
		if err != nil {
			t.Fatal(err)
		}
		if len(asc) < 2 || len(desc) < 2 {
			t.Fatalf("sort: not enough rows")
		}
		if asc[0].Amount > asc[len(asc)-1].Amount {
			t.Fatalf("asc not ascending")
		}
		if desc[0].Amount < desc[len(desc)-1].Amount {
			t.Fatalf("desc not descending")
		}
	})

	t.Run("total with offset past end", func(t *testing.T) {
		f := base()
		f.Offset = 1000
		got, total, err := repo.SearchPayments(ctx, f, searchToday)
		if err != nil {
			t.Fatal(err)
		}
		if total != 6 {
			t.Fatalf("total should count all 6 regardless of offset: %d", total)
		}
		if len(got) != 0 {
			t.Fatalf("offset past end should yield no rows: %d", len(got))
		}
	})
}

func TestSearchRecurrentPayments(t *testing.T) {
	db := searchDB(t)
	seedSearchFixtures(t, db.db)
	repo := db.RecurrentPayment()
	ctx := context.Background()

	base := func() model.RecurringFilter {
		return model.RecurringFilter{Sort: "description", Order: "asc", Limit: 50, Statuses: []string{"active"}}
	}

	t.Run("default only active", func(t *testing.T) {
		f := base()
		got, total, err := repo.SearchRecurrentPayments(ctx, f, searchToday)
		if err != nil {
			t.Fatal(err)
		}
		if total != 2 {
			t.Fatalf("active only: total=%d", total)
		}
		for _, r := range got {
			if r.StoppedAt != nil {
				t.Fatalf("active filter returned stopped %s", r.ID)
			}
		}
	})

	t.Run("include stopped", func(t *testing.T) {
		f := base()
		f.Statuses = []string{"active", "stopped"}
		_, total, err := repo.SearchRecurrentPayments(ctx, f, searchToday)
		if err != nil {
			t.Fatal(err)
		}
		if total != 3 {
			t.Fatalf("include stopped: total=%d", total)
		}
	})

	t.Run("next_due populated", func(t *testing.T) {
		f := base()
		got, _, err := repo.SearchRecurrentPayments(ctx, f, searchToday)
		if err != nil {
			t.Fatal(err)
		}
		byID := map[string]model.RecurringSearchRow{}
		for _, r := range got {
			byID[r.ID] = r
		}
		rp1 := byID[fxRP1]
		if rp1.NextDueDate == nil || *rp1.NextDueDate != "2026-06-30" {
			t.Fatalf("RP1 next_due: %v", rp1.NextDueDate)
		}
		rp2 := byID[fxRP2]
		if rp2.NextDueDate == nil || *rp2.NextDueDate != "2026-07-01" {
			t.Fatalf("RP2 next_due: %v", rp2.NextDueDate)
		}
	})

	t.Run("stopped has no next_due and status", func(t *testing.T) {
		f := base()
		f.Statuses = []string{"stopped"}
		got, total, err := repo.SearchRecurrentPayments(ctx, f, searchToday)
		if err != nil {
			t.Fatal(err)
		}
		if total != 1 {
			t.Fatalf("stopped: total=%d", total)
		}
		if got[0].ID != fxRP3 || got[0].NextDueDate != nil || got[0].StoppedAt == nil {
			t.Fatalf("stopped row: %#v", got[0])
		}
	})

	t.Run("frequency filter", func(t *testing.T) {
		f := base()
		f.Frequencies = []string{"weekly"}
		got, total, err := repo.SearchRecurrentPayments(ctx, f, searchToday)
		if err != nil {
			t.Fatal(err)
		}
		if total != 1 || got[0].ID != fxRP2 {
			t.Fatalf("weekly: total=%d", total)
		}
	})

	t.Run("currency filter", func(t *testing.T) {
		f := base()
		f.Statuses = []string{"active", "stopped"}
		f.Currencies = []string{"USD"}
		_, total, err := repo.SearchRecurrentPayments(ctx, f, searchToday)
		if err != nil {
			t.Fatal(err)
		}
		if total != 2 { // RP1 + RP3
			t.Fatalf("USD templates: total=%d", total)
		}
	})

	t.Run("q by assignee name", func(t *testing.T) {
		f := base()
		f.Statuses = []string{"active", "stopped"}
		f.Query = "Мария"
		_, total, err := repo.SearchRecurrentPayments(ctx, f, searchToday)
		if err != nil {
			t.Fatal(err)
		}
		if total != 2 { // RP1 и RP3 назначены Марии
			t.Fatalf("q name: total=%d", total)
		}
	})

	t.Run("next_due range", func(t *testing.T) {
		f := base()
		f.NextDueFrom = "2026-07-01"
		got, total, err := repo.SearchRecurrentPayments(ctx, f, searchToday)
		if err != nil {
			t.Fatal(err)
		}
		if total != 1 || got[0].ID != fxRP2 {
			t.Fatalf("next_due range: total=%d", total)
		}
	})

	t.Run("sort amount desc", func(t *testing.T) {
		f := base()
		f.Statuses = []string{"active", "stopped"}
		f.Sort = "amount"
		f.Order = "desc"
		got, _, err := repo.SearchRecurrentPayments(ctx, f, searchToday)
		if err != nil {
			t.Fatal(err)
		}
		for i := 1; i < len(got); i++ {
			if got[i-1].Amount < got[i].Amount {
				t.Fatalf("amount desc not sorted")
			}
		}
	})
}
