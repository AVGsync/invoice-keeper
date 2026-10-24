package audit

import (
	"strings"
)

// redactedValue — чем заменяется чувствительное значение.
const redactedValue = "[redacted]"

// maxStringRunes — до скольких рун обрезаются строковые значения в payload/changes.
const maxStringRunes = 256

// maxUserAgentRunes — предел длины user-agent (как в CHECK таблицы).
const maxUserAgentRunes = 512

// sensitiveSubstrings — ключи, СОДЕРЖАЩИЕ любую из этих подстрок (без учёта
// регистра), вырезаются целиком. Никогда не пишем пароли, хеши, токены, куки,
// заголовок авторизации, коды привязки и initData.
var sensitiveSubstrings = []string{
	"password", "token", "secret", "authorization", "cookie",
	"init_data", "initdata", "hash",
}

// sensitiveExact — ключи, совпадающие точно (без учёта регистра): код привязки.
var sensitiveExact = []string{"code"}

// isSensitiveKey сообщает, что значение под ключом нужно вырезать.
func isSensitiveKey(key string) bool {
	lower := strings.ToLower(key)
	for _, s := range sensitiveSubstrings {
		if strings.Contains(lower, s) {
			return true
		}
	}
	for _, e := range sensitiveExact {
		if lower == e {
			return true
		}
	}
	return false
}

// isEmailKey сообщает, что значение под ключом — email и его нужно маскировать.
func isEmailKey(key string) bool {
	return strings.ToLower(key) == "email"
}

// MaskEmail превращает "olga@example.com" в "o***@example.com". Без '@' или с
// пустой локальной частью маскируется как "***".
func MaskEmail(s string) string {
	s = strings.TrimSpace(s)
	at := strings.LastIndexByte(s, '@')
	if at <= 0 {
		return "***"
	}
	local := []rune(s[:at])
	domain := s[at:]
	return string(local[0]) + "***" + domain
}

// truncateRunes обрезает строку до n рун.
func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

// Redact рекурсивно очищает payload: вырезает чувствительные ключи, маскирует
// email, обрезает строки. Возвращает новую карту; исходная не меняется.
func Redact(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = redactValue(k, v)
	}
	return out
}

// redactValue применяет правило ключа к значению и рекурсивно спускается внутрь.
func redactValue(key string, v any) any {
	if isSensitiveKey(key) {
		return redactedValue
	}
	if isEmailKey(key) {
		if s, ok := v.(string); ok {
			return MaskEmail(s)
		}
	}
	switch t := v.(type) {
	case string:
		return truncateRunes(t, maxStringRunes)
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, nv := range t {
			out[k] = redactValue(k, nv)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, nv := range t {
			// У элементов массива нет ключа — чувствительность не применяется,
			// но строки всё равно обрезаются, а вложенные карты чистятся.
			out[i] = redactValue("", nv)
		}
		return out
	default:
		return v
	}
}

// redactChanges применяет те же правила к изменениям «было → стало».
func redactChanges(m map[string]Change) map[string]Change {
	if m == nil {
		return nil
	}
	out := make(map[string]Change, len(m))
	for field, ch := range m {
		out[field] = Change{
			From: redactValue(field, ch.From),
			To:   redactValue(field, ch.To),
		}
	}
	return out
}
