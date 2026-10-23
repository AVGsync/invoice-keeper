package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/AVGsync/invoice-keeper/backend/internal/model"
)

// Учётка администратора из переменных окружения.
//
// Раньше администратор заводился миграцией 000002 с зашитым bcrypt-хэшем:
// один и тот же пароль у всех, кто поднимал проект, и лежал он в публичном
// репозитории. Теперь логин и пароль берутся из .env, и при каждом старте
// приложение приводит учётку к тому, что там записано.
//
// .env — единственный источник правды: поменять пароль сотрудника через CRM
// нельзя (PATCH /crm/users меняет только ФИО и логин), поэтому новый пароль
// администратора задаётся правкой ADMIN_PASSWORD и перезапуском backend.

const adminRole = "admin"

// adminFullName — ФИО по умолчанию, только для вновь созданной учётки. У уже
// существующей оно не трогается: его могли поменять в CRM.
const adminFullName = "Администратор"

type AdminAccountRepository interface {
	GetUserByLogin(login string, ctx context.Context) (model.User, error)
	CreateUser(user model.User, ctx context.Context) error
	UpdateUser(user model.User, ctx context.Context) error
	CountUsersByRole(role string, ctx context.Context) (int, error)
}

type AdminAccountHasher interface {
	HashPassword(password string) (string, error)
	CheckPassword(plain, hashed string) bool
}

type AdminAccount struct {
	repository AdminAccountRepository
	hasher     AdminAccountHasher
}

func NewAdminAccount(repository AdminAccountRepository, hasher AdminAccountHasher) (*AdminAccount, error) {
	if repository == nil || hasher == nil {
		return nil, errors.New("admin account requires repository and hasher")
	}
	return &AdminAccount{repository: repository, hasher: hasher}, nil
}

// Ensure создаёт администратора с логином и паролем из окружения, а если он
// уже есть — проверяет, что пароль совпадает с ADMIN_PASSWORD, и переписывает
// хэш, когда нет.
//
// Пустой пароль означает, что переменная в .env не задана: на уже работающей
// установке это не повод падать — там есть администратор со своим паролем.
// А вот на пустой базе войти было бы некому, поэтому там это ошибка.
func (a *AdminAccount) Ensure(login, password string, ctx context.Context) error {
	if password == "" {
		return a.requireExistingAdmin(ctx)
	}

	user, err := a.repository.GetUserByLogin(login, ctx)
	switch {
	case err == nil:
	case errors.Is(err, sql.ErrNoRows):
		return a.create(login, password, ctx)
	default:
		return fmt.Errorf("admin account: look up %q: %w", login, err)
	}

	if user.Role != adminRole {
		// Логин занят менеджером: молча повысить его до администратора нельзя,
		// а переписывать ему пароль — тем более.
		return fmt.Errorf("admin account: login %q belongs to role %q", login, user.Role)
	}
	if a.hasher.CheckPassword(password, user.PasswordHash) {
		return nil
	}

	hash, err := a.hasher.HashPassword(password)
	if err != nil {
		return fmt.Errorf("admin account: hash password: %w", err)
	}
	user.PasswordHash = hash
	if err := a.repository.UpdateUser(user, ctx); err != nil {
		return fmt.Errorf("admin account: update password: %w", err)
	}
	slog.Info("admin account: password set from ADMIN_PASSWORD", "login", login)
	return nil
}

func (a *AdminAccount) create(login, password string, ctx context.Context) error {
	hash, err := a.hasher.HashPassword(password)
	if err != nil {
		return fmt.Errorf("admin account: hash password: %w", err)
	}
	user := model.User{FullName: adminFullName, Login: login, PasswordHash: hash, Role: adminRole}
	if err := a.repository.CreateUser(user, ctx); err != nil {
		return fmt.Errorf("admin account: create %q: %w", login, err)
	}
	slog.Info("admin account: created from ADMIN_LOGIN/ADMIN_PASSWORD", "login", login)
	return nil
}

func (a *AdminAccount) requireExistingAdmin(ctx context.Context) error {
	count, err := a.repository.CountUsersByRole(adminRole, ctx)
	if err != nil {
		return fmt.Errorf("admin account: count admins: %w", err)
	}
	if count == 0 {
		return errors.New("admin account: ADMIN_PASSWORD is required: database has no admin to log in with")
	}
	slog.Warn("admin account: ADMIN_PASSWORD is empty, existing admin password left as is")
	return nil
}
