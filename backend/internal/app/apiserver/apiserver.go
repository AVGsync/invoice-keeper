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

	// Регистрирует сгенерированную swag спецификацию (make swagger).
	_ "github.com/AVGsync/invoice-keeper/backend/docs"
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
	// tg — компоненты Telegram-бота, собранные в configureRouter и запускаемые
	// в startBackgroundWorkers (см. telegram.go).
	tg *telegramRuntime
	// assistant — сервис NL-ассистента; nil, когда NLQ_ENABLED=false
	// (см. assistant.go). Общий для HTTP-ручки и бота (порт NLAnswerer).
	assistant *service.AssistantService
	// mailer — SMTP-рассылка; не используется (решение №1: напоминания только в
	// командный чат Telegram). nil, когда SMTP_HOST пуст.
	mailer *notifications.SMTPMailer
	// payments — сервис периодов (общий для HTTP и Telegram-адаптера).
	payments *service.PaymentService
	// scheduler — генератор периодов и пересчёт overdue (фоновый воркер + ensurer).
	scheduler *service.PaymentScheduler
}

func New(config *Config) (*APIServer, error) {
	logger, err := newLogger(config)
	if err != nil {
		return nil, fmt.Errorf("apiserver: configure logger: %w", err)
	}

	// SMTP необязателен (решение №1: email не используется). Пустой SMTP_HOST —
	// почта выключена, старт продолжается. Непустой host с неполной конфигурацией
	// — ошибка (NewSMTPMailer сообщит, чего не хватает).
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
	// БД закрываем только после остановки фоновых воркеров (см. ниже wg.Wait),
	// иначе воркер может обратиться к уже закрытому пулу (docs/TELEGRAM_BOT.md §5.3).
	if err := s.ensureAdminAccount(); err != nil {
		s.db.Close()
		return fmt.Errorf("apiserver: %w", err)
	}

	s.configureRouter()

	// Корневой контекст фоновых воркеров. Отменяется по сигналу остановки, и все
	// горутины, запущенные через startBackgroundWorkers, завершаются по нему.
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

	// Graceful shutdown: по SIGINT/SIGTERM отменяем корневой контекст воркеров
	// и даём серверу закрыть соединения.
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
	// Контекст уже отменён в обработчике сигнала; дожидаемся воркеров и только
	// потом закрываем пул БД.
	wg.Wait()
	s.db.Close()
	return sdErr
}

// startBackgroundWorkers запускает фоновые горутины приложения. В PR №0 их ещё
// нет: здесь появятся движок напоминаний (reminder.Loop) и Telegram-поллер,
// каждый как wg.Add(1); go worker.Run(ctx). Все слушают отмену ctx и
// учитываются в wg, чтобы db.Close выполнился после их остановки (§5.3).
func (s *APIServer) startBackgroundWorkers(ctx context.Context, wg *sync.WaitGroup) {
	// Генератор периодов и пересчёт overdue — независимо от бота (см.
	// payments_worker.go).
	s.startPaymentsScheduler(ctx, wg)

	// Telegram-поллер/вебхук, засев командного чата и движок напоминаний
	// (см. telegram.go).
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

	userService := service.NewUserService(s.db.User(), hasher, jwtManager, accessTTL, refreshTTL)
	userHandler := handler.NewUserHandler(userService, accessTTL, refreshTTL)

	clk := clock.New(s.config.Location())
	paymentService := service.NewPaymentService(s.db.Payment(), clk)
	scheduler := service.NewPaymentScheduler(s.db.RecurrentPayment(), s.db.Payment(), clk)
	recPaymentService := service.NewRecurrentPaymentService(s.db.RecurrentPayment(), s.db.Payment(), clk)
	// Сохраняем для Telegram-адаптера (paymentsPayer) и фонового воркера.
	s.payments = paymentService
	s.scheduler = scheduler
	paymentHandler := handler.NewPaymentHandler(recPaymentService, paymentService, scheduler, clk)

	notificationsService := service.NewNotificationsService(s.db.Notifications())
	notificationsHandler := handler.NewNotificationsHandler(notificationsService)

	personaService := service.NewPersonaService(s.db.Persona())
	personaHandler := handler.NewPersonaHandler(personaService)

	mw := middleware.NewMiddleware(jwtManager, s.config.AllowedOrigins)

	// Сервис ассистента собирается до бота: бот получает его как порт NLAnswerer
	// для свободного текста в личке и команды /ask в группах (NL_QUERIES.md §4.4).
	s.assistant = s.buildAssistant()

	// Компоненты Telegram-бота собираются один раз и используются и роутером,
	// и фоновыми воркерами (startBackgroundWorkers).
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

		// Telegram: публичный webhook (только в режиме webhook) и авторизованные
		// ручки привязки/командного чата с JSON-ошибками (telegram.go).
		s.mountTelegramWebhook(r, mw)
		s.mountTelegramMiniApp(r, mw)
		s.mountTelegram(r, mw)

		r.Group(func(r chi.Router) {
			r.Use(api.RequestIDMiddleware)
			r.Use(mw.Auth)
			r.Use(mw.LimitByUser(120, time.Minute))
			r.Use(mw.IsUserOrAdmin)

			r.Get("/auth/me", userHandler.Me())

			s.mountUsers(r, mw, userHandler)
			s.mountPersonas(r, mw, personaHandler)
			s.mountPayments(r, mw, paymentHandler)
			s.mountNotifications(r, mw, notificationsHandler)
			s.mountAssistant(r, mw)
		})
	})
}

// mountAuth монтирует публичную группу /auth (login, refresh, logout) с жёстким
// лимитом по IP против брутфорса.
func (s *APIServer) mountAuth(r chi.Router, mw *middleware.Middleware, userHandler *handler.UserHandler) {
	r.Route("/auth", func(r chi.Router) {
		// Жёсткий лимит по IP против брутфорса логина/refresh. Значения — под тюнинг.
		r.Use(mw.LimitByIP(10, time.Minute))
		r.Use(mw.MaxBytes(defaultMaxRequestBody))
		r.Post("/login", userHandler.Login())
		r.Post("/refresh", userHandler.RefreshToken())
		r.Post("/logout", userHandler.Logout())
	})
}

// mountUsers монтирует управление пользователями — только администратор.
// Вызывается внутри защищённой группы (Auth, LimitByUser, IsUserOrAdmin).
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
// Вызывается внутри защищённой группы (Auth, LimitByUser, IsUserOrAdmin).
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
