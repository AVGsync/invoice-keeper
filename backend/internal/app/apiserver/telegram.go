package apiserver

import (
	"context"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/AVGsync/invoice-keeper/backend/internal/clock"
	"github.com/AVGsync/invoice-keeper/backend/internal/infrastructure/telegram"
	"github.com/AVGsync/invoice-keeper/backend/internal/reminder"
	"github.com/AVGsync/invoice-keeper/backend/internal/telegrambot"
	"github.com/AVGsync/invoice-keeper/backend/internal/transport/http/api"
	"github.com/AVGsync/invoice-keeper/backend/internal/transport/http/handler"
	"github.com/AVGsync/invoice-keeper/backend/internal/transport/http/middleware"

	"github.com/go-chi/chi/v5"
)

// telegramRuntime — собранные компоненты бота. Единственное место, где склеены
// клиент Bot API, сервис, HTTP-хендлер и поллер (docs/TELEGRAM_BOT.md §8).
// Когда бот выключен (режим off или пустой токен), enabled=false: REST-ручки
// работают (link-code отдаёт 503, чтение привязки доступно), но горутин нет.
type telegramRuntime struct {
	service *telegrambot.Service
	handler *handler.TelegramHandler
	client  *telegram.Client
	loop    *reminder.Loop // движок напоминаний; nil, когда бот выключен
	enabled bool
	mode    string
}

// buildTelegram собирает компоненты бота из конфигурации. Вызывается из
// configureRouter. UserReader — репозиторий пользователей (GetUserByID,
// GetUserByLogin); tokens — UserService (вход в Mini App, §3.3); hasher —
// проверка пароля при входе по логину.
func (s *APIServer) buildTelegram(tokens telegrambot.TokenIssuer, hasher telegrambot.Hasher) *telegramRuntime {
	enabled := s.config.TelegramMode != "off" && s.config.TelegramBotToken != ""

	var realClient *telegram.Client
	var botAPI telegrambot.BotAPI
	if enabled {
		realClient = telegram.NewClient(s.config.TelegramBotToken, s.config.TelegramAPIURL)
		botAPI = realClient
	}

	service := telegrambot.NewService(
		s.db.Telegram(),
		s.db.User(),
		botAPI,
		tokens,
		hasher,
		telegrambot.Config{
			Enabled:        enabled,
			BotUsername:    s.config.TelegramBotUsername,
			MiniAppURL:     s.config.MiniAppURL,
			LinkCodeTTL:    10 * time.Minute,
			BotToken:       s.config.TelegramBotToken,
			InitDataMaxAge: s.config.TelegramInitDataMaxAge,
		},
	)

	rt := &telegramRuntime{
		service: service,
		handler: handler.NewTelegramHandler(service, s.config.TelegramWebhookSecret),
		client:  realClient,
		enabled: enabled,
		mode:    s.config.TelegramMode,
	}

	// Движок напоминаний поднимается только с живым клиентом (бот включён):
	// без него некому отправлять сообщения и нажимать «Оплачено».
	if enabled {
		s.buildReminder(rt, realClient)
	}
	return rt
}

