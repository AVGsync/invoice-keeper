package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/AVGsync/invoice-keeper/backend/internal/model"
)

// AuditLister читает журнал действий (репозиторий audit_log).
type AuditLister interface {
	List(ctx context.Context, f model.AuditFilter) ([]model.AuditEntry, int, error)
}

// AuditService отдаёт журнал действий на чтение. Запись журнала идёт мимо него —
// через audit.Logger прямо из доменных сервисов (решение C.1).
type AuditService struct {
	repo AuditLister
}

func NewAuditService(repo AuditLister) *AuditService {
	return &AuditService{repo: repo}
}

// List возвращает страницу журнала и общее число строк по фильтру.
func (s *AuditService) List(ctx context.Context, f model.AuditFilter) ([]model.AuditEntry, int, error) {
	entries, total, err := s.repo.List(ctx, f)
	if err != nil {
		slog.Error("audit service: list failed", "error", err)
		return nil, 0, fmt.Errorf("audit service: list: %w", err)
	}
	return entries, total, nil
}
