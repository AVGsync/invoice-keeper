package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/AVGsync/invoice-keeper/backend/internal/audit"
	"github.com/AVGsync/invoice-keeper/backend/internal/model"
	"github.com/AVGsync/invoice-keeper/backend/internal/model/request"
	"github.com/AVGsync/invoice-keeper/backend/internal/model/response"
)

var ErrInvalidCredentials = errors.New("invalid credentials")

// Через API заводятся только обычные пользователи: администратор задаётся в .env (см. AdminAccount) и через панель не управляется.
const userRole = "user"

type UserRepository interface {
	GetUserByLogin(login string, ctx context.Context) (model.User, error)
	AddRefreshToken(tokenHash string, expiresAt time.Time, createdAt time.Time, revokedAt *time.Time, ctx context.Context) error
	GetRefreshTokenByHash(tokenHash string, ctx context.Context) (model.RefreshToken, error)
	GetUserByID(id string, ctx context.Context) (model.User, error)
	RevokeRefreshToken(tokenHash string, ctx context.Context) error
	CreateUser(user model.User, ctx context.Context) error
	GetAllUsers(ctx context.Context) ([]model.User, error)
	DeleteUserByID(id string, ctx context.Context) error
	UpdateUser(user model.User, ctx context.Context) error
}

type Hasher interface {
	HashPassword(password string) (string, error)
	CheckPassword(plain, hashed string) bool
	HashRefreshToken(token string) (string, error)
	CheckRefreshToken(token string, hashed string) bool
}

type JWTManager interface {
	GenerateAccess(userID string, role string) (string, error)
	GenerateRefresh(userID string, role string) (string, error)
	ValidateRefresh(tokenStr string) (*model.Claims, error)
}

type UserService struct {
	repository UserRepository
	hasher     Hasher
	jwtManager JWTManager
	accessTTL  time.Duration
	refreshTTL time.Duration
	// audit — журнал действий; по умолчанию Nop (конструктор не меняется, тесты не ломаются).
	audit audit.Recorder
}

func NewUserService(repository UserRepository, hasher Hasher, jwtManager JWTManager, accessTTL, refreshTTL time.Duration) *UserService {
	return &UserService{
		repository: repository,
		hasher:     hasher,
		jwtManager: jwtManager,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
		audit:      audit.Nop{},
	}
}

// SetAuditRecorder подключает журнал действий.
func (s *UserService) SetAuditRecorder(r audit.Recorder) { s.audit = r }

// truncateLogin обрезает логин до n рун — логин в payload журнала не раздуваем.
func truncateLogin(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

// IssueAccessForUser выпускает только access-токен для пользователя userID.
func (s *UserService) IssueAccessForUser(ctx context.Context, userID string) (string, int, error) {
	user, err := s.repository.GetUserByID(userID, ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", 0, model.ErrUserNotFound
		}
		slog.Error("user service: issue access for user: failed to get user by ID", "error", err)
		return "", 0, fmt.Errorf("user service: issue access for user: %w", err)
	}

	token, err := s.jwtManager.GenerateAccess(user.ID, user.Role)
	if err != nil {
		slog.Error("user service: issue access for user: failed to generate access token", "error", err)
		return "", 0, fmt.Errorf("user service: issue access for user: %w", err)
	}

	return token, int(s.accessTTL.Seconds()), nil
}

func (s *UserService) AuthenticateUser(req request.LoginRequest, ctx context.Context) (string, string, error) {
	user, err := s.repository.GetUserByLogin(req.Login, ctx)
	if err != nil {
		slog.Error("user service: authenticate user: failed to get user by login", "error", err)
		s.recordLoginFailure(ctx, req.Login)
		return "", "", fmt.Errorf("user service: authenticate user: %w", err)
	}

	if !s.hasher.CheckPassword(req.Password, user.PasswordHash) {
		s.recordLoginFailure(ctx, req.Login)
		return "", "", ErrInvalidCredentials
	}

	tokenAccess, tokenRefresh, err := s.startSession(ctx, user)
	if err != nil {
		return "", "", err
	}

	s.audit.Record(ctx, audit.Entry{
		Action:     audit.ActionAuthLogin,
		ActorID:    user.ID,
		ActorRole:  user.Role,
		EntityType: audit.EntityUser,
		EntityID:   user.ID,
		Payload:    map[string]any{"login": truncateLogin(user.Login, 64)},
	})

	return tokenAccess, tokenRefresh, nil
}