// buildReminder собирает движок напоминаний и подключает кнопку «Оплачено» к
// сервису бота. Источник периодов и запись оплаты зависят от REMINDER_SOURCE:
// stub (in-memory демо) или db (SQL по таблицам B; Payer — заглушка до этапа 4).
func (s *APIServer) buildReminder(rt *telegramRuntime, client *telegram.Client) {
	clk := clock.New(s.config.Location())

	// isAdmin читает роль из users.role: реальный привязанный админ должен мочь
	// нажать «Оплачено» и в stub-режиме (решение №4). Используется заглушкой
	// MemoryStore; в db-режиме права проверяет сервис B.
	users := s.db.User()
	isAdmin := func(ctx context.Context, userID string) bool {
		u, err := users.GetUserByID(userID, ctx)
		return err == nil && u.Role == "admin"
	}

	demo := s.config.ReminderSource != "db"

	var source reminder.OccurrenceSource
	var payer reminder.Payer
	switch s.config.ReminderSource {
	case "db":
		source = s.db.TelegramSource()
		// paymentsPayer: права и запись — через сервис B, «завершён» — по источнику.
		payer = paymentsPayer{payments: s.payments, source: source, isAdmin: isAdmin}
	default: // stub
		// Назначаем демо-платежи реальному админу (если он уже создан), чтобы
		// «Оплачено» работало и как ответственный, и как админ.
		adminID := ""
		if u, err := users.GetUserByLogin(s.config.AdminLogin, context.Background()); err == nil {
			adminID = u.ID
		}
		occs := reminder.DemoFixtures(clk.Today(), adminID)
		store := reminder.NewMemoryStore(occs, isAdmin)
		source, payer = store, store
	}

	notif := s.db.TelegramNotifications()
	rcfg := reminder.Config{
		LeadDays:         s.config.ReminderLeadDays,
		Tick:             s.config.ReminderTick,
		BotUsername:      s.config.TelegramBotUsername,
		MiniAppShortName: s.config.TelegramMiniAppShortName,
		Demo:             demo,
	}

	botSvc := reminder.NewBotService(source, payer, notif, client, clk, rcfg)
	rt.service.AttachPayments(botSvc)

	// ensurer: в режиме db периоды гарантирует PaymentScheduler перед выборкой
	// (§2.2). В stub источник сам отдаёт фикстуры, ensurer не нужен.
	var ensurer reminder.OccurrenceEnsurer
	if s.config.ReminderSource == "db" && s.scheduler != nil {
		ensurer = s.scheduler
	}
	rt.loop = reminder.NewLoop(source, notif, s.db.Telegram(), client, clk, ensurer, rcfg)
}

// mountTelegramWebhook монтирует публичный маршрут webhook (вне групп Auth и
// лимитеров, только MaxBytes, ответ всегда 200). Только в режиме webhook (§6.1).
func (s *APIServer) mountTelegramWebhook(r chi.Router, mw *middleware.Middleware) {
	if s.tg == nil || s.tg.mode != "webhook" {
		return
	}
	r.Group(func(r chi.Router) {
		r.Use(mw.MaxBytes(defaultMaxRequestBody))
		r.Post("/telegram/webhook", s.tg.handler.Webhook())
	})
}

// mountTelegramMiniApp монтирует публичные эндпоинты входа в Mini App (§7).
// Вне групп Auth: аутентификация идёт по initData Telegram, а не по JWT.
// Лимит по IP (auth 30/min, link 10/min как /auth/login), MaxBytes, JSON-ошибки
// с request_id (RequestIDMiddleware). Раздаётся с того же origin, CORS не нужен.
func (s *APIServer) mountTelegramMiniApp(r chi.Router, mw *middleware.Middleware) {
	if s.tg == nil {
		return
	}
	h := s.tg.handler
	r.Group(func(r chi.Router) {
		r.Use(api.RequestIDMiddleware)
		r.Use(mw.MaxBytes(defaultMaxRequestBody))

		r.With(mw.LimitByIPJSON(30, time.Minute)).Post("/telegram/miniapp/auth", h.MiniAppAuth())
		r.With(mw.LimitByIPJSON(10, time.Minute)).Post("/telegram/miniapp/link", h.MiniAppLink())
	})
}

