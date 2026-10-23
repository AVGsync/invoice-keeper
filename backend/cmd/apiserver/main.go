package main

import (
	// Embed the IANA timezone database into the server binary. The production
	// Alpine image intentionally contains only the binary and has no system
	// tzdata, while the app timezone is configured by IANA name such as Europe/Moscow.
	_ "time/tzdata"

	"github.com/AVGsync/invoice-keeper/backend/internal/app/apiserver"
)

// @title                       Invoice Keeper API
// @version                     1.0
// @description                 Напоминания о регулярных платежах.
// @BasePath                    /api/v1
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