// startSession выпускает пару токенов и сохраняет хеш refresh-токена.
func (s *UserService) startSession(ctx context.Context, user model.User) (string, string, error) {
	tokenAccess, err := s.jwtManager.GenerateAccess(user.ID, user.Role)
	if err != nil {
		slog.Error("user service: start session: failed to generate access token", "error", err)
		return "", "", fmt.Errorf("user service: start session: %w", err)
	}

	tokenRefresh, err := s.jwtManager.GenerateRefresh(user.ID, user.Role)
	if err != nil {
		slog.Error("user service: start session: failed to generate refresh token", "error", err)
		return "", "", fmt.Errorf("user service: start session: %w", err)
	}

	hashRefresh, err := s.hasher.HashRefreshToken(tokenRefresh)
	if err != nil {
		slog.Error("user service: start session: failed to hash refresh token", "error", err)
		return "", "", fmt.Errorf("user service: start session: %w", err)
	}

	err = s.repository.AddRefreshToken(
		hashRefresh,
		time.Now().Add(s.refreshTTL),
		time.Now(),
		nil,
		ctx,
	)
	if err != nil {
		slog.Error("user service: start session: failed to add refresh token", "error", err)
		return "", "", fmt.Errorf("user service: start session: %w", err)
	}

	return tokenAccess, tokenRefresh, nil
}

// IssueSession открывает сессию пользователя без пароля (вход через Telegram).
func (s *UserService) IssueSession(ctx context.Context, userID string) (string, string, error) {
	user, err := s.repository.GetUserByID(userID, ctx)
	if err != nil {
		return "", "", fmt.Errorf("user service: issue session: %w", err)
	}
	access, refresh, err := s.startSession(ctx, user)
	if err != nil {
		return "", "", err
	}
	s.audit.Record(ctx, audit.Entry{
		Action:     audit.ActionAuthLogin,
		ActorID:    user.ID,
		ActorRole:  user.Role,
		EntityType: audit.EntityUser,
		EntityID:   user.ID,
		Payload:    map[string]any{"login": truncateLogin(user.Login, 64), "via": "telegram"},
	})
	return access, refresh, nil
}

// recordLoginFailure пишет неуспешный вход (actor неизвестен).
func (s *UserService) recordLoginFailure(ctx context.Context, login string) {
	s.audit.Record(ctx, audit.Entry{
		Action:     audit.ActionAuthLogin,
		Outcome:    audit.OutcomeFailure,
		EntityType: audit.EntityUser,
		Payload: map[string]any{
			"login":  truncateLogin(login, 64),
			"reason": "invalid_credentials",
		},
	})
}

func (s *UserService) ValidateRefreshToken(token string) (model.RefreshToken, string, error) {
	tokenHash, err := s.hasher.HashRefreshToken(token)
	if err != nil {
		slog.Error("user service: validate refresh token: failed to hash refresh token", "error", err)
		return model.RefreshToken{}, "", fmt.Errorf("user service: validate refresh token: %w", err)
	}

	var refreshToken model.RefreshToken
	refreshToken, err = s.repository.GetRefreshTokenByHash(tokenHash, context.Background())
	if err != nil {
		slog.Error("user service: validate refresh token: failed to get refresh token by hash", "error", err)
		return model.RefreshToken{}, "", fmt.Errorf("user service: validate refresh token: %w", err)
	}

	if refreshToken.RevokedAt != nil {
		return model.RefreshToken{}, "", fmt.Errorf("user service: validate refresh token: token has been revoked")
	}
	if refreshToken.ExpiresAt.Before(time.Now()) {
		return model.RefreshToken{}, "", fmt.Errorf("user service: validate refresh token: token has expired")
	}

	claims, err := s.jwtManager.ValidateRefresh(token)
	if err != nil {
		slog.Error("user service: validate refresh token: failed to validate refresh token", "error", err)
		return model.RefreshToken{}, "", fmt.Errorf("user service: validate refresh token: %w", err)
	}

	return refreshToken, claims.UserID, nil
}