// mountTelegram монтирует REST привязки и командного чата. Ошибки в едином
// JSON-формате (§7), поэтому используются *JSON-варианты middleware.
func (s *APIServer) mountTelegram(r chi.Router, mw *middleware.Middleware) {
	if s.tg == nil {
		return
	}
	h := s.tg.handler
	r.Group(func(r chi.Router) {
		r.Use(api.RequestIDMiddleware)
		r.Use(mw.AuthJSON)
		r.Use(mw.LimitByUserJSON(120, time.Minute))
		r.Use(mw.IsUserOrAdminJSON)
		r.Use(mw.MaxBytes(defaultMaxRequestBody))

		r.Post("/me/telegram/link-code", h.CreateLinkCode())
		r.Get("/me/telegram", h.GetLink())
		r.Delete("/me/telegram", h.DeleteLink())

		// Админские ручки командного чата.
		r.Group(func(r chi.Router) {
			r.Use(mw.IsAdminJSON)
			r.Get("/telegram/team-chat", h.GetTeamChat())
			r.Post("/telegram/team-chat/test", h.TestTeamChat())
		})
	})
}

// startTelegram поднимает фоновые задачи бота: засев командного чата, проверку
// токена, поллер (polling) или регистрацию webhook (webhook). Все горутины
// слушают ctx и учтены в wg (§5.3).
func (s *APIServer) startTelegram(ctx context.Context, wg *sync.WaitGroup) {
	if s.tg == nil || !s.tg.enabled {
		slog.Info("telegram bot: disabled (mode off or empty token)")
		return
	}

	// NL-ответы считаются в фоне под корневым контекстом и общим wg: долгий
	// LLM-ответ (до 30 с) не висит на request-ctx вебхука и дожидается при
	// graceful shutdown (docs/NL_QUERIES.md §4.4, §5.3).
	if s.assistant != nil {
		s.tg.service.StartNL(ctx, wg)
	}

	// В stub-режиме не засеваем командный чат из TELEGRAM_TEAM_CHAT_ID: при общем
	// боте (решение №7) локальный запуск с прод-настройками иначе отправил бы
	// демо-напоминания в реальный рабочий чат (риск §11, finding). Демо-сообщения
	// помечены [DEMO]; командный чат в demo задаётся вручную через /bind.
	seedTeamChat := s.config.ReminderSource == "db"

	// Засев командного чата из TELEGRAM_TEAM_CHAT_ID, если активного ещё нет.
	if raw := s.config.TelegramTeamChatID; seedTeamChat && raw != "" {
		if id, err := strconv.ParseInt(raw, 10, 64); err != nil {
			slog.Error("telegram bot: bad TELEGRAM_TEAM_CHAT_ID", "value", raw, "error", err)
		} else {
			sctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			if err := s.tg.service.SeedTeamChat(sctx, id); err != nil {
				slog.Error("telegram bot: seed team chat", "error", err)
			}
			cancel()
		}
	}

	// Проверка токена: лог имени бота, падать не обязательно.
	gctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	if me, err := s.tg.client.GetMe(gctx); err != nil {
		slog.Error("telegram bot: getMe failed", "error", err)
	} else {
		slog.Info("telegram bot: authorized", "username", me.Username, "mode", s.tg.mode)
	}
	cancel()

	switch s.tg.mode {
	case "polling":
		poller := telegrambot.NewPoller(s.tg.service, s.tg.client)
		wg.Add(1)
		go func() {
			defer wg.Done()
			poller.Run(ctx)
		}()
	case "webhook":
		wctx, wcancel := context.WithTimeout(ctx, 10*time.Second)
		if err := s.tg.client.SetWebhook(wctx, telegram.SetWebhookParams{
			URL:            s.config.TelegramWebhookURL,
			SecretToken:    s.config.TelegramWebhookSecret,
			AllowedUpdates: []string{"message", "callback_query", "my_chat_member"},
		}); err != nil {
			slog.Error("telegram bot: setWebhook failed", "error", err)
		} else {
			slog.Info("telegram bot: webhook set", "url", s.config.TelegramWebhookURL)
		}
		wcancel()
	}

	// Движок напоминаний: одна горутина, первый тик сразу (§5.3). Работает в
	// обоих режимах приёма апдейтов.
	if s.tg.loop != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.tg.loop.Run(ctx)
		}()
	}
}
