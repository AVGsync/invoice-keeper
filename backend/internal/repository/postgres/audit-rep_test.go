//go:build integration

package postgres

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/AVGsync/invoice-keeper/backend/internal/audit"
	"github.com/AVGsync/invoice-keeper/backend/internal/model"
)

// Контракт журнала действий (трек C) против реальной таблицы audit_log.
//
// ВНИМАНИЕ: тест ПИШЕТ и УДАЛЯЕТ строки по адресу TEST_DATABASE_URL. Удаляются
// только свои строки (request_id LIKE 'audt_%' и пользователь-фикстура
// cccccccc-…), не TRUNCATE. НЕ указывайте сюда рабочую БД. Запуск:
//
//	TEST_DATABASE_URL=postgres://.../invoice_keeper_test?sslmode=disable \
//	    go test -tags integration ./internal/repository/postgres/

const (
	auditUser = "cccccccc-0000-4000-8000-000000000011"
)

func auditDB(t *testing.T) (*DB, *sql.DB) {
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
		t.Fatalf("ping: %v", err)
	}
	var n int
	if err := raw.QueryRow(`SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'audit_log'`).Scan(&n); err != nil {
		t.Fatalf("check table: %v", err)
	}
	if n == 0 {
		t.Skip("таблицы audit_log ещё нет (миграция 000012 не накатана)")
	}
	return &DB{db: raw}, raw
}

func auditCleanup(t *testing.T, raw *sql.DB) {
	t.Helper()
	if _, err := raw.Exec(`DELETE FROM audit_log WHERE request_id LIKE 'audt_%'`); err != nil {
		t.Fatalf("cleanup audit_log: %v", err)
	}
	if _, err := raw.Exec(`DELETE FROM users WHERE id = $1`, auditUser); err != nil {
		t.Fatalf("cleanup users: %v", err)
	}
}

func sp(s string) *string { return &s }

func TestAuditRepository(t *testing.T) {
	db, raw := auditDB(t)
	auditCleanup(t, raw)
	t.Cleanup(func() { auditCleanup(t, raw) })

	// Пользователь-актор для JOIN имени.
	if _, err := raw.Exec(
		`INSERT INTO users (id, full_name, login, password_hash, role) VALUES ($1, 'Audit Olga', 'audt_olga', 'x', 'admin')`,
		auditUser,
	); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	repo := db.Audit()
	ctx := context.Background()

	rows := []audit.Row{
		{
			ActorID: sp(auditUser), ActorRole: sp("admin"), Channel: "web",
			Action: "recurring.update", Outcome: "success",
			EntityType: sp("recurring_payment"), EntityID: sp("rp-audt-1"),
			Changes: []byte(`{"amount":{"from":100,"to":200}}`),
			IP:      sp("203.0.113.5"), UserAgent: sp("UA"), RequestID: sp("audt_1"),
		},
		{
			ActorID: sp(auditUser), ActorRole: sp("admin"), Channel: "miniapp",
			Action: "occurrence.confirm", Outcome: "success",
			EntityType: sp("occurrence"), EntityID: sp("occ-audt-1"),
			Payload:   []byte(`{"paid_late":true}`),
			RequestID: sp("audt_2"),
		},
		{
			Channel: "web", Action: "auth.login", Outcome: "failure",
			EntityType: sp("user"),
			Payload:    []byte(`{"login":"ghost","reason":"invalid_credentials"}`),
			RequestID:  sp("audt_3"),
		},
	}
	for i, r := range rows {
		if err := repo.Insert(ctx, r); err != nil {
			t.Fatalf("insert row %d: %v", i, err)
		}
	}

	base := model.AuditFilter{Order: "desc", Limit: 50, Offset: 0}

	t.Run("filter by actor with joined name", func(t *testing.T) {
		f := base
		f.ActorIDs = []string{auditUser}
		got, total, err := repo.List(ctx, f)
		if err != nil {
			t.Fatal(err)
		}
		if total != 2 || len(got) != 2 {
			t.Fatalf("total=%d len=%d, want 2/2", total, len(got))
		}
		for _, e := range got {
			if e.ActorName == nil || *e.ActorName != "Audit Olga" {
				t.Errorf("actor name not joined: %v", e.ActorName)
			}
		}
	})

	t.Run("action prefix", func(t *testing.T) {
		f := base
		f.Actions = []string{"recurring.*"}
		got, total, err := repo.List(ctx, f)
		if err != nil {
			t.Fatal(err)
		}
		if total != 1 || got[0].Action != "recurring.update" {
			t.Fatalf("prefix filter wrong: total=%d %+v", total, got)
		}
		if got[0].IP == nil || *got[0].IP != "203.0.113.5" {
			t.Errorf("INET not read back: %v", got[0].IP)
		}
	})

	t.Run("channel filter", func(t *testing.T) {
		f := base
		f.Channels = []string{"miniapp"}
		got, total, _ := repo.List(ctx, f)
		if total != 1 || got[0].Action != "occurrence.confirm" {
			t.Fatalf("channel filter wrong: total=%d", total)
		}
	})

	t.Run("outcome failure has nil actor", func(t *testing.T) {
		f := base
		f.Outcome = "failure"
		got, total, _ := repo.List(ctx, f)
		if total != 1 {
			t.Fatalf("outcome filter wrong: total=%d", total)
		}
		if got[0].ActorID != nil {
			t.Errorf("failure row should have nil actor")
		}
	})

	t.Run("q searches entity and payload", func(t *testing.T) {
		f := base
		f.Query = "rp-audt-1"
		if _, total, _ := repo.List(ctx, f); total != 1 {
			t.Errorf("q by entity_id: total=%d want 1", total)
		}
		f.Query = "ghost"
		if _, total, _ := repo.List(ctx, f); total != 1 {
			t.Errorf("q by payload: total=%d want 1", total)
		}
	})

	t.Run("pagination keeps total", func(t *testing.T) {
		f := base
		f.ActorIDs = []string{auditUser}
		f.Limit = 1
		got, total, _ := repo.List(ctx, f)
		if total != 2 || len(got) != 1 {
			t.Fatalf("pagination wrong: total=%d len=%d", total, len(got))
		}
	})

	t.Run("delete user with audit rows (no FK)", func(t *testing.T) {
		if _, err := raw.Exec(`DELETE FROM users WHERE id = $1`, auditUser); err != nil {
			t.Fatalf("delete user with audit rows must succeed (no FK): %v", err)
		}
		f := base
		f.ActorIDs = []string{auditUser}
		got, total, _ := repo.List(ctx, f)
		if total != 2 {
			t.Fatalf("rows must survive user deletion: total=%d", total)
		}
		for _, e := range got {
			if e.ActorName != nil {
				t.Errorf("full_name must be nil after user deleted: %v", *e.ActorName)
			}
			if e.ActorID == nil || *e.ActorID != auditUser {
				t.Errorf("actor_id must survive: %v", e.ActorID)
			}
		}
	})
}