func (s *UserService) GenerateAccessRefreshTokens(refreshToken model.RefreshToken, userID string) (string, string, error) {
	user, err := s.repository.GetUserByID(userID, context.Background())
	if err != nil {
		slog.Error("user service: generate access and refresh tokens: failed to get user by ID", "error", err)
		return "", "", fmt.Errorf("user service: generate access and refresh tokens: %w", err)
	}

	tokenAccess, err := s.jwtManager.GenerateAccess(user.ID, user.Role)
	if err != nil {
		slog.Error("user service: generate access and refresh tokens: failed to generate access token", "error", err)
		return "", "", fmt.Errorf("user service: generate access and refresh tokens: %w", err)
	}

	tokenRefresh, err := s.jwtManager.GenerateRefresh(user.ID, user.Role)
	if err != nil {
		slog.Error("user service: generate access and refresh tokens: failed to generate refresh token", "error", err)
		return "", "", fmt.Errorf("user service: generate access and refresh tokens: %w", err)
	}

	hashRefresh, err := s.hasher.HashRefreshToken(tokenRefresh)
	if err != nil {
		slog.Error("user service: generate access and refresh tokens: failed to hash refresh token", "error", err)
		return "", "", fmt.Errorf("user service: generate access and refresh tokens: %w", err)
	}
	err = s.repository.AddRefreshToken(
		hashRefresh,
		time.Now().Add(s.refreshTTL),
		time.Now(),
		nil,
		context.Background(),
	)
	if err != nil {
		slog.Error("user service: generate access and refresh tokens: failed to add new refresh token", "error", err)
		return "", "", fmt.Errorf("user service: generate access and refresh tokens: %w", err)
	}

	err = s.repository.RevokeRefreshToken(refreshToken.TokenHash, context.Background())
	if err != nil {
		slog.Error("user service: generate access and refresh tokens: failed to revoke old refresh token", "error", err)
		return "", "", fmt.Errorf("user service: generate access and refresh tokens: %w", err)
	}

	return tokenAccess, tokenRefresh, nil

}

func (s *UserService) RevokeRefreshToken(ctx context.Context, token string) error {
	tokenHash, err := s.hasher.HashRefreshToken(token)
	if err != nil {
		slog.Error("user service: revoke refresh token: failed to hash refresh token", "error", err)
		return fmt.Errorf("user service: revoke refresh token: %w", err)
	}

	err = s.repository.RevokeRefreshToken(tokenHash, ctx)
	if err != nil {
		slog.Error("user service: revoke refresh token: failed to revoke refresh token", "error", err)
		return fmt.Errorf("user service: revoke refresh token: %w", err)
	}

	// Актора берём из самого refresh-токена (без БД): он уже отозван, но подпись ещё читается.
	entry := audit.Entry{Action: audit.ActionAuthLogout, EntityType: audit.EntityUser}
	if claims, verr := s.jwtManager.ValidateRefresh(token); verr == nil {
		entry.ActorID = claims.UserID
		entry.ActorRole = claims.Role
		entry.EntityID = claims.UserID
	}
	s.audit.Record(ctx, entry)

	return nil
}

func (s *UserService) CreateUser(user request.CreateUserRequest, ctx context.Context) error {
	passwordHash, err := s.hasher.HashPassword(user.Password)
	if err != nil {
		slog.Error("user service: create user: failed to hash password", "error", err)
		return fmt.Errorf("user service: create user: %w", err)
	}
	newUser := model.User{
		FullName:     user.FullName,
		Login:        user.Login,
		PasswordHash: passwordHash,
		Role:         userRole,
	}
	if user.PersonaID != nil {
		newUser.Persona = &model.PersonaRef{ID: *user.PersonaID}
	}

	err = s.repository.CreateUser(newUser, ctx)
	if err != nil {
		slog.Error("user service: create user: failed to create user", "error", err)
		return fmt.Errorf("user service: create user: %w", err)
	}

	payload := map[string]any{"login": newUser.Login, "full_name": newUser.FullName}
	if user.PersonaID != nil {
		payload["persona_id"] = *user.PersonaID
	}
	entry := audit.Entry{
		Action:     audit.ActionUserCreate,
		EntityType: audit.EntityUser,
		Payload:    payload,
	}
	if created, gerr := s.repository.GetUserByLogin(newUser.Login, ctx); gerr == nil {
		entry.EntityID = created.ID
	}
	s.audit.Record(ctx, entry)

	return nil
}

