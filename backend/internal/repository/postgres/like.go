package postgres

import "strings"

// escapeLike экранирует спецсимволы LIKE/ILIKE (\, %, _), чтобы пользовательский ввод искался как литеральная подстрока.
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}
