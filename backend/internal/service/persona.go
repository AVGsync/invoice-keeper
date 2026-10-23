package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/AVGsync/invoice-keeper/backend/internal/model"
	"github.com/AVGsync/invoice-keeper/backend/internal/model/request"
	"github.com/AVGsync/invoice-keeper/backend/internal/model/response"
)

type PersonaRepository interface {
	GetAllPersonas(ctx context.Context) ([]model.Persona, error)
	GetPersonaByID(id string, ctx context.Context) (model.Persona, error)
	CreatePersona(p model.Persona, ctx context.Context) (model.Persona, error)
	UpdatePersona(p model.Persona, ctx context.Context) error
	DeletePersona(id string, ctx context.Context) error
}

// PersonaService ведёт справочник персон (должностей), которые администратор
// назначает пользователям при создании.
type PersonaService struct {
	repository PersonaRepository
}

func NewPersonaService(repository PersonaRepository) *PersonaService {
	return &PersonaService{repository: repository}
}

// trimmedOrNil: пустое после обрезки описание храним как NULL.
func trimmedOrNil(s *string) *string {
	if s == nil {
		return nil
	}
	v := strings.TrimSpace(*s)
	if v == "" {
		return nil
	}
	return &v
}

func (s *PersonaService) GetAllPersonas(ctx context.Context) (response.PersonaListResponse, error) {
	personas, err := s.repository.GetAllPersonas(ctx)
	if err != nil {
		slog.Error("persona service: get all personas: failed to get personas", "error", err)
		return response.PersonaListResponse{}, fmt.Errorf("persona service: get all personas: %w", err)
	}
	return response.PersonaListResponse{Items: personas, Total: len(personas)}, nil
}

func (s *PersonaService) CreatePersona(req request.CreatePersonaRequest, ctx context.Context) (model.Persona, error) {
	persona, err := s.repository.CreatePersona(model.Persona{
		Name:        strings.TrimSpace(req.Name),
		Description: trimmedOrNil(req.Description),
	}, ctx)
	if err != nil {
		slog.Error("persona service: create persona: failed to create persona", "error", err)
		return model.Persona{}, fmt.Errorf("persona service: create persona: %w", err)
	}
	return persona, nil
}

func (s *PersonaService) UpdatePersona(id string, req request.UpdatePersonaRequest, ctx context.Context) (model.Persona, error) {
	persona, err := s.repository.GetPersonaByID(id, ctx)
	if err != nil {
		return model.Persona{}, fmt.Errorf("persona service: update persona: %w", err)
	}
	if req.Name != nil {
		persona.Name = strings.TrimSpace(*req.Name)
	}
	if req.Description != nil {
		persona.Description = trimmedOrNil(req.Description)
	}

	if err := s.repository.UpdatePersona(persona, ctx); err != nil {
		slog.Error("persona service: update persona: failed to update persona", "error", err)
		return model.Persona{}, fmt.Errorf("persona service: update persona: %w", err)
	}
	return persona, nil
}

func (s *PersonaService) DeletePersona(id string, ctx context.Context) error {
	if err := s.repository.DeletePersona(id, ctx); err != nil {
		slog.Error("persona service: delete persona: failed to delete persona", "error", err)
		return fmt.Errorf("persona service: delete persona: %w", err)
	}
	return nil
}
