package main

import (
	_ "time/tzdata"

	"github.com/AVGsync/invoice-keeper/backend/internal/app/apiserver"
)

// @title                       Invoice Keeper API
// @version                     1.0
// @description                 Напоминания о регулярных платежах.
// @BasePath                    /api/v1
// @tag.name         Система
// @tag.description  Служебные эндпоинты (проверка доступности).
// @tag.name         Auth
// @tag.description  Вход, выход, обновление токенов и текущий пользователь.
// @tag.name         Admin
// @tag.description  Управление пользователями — только администратор. Учётка администратора задаётся в .env и через API не меняется.
// @tag.name         Personas
// @tag.description  Справочник персон — должностей или зон ответственности (Бухгалтер, Руководитель, DevOps). Ведёт администратор; при создании пользователя персона выбирается из этого списка (`persona_id` в `POST /users`). На права доступа не влияет — права задаёт роль.
// @securityDefinitions.apikey  BearerAuth
// @in                          header
// @name                        Authorization
// @description                 "Bearer <access token>"
// @securityDefinitions.apikey  CookieAuth
// @in                          cookie
// @name                        AccessToken
func main() {
	config, err := apiserver.NewConfig()
	if err != nil {
		panic(err)
	}

	s, err := apiserver.New(config)
	if err != nil {
		panic(err)
	}

	if err := s.Start(); err != nil {
		panic(err)
	}
}
