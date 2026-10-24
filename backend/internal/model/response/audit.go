package response

import (
	"encoding/json"
	"time"

	"github.com/AVGsync/invoice-keeper/backend/internal/audit"
)

// AuditActor — актор записи журнала. null, если actor_id пуст; full_name null,
// если пользователь удалён (LEFT JOIN не нашёл имя).
type AuditActor struct {
	ID       string  `json:"id"`
	FullName *string `json:"full_name" extensions:"x-nullable"`
	Role     *string `json:"role" extensions:"x-nullable"`
}

// AuditLogItem — одна запись журнала наружу.
type AuditLogItem struct {
	ID          int64           `json:"id"`
	CreatedAt   time.Time       `json:"created_at"`
	Actor       *AuditActor     `json:"actor" extensions:"x-nullable"`
	Channel     string          `json:"channel"`
	Action      string          `json:"action"`
	ActionLabel string          `json:"action_label"`
	Outcome     string          `json:"outcome"`
	EntityType  *string         `json:"entity_type" extensions:"x-nullable"`
	EntityID    *string         `json:"entity_id" extensions:"x-nullable"`
	Changes     json.RawMessage `json:"changes" swaggertype:"object"`
	Payload     json.RawMessage `json:"payload" swaggertype:"object"`
	IP          *string         `json:"ip" extensions:"x-nullable"`
	UserAgent   *string         `json:"user_agent" extensions:"x-nullable"`
	RequestID   *string         `json:"request_id" extensions:"x-nullable"`
}

// AuditLogResponse — страница журнала с общим количеством по фильтру.
type AuditLogResponse struct {
	Items  []AuditLogItem `json:"items"`
	Total  int            `json:"total"`
	Limit  int            `json:"limit"`
	Offset int            `json:"offset"`
}

// AuditActionsResponse — каталог действий для фильтров UI.
type AuditActionsResponse struct {
	Items []audit.ActionInfo `json:"items"`
}
