//go:build integration

package postgres

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/AVGsync/invoice-keeper/backend/internal/model"
)

// Контракт хранилища истории ассистента против реальной таблицы assistant_turns
// (миграция 000013). ПИШЕТ и УДАЛЯЕТ строки фикстур по TEST_DATABASE_URL —
// только свои UUID (dddddddd-…), не TRUNCATE. Запуск:
//
//	TEST_DATABASE_URL=postgres://.../invoice_keeper_test?sslmode=disable \
//	    go test -tags integration ./internal/repository/postgres/
const (
	histUser  = "dddddddd-0000-4000-8000-000000000011"
	histUser2 = "dddddddd-0000-4000-8000-000000000012"
)

func historyDB(t *testing.T) (*DB, *sql.DB) {
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
	if err := raw.QueryRow(`SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'assistant_turns'`).Scan(&n); err != nil {
		t.Fatalf("check table: %v", err)
	}
	if n == 0 {
		t.Skip("таблицы assistant_turns ещё нет (миграция 000013 не накатана)")
	}
	return &DB{db: raw}, raw
}

func historyCleanup(t *testing.T, raw *sql.DB) {
	t.Helper()
	// assistant_turns уйдут каскадом при удалении пользователей.
	if _, err := raw.Exec(`DELETE FROM users WHERE login LIKE 'hist_%'`); err != nil {
		t.Fatalf("cleanup users: %v", err)
	}
}

func seedHistUser(t *testing.T, raw *sql.DB, id, login string) {
	t.Helper()
	if _, err := raw.Exec(
		`INSERT INTO users (id, full_name, login, password_hash, role) VALUES ($1, 'Hist User', $2, 'x', 'user')`,
		id, login,
	); err != nil {
		t.Fatalf("seed user %s: %v", login, err)
	}
}

func TestAssistantHistoryRepository(t *testing.T) {
	db, raw := historyDB(t)
	historyCleanup(t, raw)
	t.Cleanup(func() { historyCleanup(t, raw) })

	seedHistUser(t, raw, histUser, "hist_main")
	seedHistUser(t, raw, histUser2, "hist_other")

	repo := db.AssistantHistory()
	ctx := context.Background()

	// Append нескольких ходов в разговор web.
	for i, st := range []string{"ok", "error", "partial", "ok"} {
		_, err := repo.Append(ctx, model.AssistantTurn{
			UserID: histUser, Channel: "web", Conversation: "web",
			AnswerID: "a", Question: "q", AnswerText: "txt",
			Status: st, Mode: "llm", Memory: []byte(`{"v":1}`),
		})
		if err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}
	// Ход другого разговора и другого пользователя.
	if _, err := repo.Append(ctx, model.AssistantTurn{
		UserID: histUser, Channel: "bot", Conversation: "tg:dm", AnswerID: "a", Question: "q2", Status: "ok",
	}); err != nil {
		t.Fatalf("append bot: %v", err)
	}
	if _, err := repo.Append(ctx, model.AssistantTurn{
		UserID: histUser2, Channel: "web", Conversation: "web", AnswerID: "a", Question: "qx", Status: "ok",
	}); err != nil {
		t.Fatalf("append other: %v", err)
	}

	// Count видит только свой разговор.
	if n, err := repo.Count(ctx, histUser, "web"); err != nil || n != 4 {
		t.Fatalf("count web = %d (err %v), want 4", n, err)
	}

	// ListRecent с фильтром статусов возвращает только ok|partial, DESC.
	okTurns, err := repo.ListRecent(ctx, histUser, "web", 10, []string{"ok", "partial"})
	if err != nil {
		t.Fatalf("list ok: %v", err)
	}
	if len(okTurns) != 3 {
		t.Fatalf("ok|partial turns = %d, want 3", len(okTurns))
	}
	for _, turn := range okTurns {
		if turn.Status == "error" {
			t.Fatal("error turn must be filtered out")
		}
	}

	// ListRecent без фильтра — все 4.
	all, err := repo.ListRecent(ctx, histUser, "web", 10, nil)
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(all) != 4 {
		t.Fatalf("all turns = %d, want 4", len(all))
	}

	// Prune оставляет keep самых свежих.
	if err := repo.Prune(ctx, histUser, "web", 2); err != nil {
		t.Fatalf("prune: %v", err)
	}
	if n, _ := repo.Count(ctx, histUser, "web"); n != 2 {
		t.Fatalf("after prune count = %d, want 2", n)
	}

	// DeleteConversation трогает только указанный разговор.
	if del, err := repo.DeleteConversation(ctx, histUser, "web"); err != nil || del != 2 {
		t.Fatalf("delete conversation = %d (err %v), want 2", del, err)
	}
	if n, _ := repo.Count(ctx, histUser, "tg:dm"); n != 1 {
		t.Fatalf("tg:dm must remain: %d", n)
	}

	// DeleteAllForUser убирает всё у пользователя, не трогая другого.
	if _, err := repo.DeleteAllForUser(ctx, histUser); err != nil {
		t.Fatalf("delete all: %v", err)
	}
	if n, _ := repo.Count(ctx, histUser, "tg:dm"); n != 0 {
		t.Fatalf("all must be gone: %d", n)
	}
	if n, _ := repo.Count(ctx, histUser2, "web"); n != 1 {
		t.Fatalf("other user untouched: %d", n)
	}

	// CHECK conversation отвергает мусор.
	if _, err := repo.Append(ctx, model.AssistantTurn{
		UserID: histUser2, Channel: "web", Conversation: "bogus", AnswerID: "a", Question: "q", Status: "ok",
	}); err == nil {
		t.Fatal("invalid conversation must violate CHECK")
	}

	// Каскад при удалении пользователя.
	if _, err := raw.Exec(`DELETE FROM users WHERE id = $1`, histUser2); err != nil {
		t.Fatalf("delete user: %v", err)
	}
	if n, _ := repo.Count(ctx, histUser2, "web"); n != 0 {
		t.Fatalf("cascade must remove turns: %d", n)
	}
}
