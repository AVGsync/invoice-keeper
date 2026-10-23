package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/AVGsync/invoice-keeper/backend/internal/model"
	"github.com/lib/pq"
)

type PersonaRepository struct {
	database *DB
}

// personaSelect — персона и число пользователей, которым она назначена.
const personaSelect = `
	SELECT p.id, p.name, p.description, p.created_at,
	       (SELECT COUNT(*) FROM users u WHERE u.persona_id = p.id)
	FROM personas p`

func scanPersona(row interface{ Scan(dest ...any) error }) (model.Persona, error) {
	var p model.Persona
	err := row.Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt, &p.UsersCount)
	return p, err
}

// personaWriteError переводит нарушение уникальности названия в доменную ошибку.
func personaWriteError(op string, err error) error {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == "23505" {
		return model.ErrPersonaNameTaken
	}
	return fmt.Errorf("persona repository: %s: %w", op, err)
}

func (r *PersonaRepository) GetAllPersonas(ctx context.Context) ([]model.Persona, error) {
	rows, err := r.database.db.QueryContext(ctx, personaSelect+`
		ORDER BY lower(p.name)
	`)
	if err != nil {
		return nil, fmt.Errorf("persona repository: get all personas: %w", err)
	}
	defer rows.Close()

	personas := []model.Persona{}
	for rows.Next() {
		p, err := scanPersona(rows)
		if err != nil {
			return nil, fmt.Errorf("persona repository: get all personas: %w", err)
		}
		personas = append(personas, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("persona repository: get all personas: %w", err)
	}
	return personas, nil
}

func (r *PersonaRepository) GetPersonaByID(id string, ctx context.Context) (model.Persona, error) {
	p, err := scanPersona(r.database.db.QueryRowContext(ctx, personaSelect+`
		WHERE p.id = $1
	`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return p, model.ErrPersonaNotFound
	}
	if err != nil {
		return p, fmt.Errorf("persona repository: get persona by id: %w", err)
	}
	return p, nil
}

func (r *PersonaRepository) CreatePersona(p model.Persona, ctx context.Context) (model.Persona, error) {
	err := r.database.db.QueryRowContext(ctx, `
		INSERT INTO personas (name, description)
		VALUES ($1, $2)
		RETURNING id, created_at
	`, p.Name, p.Description).Scan(&p.ID, &p.CreatedAt)
	if err != nil {
		return p, personaWriteError("create persona", err)
	}
	return p, nil
}

func (r *PersonaRepository) UpdatePersona(p model.Persona, ctx context.Context) error {
	res, err := r.database.db.ExecContext(ctx, `
		UPDATE personas
		SET name = $1, description = $2
		WHERE id = $3
	`, p.Name, p.Description, p.ID)
	if err != nil {
		return personaWriteError("update persona", err)
	}
	if n, err := res.RowsAffected(); err != nil {
		return fmt.Errorf("persona repository: update persona: %w", err)
	} else if n == 0 {
		return model.ErrPersonaNotFound
	}
	return nil
}

// DeletePersona удаляет персону; у пользователей с ней persona_id становится NULL.
func (r *PersonaRepository) DeletePersona(id string, ctx context.Context) error {
	res, err := r.database.db.ExecContext(ctx, `
		DELETE FROM personas
		WHERE id = $1
	`, id)
	if err != nil {
		return fmt.Errorf("persona repository: delete persona: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil {
		return fmt.Errorf("persona repository: delete persona: %w", err)
	} else if n == 0 {
		return model.ErrPersonaNotFound
	}
	return nil
}
