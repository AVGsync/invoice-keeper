package apiserver

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	_ "github.com/AVGsync/invoice-keeper/backend/docs"
	"github.com/AVGsync/invoice-keeper/backend/internal/audit"
	"github.com/AVGsync/invoice-keeper/backend/internal/clock"
	"github.com/AVGsync/invoice-keeper/backend/internal/infrastructure/security"
	"github.com/AVGsync/invoice-keeper/backend/internal/notifications"
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
	// tg — компоненты Telegram-бота, собранные в configureRouter и запускаемые в startBackgroundWorkers (см. telegram.go).
	tg *telegramRuntime
	// assistant — сервис NL-ассистента; nil, когда NLQ_ENABLED=false (см. assistant.go).
	assistant *service.AssistantService
	// assistantHistory — история диалогов ассистента; есть всегда (ручки истории работают и при NLQ_ENABLED=false).
	assistantHistory *service.AssistantHistoryService
	// mailer — SMTP-клиент; пока только для тестового письма.
	mailer *notifications.SMTPMailer
	// payments — сервис периодов (общий для HTTP и Telegram-адаптера).
	payments *service.PaymentService
	// scheduler — генератор периодов и пересчёт overdue (фоновый воркер + ensurer).
	scheduler *service.PaymentScheduler
	// audit — журнал действий; общий для HTTP и бота.
	audit *audit.Logger
}

func New(config *Config) (*APIServer, error) {
	logger, err := newLogger(config)
	if err != nil {
		return nil, fmt.Errorf("apiserver: configure logger: %w", err)
	}

	// SMTP необязателен.
	var mailer *notifications.SMTPMailer
	if config.SMTPHost == "" {
		slog.Info("smtp: disabled (SMTP_HOST is empty)")
	} else {
		mailer, err = notifications.NewSMTPMailer(
			notifications.SMTPConfig{
				Host:     config.SMTPHost,
				Port:     config.SMTPPort,
				Username: config.SMTPUsername,
				Password: config.SMTPPassword,
				From:     config.SMTPFrom,
			},
			logger,
		)
		if err != nil {
			return nil, fmt.Errorf("initialize SMTP mailer: %w", err)
		}
	}

	return &APIServer{
		config: config,
		logger: logger,
		router: chi.NewRouter(),
		mailer: mailer,
	}, nil
}

func (s *APIServer) Start() error {
	if err := s.configureDB(); err != nil {
		return fmt.Errorf("apiserver: configure database: %w", err)
	}
	if err := s.ensureAdminAccount(); err != nil {
		s.db.Close()
		return fmt.Errorf("apiserver: %w", err)
	}

	s.configureRouter()

	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup
	s.startBackgroundWorkers(rootCtx, &wg)

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

	// Graceful shutdown: по SIGINT/SIGTERM отменяем корневой контекст воркеров и даём серверу закрыть соединения.
	shutdownErr := make(chan error, 1)
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
		<-quit

		slog.Info("shutting down api server")
		cancel()
		ctx, c := context.WithTimeout(context.Background(), 10*time.Second)
		defer c()
		shutdownErr <- srv.Shutdown(ctx)
	}()

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		// Сервер не поднялся: останавливаем воркеров и закрываем БД.
		cancel()
		wg.Wait()
		s.db.Close()
		return err
	}

	sdErr := <-shutdownErr
	// Контекст уже отменён в обработчике сигнала; дожидаемся воркеров и только потом закрываем пул БД.
	wg.Wait()
	s.db.Close()
	return sdErr
}

