package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/AVGsync/invoice-keeper/backend/internal/model"
	_ "github.com/AVGsync/invoice-keeper/backend/internal/model/request"
	_ "github.com/AVGsync/invoice-keeper/backend/internal/model/response"
	"github.com/lib/pq"
)

type UserRepository struct {
	database *DB
}

// userSelect — пользователь вместе с назначенной персоной (LEFT JOIN: персоны может не быть).
const userSelect = `
	SELECT u.id, u.full_name, u.login, u.password_hash, u.role, u.created_at, p.id, p.name,
	       tl.user_id IS NOT NULL, COALESCE(tl.tg_username, ''), tl.linked_at,
	       ns.user_id IS NOT NULL, COALESCE(ns.email_verified, FALSE)
	FROM users u
	LEFT JOIN personas p ON p.id = u.persona_id
	LEFT JOIN telegram_links tl ON tl.user_id = u.id
	LEFT JOIN user_notification_settings ns ON ns.user_id = u.id`

func scanUser(row interface{ Scan(dest ...any) error }) (model.User, error) {
	var (
		user        model.User
		personaID   *string
		personaName *string
	)
	err := row.Scan(
		&user.ID,
		&user.FullName,
		&user.Login,
		&user.PasswordHash,
		&user.Role,
		&user.CreatedAt,
		&personaID,
		&personaName,
		&user.TelegramLinked,
		&user.TelegramUsername,
		&user.TelegramLinkedAt,
		&user.EmailLinked,
		&user.EmailVerified,
	)
	if err == nil && personaID != nil && personaName != nil {
		user.Persona = &model.PersonaRef{ID: *personaID, Name: *personaName}
	}
	return user, err
}

func (r *UserRepository) GetUserByLogin(login string, ctx context.Context) (model.User, error) {
	user, err := scanUser(r.database.db.QueryRowContext(ctx, userSelect+`
		WHERE u.login = $1
	`, login))
	if err != nil {
		return user, fmt.Errorf("user repository: get user by login: %w", err)
	}
	return user, nil
}

func (r *UserRepository) GetUserByID(id string, ctx context.Context) (model.User, error) {
	user, err := scanUser(r.database.db.QueryRowContext(ctx, userSelect+`
		WHERE u.id = $1
	`, id))
	if err != nil {
		return user, fmt.Errorf("user repository: get user by ID: %w", err)
	}
	return user, nil
}

func (r *UserRepository) AddRefreshToken(tokenHash string, expiresAt time.Time, createdAt time.Time, revokedAt *time.Time, ctx context.Context) error {
	_, err := r.database.db.ExecContext(ctx, `
		INSERT INTO refresh_tokens (token_hash, expires_at, created_at, revoked_at)
		VALUES ($1, $2, $3, $4)
	`, tokenHash, expiresAt, createdAt, revokedAt)
	if err != nil {
		return fmt.Errorf("user repository: add refresh token: %w", err)
	}
	return nil
}

func (r *UserRepository) GetRefreshTokenByHash(tokenHash string, ctx context.Context) (model.RefreshToken, error) {
	var refreshToken model.RefreshToken
	err := r.database.db.QueryRowContext(ctx, `
		SELECT id, token_hash, expires_at, created_at, revoked_at
		FROM refresh_tokens
		WHERE token_hash = $1
	`, tokenHash).Scan(
		&refreshToken.ID,
		&refreshToken.TokenHash,
		&refreshToken.ExpiresAt,
		&refreshToken.CreatedAt,
		&refreshToken.RevokedAt,
	)
	if err != nil {
		return refreshToken, fmt.Errorf("user repository: get refresh token by hash: %w", err)
	}
	return refreshToken, nil
}

func (r *UserRepository) RevokeRefreshToken(tokenHash string, ctx context.Context) error {
	_, err := r.database.db.ExecContext(ctx, `
		UPDATE refresh_tokens
		SET revoked_at = $1
		WHERE token_hash = $2
	`, time.Now(), tokenHash)
	if err != nil {
		return fmt.Errorf("user repository: revoke refresh token: %w", err)
	}
	return nil
}

func (r *UserRepository) CreateUser(user model.User, ctx context.Context) error {
	var personaID *string
	if user.Persona != nil {
		personaID = &user.Persona.ID
	}

	var id string
	err := r.database.db.QueryRowContext(ctx, `
		INSERT INTO users (full_name, login, password_hash, role, persona_id)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, user.FullName, user.Login, user.PasswordHash, user.Role, personaID).Scan(&id)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) {
			switch pqErr.Code {
			case "23505":
				return model.ErrLoginTaken
			case "23503": // persona_id ссылается на несуществующую персону
				return model.ErrPersonaNotFound
			}
		}
		return fmt.Errorf("user repository: create user: %w", err)
	}
	return nil
}

func (r *UserRepository) GetAllUsers(ctx context.Context) ([]model.User, error) {
	rows, err := r.database.db.QueryContext(ctx, userSelect+`
		ORDER BY u.created_at, u.login
	`)
	if err != nil {
		return nil, fmt.Errorf("user repository: get all users: %w", err)
	}
	defer rows.Close()

	users := []model.User{}
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("user repository: get all users: %w", err)
		}
		users = append(users, user)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("user repository: get all users: %w", err)
	}

	return users, nil
}

func (r *UserRepository) CountUsersByRole(role string, ctx context.Context) (int, error) {
	var count int
	err := r.database.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM users
		WHERE role = $1
	`, role).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("user repository: count users by role: %w", err)
	}
	return count, nil
}

func (r *UserRepository) DeleteUserByID(id string, ctx context.Context) error {
	res, err := r.database.db.ExecContext(ctx, `
		DELETE FROM users
		WHERE id = $1
	`, id)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23503" {
			return model.ErrUserInUse
		}
		return fmt.Errorf("user repository: delete user by id: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("user repository: delete user by id: %w", err)
	}
	if rows == 0 {
		return model.ErrUserNotFound
	}
	return nil
}

func (r *UserRepository) UpdateUser(user model.User, ctx context.Context) error {
	var personaID *string
	if user.Persona != nil {
		personaID = &user.Persona.ID
	}
	_, err := r.database.db.ExecContext(ctx, `
		UPDATE users
		SET full_name = $1, login = $2, password_hash = $3, role = $4, persona_id = $5
		WHERE id = $6
	`, user.FullName, user.Login, user.PasswordHash, user.Role, personaID, user.ID)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) {
			switch pqErr.Code {
			case "23505":
				return model.ErrLoginTaken
			case "23503":
				return model.ErrPersonaNotFound
			}
		}
		return fmt.Errorf("user repository: update user: %w", err)
	}
	return nil
}
