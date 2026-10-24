package apiserver

import (
	"github.com/AVGsync/invoice-keeper/backend/internal/transport/http/handler"
	"github.com/AVGsync/invoice-keeper/backend/internal/transport/http/middleware"

	"github.com/go-chi/chi/v5"
)

// mountAudit монтирует ленты журнала действий.
func (s *APIServer) mountAudit(r chi.Router, mw *middleware.Middleware, h *handler.AuditHandler) {
	r.Get("/me/activity", h.MyActivity())
	r.Get("/audit-log/actions", h.Actions())
	r.With(mw.IsAdmin).Get("/audit-log", h.AuditLog())
}
