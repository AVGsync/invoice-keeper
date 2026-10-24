package postgres

import (
	"fmt"
	"strings"
)

// whereBuilder собирает WHERE с позиционными параметрами $n.
type whereBuilder struct {
	clauses []string
	args    []any
}

// arg добавляет значение-аргумент и возвращает его плейсхолдер ("$1", "$2", …).
func (w *whereBuilder) arg(v any) string {
	w.args = append(w.args, v)
	return fmt.Sprintf("$%d", len(w.args))
}

// add добавляет предикат.
func (w *whereBuilder) add(clause string) {
	w.clauses = append(w.clauses, clause)
}

// sql возвращает "" (нет условий) или " WHERE c1 AND c2 …".
func (w *whereBuilder) sql() string {
	if len(w.clauses) == 0 {
		return ""
	}
	return " WHERE " + strings.Join(w.clauses, " AND ")
}
