package postgres

import "strings"

// escapeLike экранирует спецсимволы LIKE/ILIKE (\, %, _), чтобы пользовательский
// ввод искался как литеральная подстрока. Использовать только вместе с
// `ESCAPE '\'` в запросе. Порядок важен: сначала сам символ экранирования.
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}
