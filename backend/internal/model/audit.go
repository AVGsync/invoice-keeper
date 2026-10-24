package model

import "time"

// AuditEntry — строка журнала действий, как её читает репозиторий (с присоединённым
// именем актора). Changes/Payload — сырой JSON из колонок jsonb (nil = NULL).
type AuditEntry struct {
	ID         int64
	CreatedAt  time.Time
	ActorID    *string
	ActorRole  *string
	ActorName  *string // users.full_name по actor_id; nil, если актора нет или он удалён
	Channel    string
	Action     string
	Outcome    string
	EntityType *string
	EntityID   *string
	Changes    []byte
	Payload    []byte
	IP         *string
	UserAgent  *string
	RequestID  *string
}

// AuditFilter — разобранные параметры чтения журнала. From/To — уже приведённые
// к границам временного окна (включая/исключая по правилу C.6). Actions может
// содержать точные коды и префиксы вида "recurring.*".
type AuditFilter struct {
	ActorIDs    []string
	Actions     []string
	EntityTypes []string
	EntityID    string
	Channels    []string
	Outcome     string
	From        *time.Time
	To          *time.Time
	Query       string
	Order       string // "asc" | "desc"
	Limit       int
	Offset      int
}
