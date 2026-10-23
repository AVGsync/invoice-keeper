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

func (r *UserRepository) GetUserByLogin(login string, ctx context.Context) (model.User, error) {
	var user model.User
	err := r.database.db.QueryRowContext(ctx, `
		SELECT id, full_name, login, password_hash, role
		FROM users
		WHERE login = $1
	`, login).Scan(
		&user.ID,
		&user.FullName,
		&user.Login,
		&user.PasswordHash,
		&user.Role,
	)
	if err != nil {
		return user, fmt.Errorf("user repository: get user by login: %w", err)
	}
	return user, nil
}

func (r *UserRepository) GetUserByID(id string, ctx context.Context) (model.User, error) {
	var user model.User
	err := r.database.db.QueryRowContext(ctx, `
		SELECT id, full_name, login, password_hash, role, created_at
		FROM users
		WHERE id = $1
	`, id).Scan(
		&user.ID,
		&user.FullName,
		&user.Login,
		&user.PasswordHash,
		&user.Role,
		&user.CreatedAt,
	)
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
	var id string
	err := r.database.db.QueryRowContext(ctx, `
		INSERT INTO users (full_name, login, password_hash, role)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`, user.FullName, user.Login, user.PasswordHash, user.Role).Scan(&id)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return model.ErrLoginTaken
		}
		return fmt.Errorf("user repository: create user: %w", err)
	}
	return nil
}

func (r *UserRepository) GetAllUsers(ctx context.Context) ([]model.User, error) {
	rows, err := r.database.db.QueryContext(ctx, `
		SELECT id, full_name, login, password_hash, role, created_at
		FROM users
	`)
	if err != nil {
		return nil, fmt.Errorf("user repository: get all users: %w", err)
	}
	defer rows.Close()

	users := []model.User{}
	for rows.Next() {
		var user model.User
		err := rows.Scan(
			&user.ID,
			&user.FullName,
			&user.Login,
			&user.PasswordHash,
			&user.Role,
			&user.CreatedAt,
		)
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
	_, err := r.database.db.ExecContext(ctx, `
		UPDATE users
		SET full_name = $1, login = $2, password_hash = $3, role = $4
		WHERE id = $5
	`, user.FullName, user.Login, user.PasswordHash, user.Role, user.ID)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return model.ErrLoginTaken
		}
		return fmt.Errorf("user repository: update user: %w", err)
	}
	return nil
}
