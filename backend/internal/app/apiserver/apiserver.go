package apiserver

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	// Регистрирует сгенерированную swag спецификацию (make swagger).
	_ "github.com/AVGsync/invoice-keeper/backend/docs"
	"github.com/AVGsync/invoice-keeper/backend/internal/infrastructure/security"
	"github.com/AVGsync/invoice-keeper/backend/internal/repository/postgres"
	"github.com/AVGsync/invoice-keeper/backend/internal/service"
	"github.com/AVGsync/invoice-keeper/backend/internal/transport/http/api"
	"github.com/AVGsync/invoice-keeper/backend/internal/transport/http/handler"
	"github.com/AVGsync/invoice-keeper/backend/internal/transport/http/middleware"

	"github.com/go-chi/chi/v5"
	httpSwagger "github.com/swaggo/http-swagger/v2"
)

// Лимит размера тела запроса для JSON-эндпоинтов.
const defaultMaxRequestBody = 1 << 20 // 1 MB

type APIServer struct {
	config *Config
	logger *slog.Logger
	router *chi.Mux
	db     *postgres.DB
}

func New(config *Config) (*APIServer, error) {
	logger, err := newLogger(config)
	if err != nil {
		return nil, fmt.Errorf("apiserver: configure logger: %w", err)
	}

	return &APIServer{
		config: config,
		logger: logger,
		router: chi.NewRouter(),
	}, nil
}

func (s *APIServer) Start() error {
	if err := s.configureDB(); err != nil {
		return fmt.Errorf("apiserver: configure database: %w", err)
	}
	defer s.db.Close()
	if err := s.ensureAdminAccount(); err != nil {
		return fmt.Errorf("apiserver: %w", err)
	}

	s.configureRouter()

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%s", s.config.BindAddr),
		Handler:           s.router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	slog.Info("Starting api server",
		"bind_addr", s.config.BindAddr,
		"log_level", s.config.LogLevel,
		"debug", s.config.Debug,
	)

	// Graceful shutdown: по SIGINT/SIGTERM даём серверу закрыть соединения.
	shutdownErr := make(chan error, 1)
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
		<-quit

		slog.Info("shutting down api server")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		shutdownErr <- srv.Shutdown(ctx)
	}()

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}

	return <-shutdownErr
}

func newLogger(cfg *Config) (*slog.Logger, error) {
	levelStr := cfg.LogLevel

	if cfg.Debug {
		levelStr = "debug"
	}

	var level slog.Level
	if err := level.UnmarshalText([]byte(levelStr)); err != nil {
		return nil, fmt.Errorf("unknown log level %q: %w", levelStr, err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	}))
	slog.SetDefault(logger)
	return logger, nil
}

func (s *APIServer) configureDB() error {
	s.db = postgres.New(s.config.DatabaseURL())

	if err := s.db.Open(); err != nil {
		return fmt.Errorf("open database connection: %w", err)
	}
	return nil
}

// ensureAdminAccount приводит учётку администратора к ADMIN_LOGIN и
// ADMIN_PASSWORD из окружения. Делается до старта HTTP: если войти в панель
// некому, поднимать сервер незачем.
func (s *APIServer) ensureAdminAccount() error {
	admin, err := service.NewAdminAccount(s.db.User(), security.NewHasher())
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return admin.Ensure(s.config.AdminLogin, s.config.AdminPassword, ctx)
}

func (s *APIServer) configureRouter() {
	hasher := security.NewHasher()
	accessTTL := time.Duration(s.config.TTLAccessToken) * time.Second
	refreshTTL := time.Duration(s.config.TTLRefreshToken) * time.Second
	jwtManager := security.NewJWTManager(s.config.JWTAccessSecret, s.config.JWTRefreshSecret, accessTTL, refreshTTL)

	userService := service.NewUserService(s.db.User(), hasher, jwtManager, refreshTTL)
	userHandler := handler.NewUserHandler(userService, accessTTL, refreshTTL)

	middleware := middleware.NewMiddleware(jwtManager, s.config.AllowedOrigins)

	s.router.Route("/api/v1", func(r chi.Router) {
		r.Use(middleware.CORS)
		r.Use(middleware.Trace)

		r.Get("/ping", ping)

		// Swagger UI и спецификация наружу только при DEBUG=true.
		if s.config.Debug {
			r.Get("/docs", func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, r.URL.Path+"/index.html", http.StatusMovedPermanently)
			})
			r.Get("/docs/*", httpSwagger.Handler(httpSwagger.URL("doc.json")))
		}

		r.Route("/auth", func(r chi.Router) {
			// Жёсткий лимит по IP против брутфорса логина/refresh. Значения — под тюнинг.
			r.Use(middleware.LimitByIP(10, time.Minute))
			r.Use(middleware.MaxBytes(defaultMaxRequestBody))
			r.Post("/login", userHandler.Login())
			r.Post("/refresh", userHandler.RefreshToken())
			r.Post("/logout", userHandler.Logout())
		})

		r.Group(func(r chi.Router) {
			r.Use(api.RequestIDMiddleware)
			r.Use(middleware.Auth)
			r.Use(middleware.LimitByUser(120, time.Minute))
			r.Use(middleware.IsUserOrAdmin)

			r.Get("/auth/me", userHandler.Me())

			// Управление пользователями — только администратор.
			r.Route("/users", func(r chi.Router) {
				r.Use(middleware.IsAdmin)
				r.Use(middleware.MaxBytes(defaultMaxRequestBody))

				r.Get("/", userHandler.GetAllUsers())
				r.Post("/", userHandler.CreateUser())
				r.Patch("/{id}", userHandler.UpdateUserByID())
				r.Delete("/{id}", userHandler.DeleteUserByID())
			})
		})
	})
}

// ping godoc
//
// @Summary      Проверка доступности API-сервера
// @Description  Лёгкий liveness-пробник: подтверждает, что HTTP-сервер поднят и роутинг
// @Description  работает. НЕ проверяет доступность PostgreSQL и внешних сервисов —
// @Description  всегда отвечает `200 pong`, если процесс жив. Аутентификация не требуется.
// @Tags         Система
// @Produce      plain
// @Success      200  {string}  string  "pong"
// @Router       /ping [get]
func ping(w http.ResponseWriter, _ *http.Request) {
	w.Write([]byte("pong"))
}