// startBackgroundWorkers запускает фоновые горутины приложения.
func (s *APIServer) startBackgroundWorkers(ctx context.Context, wg *sync.WaitGroup) {
	s.startPaymentsScheduler(ctx, wg)

	s.startTelegram(ctx, wg)
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

// ensureAdminAccount приводит учётку администратора к ADMIN_LOGIN и ADMIN_PASSWORD из окружения.
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
	s.audit = audit.NewLogger(s.db.Audit())

	hasher := security.NewHasher()
	accessTTL := time.Duration(s.config.TTLAccessToken) * time.Second
	refreshTTL := time.Duration(s.config.TTLRefreshToken) * time.Second
	jwtManager := security.NewJWTManager(s.config.JWTAccessSecret, s.config.JWTRefreshSecret, accessTTL, refreshTTL)

	userService := service.NewUserService(s.db.User(), hasher, jwtManager, accessTTL, refreshTTL)
	userHandler := handler.NewUserHandler(userService, accessTTL, refreshTTL)

	clk := clock.New(s.config.Location())
	paymentService := service.NewPaymentService(s.db.Payment(), clk)
	scheduler := service.NewPaymentScheduler(s.db.RecurrentPayment(), s.db.Payment(), clk)
	recPaymentService := service.NewRecurrentPaymentService(s.db.RecurrentPayment(), s.db.Payment(), clk)
	s.payments = paymentService
	s.scheduler = scheduler
	paymentHandler := handler.NewPaymentHandler(recPaymentService, paymentService, scheduler, clk)

	receiptService := s.newReceiptService()
	receiptService.SetAuditRecorder(s.audit)
	receiptHandler := handler.NewReceiptHandler(receiptService)

	searchService := service.NewPaymentSearchService(s.db.Payment(), s.db.RecurrentPayment(), clk)
	searchHandler := handler.NewSearchHandler(searchService, clk)

	notificationsService := service.NewNotificationsService(s.db.Notifications())
	notificationsHandler := handler.NewNotificationsHandler(notificationsService)
	var testMailer handler.Mailer
	if s.mailer != nil {
		testMailer = s.mailer
	}
	testMailHandler := handler.NewTestMailHandler(testMailer)
	testMailHandler.SetAuditRecorder(s.audit)

	personaService := service.NewPersonaService(s.db.Persona())
	personaHandler := handler.NewPersonaHandler(personaService)

	userService.SetAuditRecorder(s.audit)
	userHandler.SetAuditRecorder(s.audit)
	paymentService.SetAuditRecorder(s.audit)
	recPaymentService.SetAuditRecorder(s.audit)
	notificationsService.SetAuditRecorder(s.audit)
	personaService.SetAuditRecorder(s.audit)
	auditHandler := handler.NewAuditHandler(service.NewAuditService(s.db.Audit()), clk)

	mw := middleware.NewMiddleware(jwtManager, s.config.AllowedOrigins)

	s.assistant = s.buildAssistant()

	s.tg = s.buildTelegram(userService, hasher)
	if s.assistant != nil && s.tg != nil {
		s.tg.service.AttachNLAnswerer(s.assistant)
	}

	s.router.Route("/api/v1", func(r chi.Router) {
		r.Use(mw.CORS)
		r.Use(mw.Trace)

		r.Get("/ping", ping)

		// Swagger UI и спецификация наружу только при DEBUG=true.
		if s.config.Debug {
			r.Get("/docs", func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, r.URL.Path+"/index.html", http.StatusMovedPermanently)
			})
			r.Get("/docs/*", httpSwagger.Handler(httpSwagger.URL("doc.json")))
		}

		s.mountAuth(r, mw, userHandler)

		r.With(mw.LimitByIP(60, time.Minute)).Get("/receipt-files/{id}", receiptHandler.ServeFile())

		s.mountTelegramWebhook(r, mw)
		s.mountTelegramMiniApp(r, mw)
		s.mountTelegram(r, mw)

		r.Group(func(r chi.Router) {
			r.Use(api.RequestIDMiddleware)
			r.Use(mw.Auth)
			r.Use(mw.AuditContext)
			r.Use(mw.LimitByUser(120, time.Minute))
			r.Use(mw.IsUserOrAdmin)

			r.Get("/auth/me", userHandler.Me())

			s.mountUsers(r, mw, userHandler)
			s.mountPersonas(r, mw, personaHandler)
			s.mountPayments(r, mw, paymentHandler, searchHandler, receiptHandler)
			s.mountNotifications(r, mw, notificationsHandler, testMailHandler)
			s.mountAssistant(r, mw)
			s.mountAudit(r, mw, auditHandler)
		})
	})
}

// mountAuth монтирует публичную группу /auth (login, refresh, logout) с жёстким лимитом по IP против брутфорса.
func (s *APIServer) mountAuth(r chi.Router, mw *middleware.Middleware, userHandler *handler.UserHandler) {
	r.Route("/auth", func(r chi.Router) {
		r.Use(api.RequestIDMiddleware)
		r.Use(mw.AuditContext)
		r.Use(mw.LimitByIP(10, time.Minute))
		r.Use(mw.MaxBytes(defaultMaxRequestBody))
		r.Post("/login", userHandler.Login())
		r.Post("/refresh", userHandler.RefreshToken())
		r.Post("/logout", userHandler.Logout())
	})
}

// mountUsers монтирует управление пользователями — только администратор.
func (s *APIServer) mountUsers(r chi.Router, mw *middleware.Middleware, userHandler *handler.UserHandler) {
	r.Route("/users", func(r chi.Router) {
		r.Use(mw.IsAdmin)
		r.Use(mw.MaxBytes(defaultMaxRequestBody))

		r.Get("/", userHandler.GetAllUsers())
		r.Post("/", userHandler.CreateUser())
		r.Patch("/{id}", userHandler.UpdateUserByID())
		r.Delete("/{id}", userHandler.DeleteUserByID())
	})
}

// mountPersonas монтирует справочник персон — только администратор.
func (s *APIServer) mountPersonas(r chi.Router, mw *middleware.Middleware, personaHandler *handler.PersonaHandler) {
	r.Route("/personas", func(r chi.Router) {
		r.Use(mw.IsAdmin)
		r.Use(mw.MaxBytes(defaultMaxRequestBody))

		r.Get("/", personaHandler.GetAllPersonas())
		r.Post("/", personaHandler.CreatePersona())
		r.Patch("/{id}", personaHandler.UpdatePersona())
		r.Delete("/{id}", personaHandler.DeletePersona())
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