func (s *UserService) GetAllUsers(ctx context.Context) ([]model.User, error) {
	users, err := s.repository.GetAllUsers(ctx)
	if err != nil {
		slog.Error("user service: get all users: failed to get users", "error", err)
		return nil, fmt.Errorf("user service: get all users: %w", err)
	}
	return users, nil
}

func (s *UserService) GetAllUsersWithTotal(ctx context.Context) (response.UserListResponse, error) {
	users, err := s.repository.GetAllUsers(ctx)
	if err != nil {
		slog.Error("user service: get all users with total: failed to get users", "error", err)
		return response.UserListResponse{}, fmt.Errorf("user service: get all users with total: %w", err)
	}
	return response.UserListResponse{Items: users, Total: len(users)}, nil
}

func (s *UserService) DeleteUserByID(id string, ctx context.Context) error {
	user, err := s.repository.GetUserByID(id, ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.ErrUserNotFound
		}
		slog.Error("user service: delete user by id: failed to get user", "error", err)
		return fmt.Errorf("user service: delete user by id: %w", err)
	}

	// Учётка администратора задаётся в .env и через панель не управляется.
	if user.Role == adminRole {
		return model.ErrAdminImmutable
	}

	if err := s.repository.DeleteUserByID(id, ctx); err != nil {
		slog.Error("user service: delete user by id: failed to delete user", "error", err)
		return fmt.Errorf("user service: delete user by id: %w", err)
	}

	s.audit.Record(ctx, audit.Entry{
		Action:     audit.ActionUserDelete,
		EntityType: audit.EntityUser,
		EntityID:   id,
		Payload:    map[string]any{"login": user.Login, "full_name": user.FullName},
	})
	return nil
}

func (s *UserService) GetUserByID(id string, ctx context.Context) (model.User, error) {
	user, err := s.repository.GetUserByID(id, ctx)
	if err != nil {
		slog.Error("user service: get user by id: failed to get user", "error", err)
		return model.User{}, fmt.Errorf("user service: get user by id: %w", err)
	}
	return user, nil
}

func (s *UserService) UpdateUserByID(ctx context.Context, userID string, req request.UserUpdateRequest) error {
	user, err := s.repository.GetUserByID(userID, ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.ErrUserNotFound
		}
		slog.Error("user service: update user by id: failed to get user", "error", err)
		return fmt.Errorf("user service: update user by id: %w", err)
	}
	if user.Role == adminRole {
		return model.ErrAdminImmutable
	}

	oldFullName, oldLogin := user.FullName, user.Login
	if req.FullName != nil {
		user.FullName = *req.FullName
	}
	if req.Login != nil {
		user.Login = *req.Login
	}
	oldPersona := ""
	if user.Persona != nil {
		oldPersona = user.Persona.ID
	}
	if req.PersonaID != nil {
		if *req.PersonaID == "" {
			user.Persona = nil
		} else {
			user.Persona = &model.PersonaRef{ID: *req.PersonaID}
		}
	}
	if req.Password != nil {
		hash, err := s.hasher.HashPassword(*req.Password)
		if err != nil {
			return fmt.Errorf("user service: update user: hash password: %w", err)
		}
		user.PasswordHash = hash
	}

	if err := s.repository.UpdateUser(user, ctx); err != nil {
		return err
	}
	newPersona := ""
	if user.Persona != nil {
		newPersona = user.Persona.ID
	}

	s.audit.Record(ctx, audit.Entry{
		Action:     audit.ActionUserUpdate,
		EntityType: audit.EntityUser,
		EntityID:   userID,
		Changes: audit.Diff().
			Add("login", oldLogin, user.Login).
			Add("full_name", oldFullName, user.FullName).
			Add("persona_id", oldPersona, newPersona).
			Map(),
		Payload: map[string]any{"password_changed": req.Password != nil},
	})
	return nil
}
