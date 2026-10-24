package telegrambot

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/AVGsync/invoice-keeper/backend/internal/audit"
	"github.com/AVGsync/invoice-keeper/backend/internal/infrastructure/telegram"
	"github.com/AVGsync/invoice-keeper/backend/internal/model"
	"github.com/AVGsync/invoice-keeper/backend/internal/reminder"
)

// ErrBotDisabled — бот выключен (пустой токен/режим off), операция недоступна.
var ErrBotDisabled = errors.New("telegram bot is disabled")

// ErrMiniAppInvalidCredentials — неверный логин или пароль при входе в Mini App.
var ErrMiniAppInvalidCredentials = errors.New("invalid credentials")

// maxActiveCodes — сколько активных кодов привязки держим на пользователя.
const maxActiveCodes = 3

// sendTimeout — таймаут одиночного вызова Bot API из обработчика.
const sendTimeout = 5 * time.Second

// Параметры NL-ответов бота.
const (
	// nlMaxGoroutines — потолок одновременных NL-вопросов в боте, чтобы долгий LLM не плодил горутины и не блокировал цикл апдейтов.
	nlMaxGoroutines = 4
	// nlAskTimeout — запас поверх бюджета ассистента (Go ждёт 25 с).
	nlAskTimeout = 30 * time.Second
	// nlTypingInterval — как часто повторять индикатор «печатает…» (он гаснет ~5 с).
	nlTypingInterval = 4 * time.Second
	// tgMessageLimit — лимит длины одного сообщения Telegram.
	tgMessageLimit = 4096
	// nlQuestionMaxRunes — предел длины вопроса.
	nlQuestionMaxRunes = 500
)

// Config — параметры бота, нужные сервису.
type Config struct {
	Enabled        bool          // режим не off и задан токен
	BotUsername    string        // для deep link t.me/<username>
	MiniAppURL     string        // web_app-кнопка в личке, если задан
	LinkCodeTTL    time.Duration // TTL кода привязки
	BotToken       string        // для проверки подписи initData Mini App
	InitDataMaxAge time.Duration // максимальный возраст initData (auth_date)
}

// Service — сервис бота.
type Service struct {
	repo   Repository
	users  UserReader
	client BotAPI
	tokens TokenIssuer // выпуск access-токена Mini App; nil, когда бот выключен
	hasher Hasher
	cfg    Config
	// payments — операции движка напоминаний (кнопка «Оплачено», команды-сводки).
	payments *reminder.BotService

	// nl — NL-ассистент (порт NLAnswerer), подключается AttachNLAnswerer.
	nl        NLAnswerer
	nlRootCtx context.Context
	nlWG      *sync.WaitGroup
	// nlSem — семафор горутин NL-ответов; nlInflight — singleflight по userID.
	nlSem      chan struct{}
	nlInflight map[string]struct{}
	nlMu       sync.Mutex

	// audit — журнал действий; по умолчанию Nop (конструктор не меняется).
	audit audit.Recorder
}

// NewService собирает сервис бота.
func NewService(repo Repository, users UserReader, client BotAPI, tokens TokenIssuer, hasher Hasher, cfg Config) *Service {
	if cfg.LinkCodeTTL <= 0 {
		cfg.LinkCodeTTL = 10 * time.Minute
	}
	if cfg.InitDataMaxAge <= 0 {
		cfg.InitDataMaxAge = 10 * time.Minute
	}
	return &Service{repo: repo, users: users, client: client, tokens: tokens, hasher: hasher, cfg: cfg, audit: audit.Nop{}}
}

// SetAuditRecorder подключает журнал действий.
func (s *Service) SetAuditRecorder(r audit.Recorder) { s.audit = r }

// MiniAppResult — результат входа в Mini App: access-токен и профиль пользователя.
type MiniAppResult struct {
	AccessToken string
	ExpiresIn   int
	User        model.User
	// DMAvailable — может ли бот писать пользователю в личку (allows_write_to_pm из initData).
	DMAvailable bool
}

// MiniAppAuth выполняет вход в Mini App по уже существующей привязке.
func (s *Service) MiniAppAuth(ctx context.Context, initData string) (MiniAppResult, error) {
	id, err := s.validateInitData(initData)
	if err != nil {
		s.recordMiniAppAuthFailure(ctx, err)
		return MiniAppResult{}, err
	}

	link, err := s.repo.GetLinkByTGUserID(ctx, id.UserID)
	if err != nil {
		if errors.Is(err, model.ErrTelegramNotLinked) {
			s.recordMiniAppAuthFailure(ctx, err)
			return MiniAppResult{}, err
		}
		return MiniAppResult{}, fmt.Errorf("telegram service: miniapp auth: %w", err)
	}

	res, err := s.issueMiniApp(ctx, link.UserID, id.AllowsWriteToPM)
	if err == nil {
		s.audit.Record(ctx, audit.Entry{
			Action:     audit.ActionAuthMiniApp,
			ActorID:    link.UserID,
			EntityType: audit.EntityUser,
			EntityID:   link.UserID,
		})
	}
	return res, err
}

// recordMiniAppAuthFailure пишет неуспешный вход в Mini App (канал miniapp из контекста).
func (s *Service) recordMiniAppAuthFailure(ctx context.Context, err error) {
	reason := miniAppAuthReason(err)
	if reason == "" {
		return
	}
	s.audit.Record(ctx, audit.Entry{
		Action:     audit.ActionAuthMiniApp,
		Outcome:    audit.OutcomeFailure,
		EntityType: audit.EntityUser,
		Payload:    map[string]any{"reason": reason},
	})
}

// MiniAppLink привязывает Telegram к аккаунту по логину и паролю из Mini App и сразу выполняет вход.
func (s *Service) MiniAppLink(ctx context.Context, initData, login, password string) (MiniAppResult, error) {
	id, err := s.validateInitData(initData)
	if err != nil {
		s.recordMiniAppLinkFailure(ctx, "", err)
		return MiniAppResult{}, err
	}

	// Несуществующий логин и неверный пароль не различаем (защита от перечисления).
	user, err := s.users.GetUserByLogin(login, ctx)
	if err != nil {
		s.recordMiniAppLinkFailure(ctx, "", ErrMiniAppInvalidCredentials)
		return MiniAppResult{}, ErrMiniAppInvalidCredentials
	}
	if !s.hasher.CheckPassword(password, user.PasswordHash) {
		s.recordMiniAppLinkFailure(ctx, user.ID, ErrMiniAppInvalidCredentials)
		return MiniAppResult{}, ErrMiniAppInvalidCredentials
	}

	if err := s.repo.UpsertLink(ctx, user.ID, id.UserID, id.Username); err != nil {
		if errors.Is(err, model.ErrTelegramAlreadyLinked) {
			s.recordMiniAppLinkFailure(ctx, user.ID, err)
			return MiniAppResult{}, err
		}
		return MiniAppResult{}, fmt.Errorf("telegram service: miniapp link: %w", err)
	}

	s.audit.Record(ctx, audit.Entry{
		Action:     audit.ActionTelegramLink,
		ActorID:    user.ID,
		EntityType: audit.EntityTelegramLink,
		EntityID:   user.ID,
		Payload:    map[string]any{"via": "miniapp", "tg_username": id.Username},
	})
	return s.issueMiniApp(ctx, user.ID, id.AllowsWriteToPM)
}

// recordMiniAppLinkFailure пишет неуспешную привязку через Mini App.
func (s *Service) recordMiniAppLinkFailure(ctx context.Context, actorID string, err error) {
	reason := miniAppLinkReason(err)
	if reason == "" {
		return
	}
	entry := audit.Entry{
		Action:     audit.ActionTelegramLink,
		Outcome:    audit.OutcomeFailure,
		ActorID:    actorID,
		EntityType: audit.EntityTelegramLink,
		Payload:    map[string]any{"reason": reason},
	}
	if actorID != "" {
		entry.EntityID = actorID
	}
	s.audit.Record(ctx, entry)
}

// validateInitData — общая проверка подписи initData для обоих Mini App-потоков.
func (s *Service) validateInitData(initData string) (telegram.InitData, error) {
	if !s.cfg.Enabled || s.tokens == nil || s.hasher == nil {
		return telegram.InitData{}, ErrBotDisabled
	}
	return telegram.ValidateInitData(initData, s.cfg.BotToken, s.cfg.InitDataMaxAge, time.Now())
}

// issueMiniApp выпускает access-токен и собирает профиль пользователя.
func (s *Service) issueMiniApp(ctx context.Context, userID string, dmAvailable bool) (MiniAppResult, error) {
	token, expiresIn, err := s.tokens.IssueAccessForUser(ctx, userID)
	if err != nil {
		return MiniAppResult{}, fmt.Errorf("telegram service: miniapp issue token: %w", err)
	}
	user, err := s.users.GetUserByID(userID, ctx)
	if err != nil {
		return MiniAppResult{}, fmt.Errorf("telegram service: miniapp load user: %w", err)
	}
	return MiniAppResult{
		AccessToken: token,
		ExpiresIn:   expiresIn,
		User:        user,
		DMAvailable: dmAvailable,
	}, nil
}

// AttachPayments подключает операции движка напоминаний.
func (s *Service) AttachPayments(p *reminder.BotService) {
	s.payments = p
}

// AttachNLAnswerer подключает NL-ассистента.
func (s *Service) AttachNLAnswerer(a NLAnswerer) {
	s.nl = a
	s.nlSem = make(chan struct{}, nlMaxGoroutines)
	s.nlInflight = make(map[string]struct{})
}

// StartNL задаёт корневой контекст и WaitGroup для фоновых NL-ответов.
func (s *Service) StartNL(ctx context.Context, wg *sync.WaitGroup) {
	s.nlRootCtx = ctx
	s.nlWG = wg
}

// LinkStatus — состояние привязки для GET /me/telegram.
type LinkStatus struct {
	Linked     bool
	Username   string
	DMEnabled  bool
	BotBlocked bool
	LinkedAt   time.Time
}

// CreateLinkCode выдаёт deep link для привязки Telegram.
func (s *Service) CreateLinkCode(ctx context.Context, userID string) (deepLink string, expiresIn int, err error) {
	if !s.cfg.Enabled || s.cfg.BotUsername == "" {
		return "", 0, ErrBotDisabled
	}

	n, err := s.repo.CountActiveCodes(ctx, userID)
	if err != nil {
		return "", 0, fmt.Errorf("telegram service: create link code: %w", err)
	}
	if n >= maxActiveCodes {
		return "", 0, model.ErrTelegramTooManyCodes
	}

	code, err := newLinkCode()
	if err != nil {
		return "", 0, fmt.Errorf("telegram service: create link code: %w", err)
	}
	hash := hashCode(code)
	if _, err := s.repo.CreateLinkCode(ctx, userID, hash, time.Now().Add(s.cfg.LinkCodeTTL)); err != nil {
		return "", 0, fmt.Errorf("telegram service: create link code: %w", err)
	}

	deepLink = fmt.Sprintf("https://t.me/%s?start=link_%s", s.cfg.BotUsername, code)
	expiresIn = int(s.cfg.LinkCodeTTL.Seconds())

	// Сам код в журнал не пишем — только факт создания ссылки и её срок.
	s.audit.Record(ctx, audit.Entry{
		Action:     audit.ActionTelegramLinkCode,
		EntityType: audit.EntityTelegramLink,
		EntityID:   userID,
		Payload:    map[string]any{"expires_in": expiresIn},
	})
	return deepLink, expiresIn, nil
}

// GetLinkStatus возвращает состояние привязки пользователя.
func (s *Service) GetLinkStatus(ctx context.Context, userID string) (LinkStatus, error) {
	link, err := s.repo.GetLinkByUserID(ctx, userID)
	if errors.Is(err, model.ErrTelegramNotLinked) {
		return LinkStatus{Linked: false}, nil
	}
	if err != nil {
		return LinkStatus{}, fmt.Errorf("telegram service: get link status: %w", err)
	}
	return LinkStatus{
		Linked:     true,
		Username:   link.Username,
		DMEnabled:  link.DMEnabled,
		BotBlocked: link.BotBlocked,
		LinkedAt:   link.LinkedAt,
	}, nil
}

// Unlink удаляет привязку пользователя.
func (s *Service) Unlink(ctx context.Context, userID string) error {
	if err := s.repo.DeleteLink(ctx, userID); err != nil {
		if errors.Is(err, model.ErrTelegramNotLinked) {
			return err
		}
		return fmt.Errorf("telegram service: unlink: %w", err)
	}

	s.audit.Record(ctx, audit.Entry{
		Action:     audit.ActionTelegramUnlink,
		EntityType: audit.EntityTelegramLink,
		EntityID:   userID,
	})
	return nil
}

// GetTeamChat возвращает активный командный чат.
func (s *Service) GetTeamChat(ctx context.Context) (model.TelegramChat, error) {
	chat, err := s.repo.GetActiveChat(ctx)
	if err == nil {
		return chat, nil
	}
	if !errors.Is(err, model.ErrTelegramNoTeamChat) {
		return model.TelegramChat{}, err
	}
	last, lerr := s.repo.GetLastDeactivatedChat(ctx)
	if lerr != nil {
		return model.TelegramChat{}, model.ErrTelegramNoTeamChat
	}
	return last, nil
}

// SendTeamChatTest отправляет тестовое сообщение в командный чат.
func (s *Service) SendTeamChatTest(ctx context.Context) error {
	if !s.cfg.Enabled {
		return ErrBotDisabled
	}
	chat, err := s.repo.GetActiveChat(ctx)
	if err != nil {
		s.audit.Record(ctx, audit.Entry{
			Action:     audit.ActionTelegramTeamChatTest,
			Outcome:    audit.OutcomeFailure,
			EntityType: audit.EntityTelegramChat,
			Payload:    map[string]any{"reason": "no_team_chat"},
		})
		return err
	}
	chatIDStr := strconv.FormatInt(chat.TGChatID, 10)
	cctx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()
	_, err = s.client.SendMessage(cctx, telegram.SendMessageParams{
		ChatID:    chat.TGChatID,
		Text:      "Проверка связи: бот Invoice Keeper на месте. Напоминания будут приходить сюда.",
		ParseMode: "HTML",
	})
	if err != nil {
		s.audit.Record(ctx, audit.Entry{
			Action:     audit.ActionTelegramTeamChatTest,
			Outcome:    audit.OutcomeFailure,
			EntityType: audit.EntityTelegramChat,
			EntityID:   chatIDStr,
			Payload:    map[string]any{"reason": "send_failed"},
		})
		return fmt.Errorf("telegram service: team chat test: %w", err)
	}

	s.audit.Record(ctx, audit.Entry{
		Action:     audit.ActionTelegramTeamChatTest,
		EntityType: audit.EntityTelegramChat,
		EntityID:   chatIDStr,
	})
	return nil
}

// SeedTeamChat вставляет начальный командный чат из TELEGRAM_TEAM_CHAT_ID при старте, если активного ещё нет.
func (s *Service) SeedTeamChat(ctx context.Context, tgChatID int64) error {
	if _, err := s.repo.GetActiveChat(ctx); err == nil {
		return nil // активный чат уже есть
	} else if !errors.Is(err, model.ErrTelegramNoTeamChat) {
		return fmt.Errorf("telegram service: seed team chat: %w", err)
	}
	if err := s.repo.SeedChat(ctx, tgChatID, "group"); err != nil {
		return fmt.Errorf("telegram service: seed team chat: %w", err)
	}
	return nil
}

// HandleUpdate — единая точка обработки апдейта для polling и webhook.
func (s *Service) HandleUpdate(ctx context.Context, upd telegram.Update) {
	fresh, err := s.repo.MarkUpdateSeen(ctx, upd.UpdateID)
	if err != nil {
		slog.Error("telegram bot: dedup update", "error", err, "update_id", upd.UpdateID)
		return
	}
	if !fresh {
		return
	}

	switch {
	case upd.Message != nil:
		s.handleMessage(ctx, upd.Message)
	case upd.CallbackQuery != nil:
		s.handleCallback(ctx, upd.CallbackQuery)
	case upd.MyChatMember != nil:
		s.handleMyChatMember(ctx, upd.MyChatMember)
	}
}

func (s *Service) handleMessage(ctx context.Context, msg *telegram.Message) {
	if msg.MigrateToChatID != 0 {
		if err := s.repo.UpdateChatID(ctx, msg.Chat.ID, msg.MigrateToChatID); err != nil {
			slog.Error("telegram bot: migrate chat id", "error", err)
		}
		return
	}

	text := strings.TrimSpace(msg.Text)
	if text == "" {
		return
	}
	if !strings.HasPrefix(text, "/") {
		// Свободный текст — NL-вопрос только в личке.
		if msg.Chat.Type == "private" {
			s.handleFreeText(ctx, msg, text)
		}
		return
	}
	cmd, arg := parseCommand(text)

	switch cmd {
	case "/ask":
		s.handleAsk(ctx, msg, arg)
	case "/new", "/reset":
		s.handleReset(ctx, msg)
	case "/start":
		if code, ok := strings.CutPrefix(arg, "link_"); ok && code != "" {
			s.handleStartLink(ctx, msg, code)
			return
		}
		s.handleStart(ctx, msg)
	case "/help":
		s.reply(ctx, msg.Chat.ID, s.helpText())
	case "/unlink":
		s.handleUnlink(ctx, msg)
	case "/bind":
		s.handleBind(ctx, msg)
	case "/unbind":
		s.handleUnbind(ctx, msg)
	case "/today":
		s.handleSummary(ctx, msg, "today")
	case "/week":
		s.handleSummary(ctx, msg, "week")
	case "/overdue":
		s.handleSummary(ctx, msg, "overdue")
	case "/chatid":
		s.reply(ctx, msg.Chat.ID, fmt.Sprintf("ID этого чата: <code>%d</code>", msg.Chat.ID))
	default:
		if msg.Chat.Type == "private" {
			s.reply(ctx, msg.Chat.ID, s.helpText())
		}
	}
}

func (s *Service) handleStart(ctx context.Context, msg *telegram.Message) {
	if msg.Chat.Type != "private" {
		return
	}
	// Пользователь снова написал боту — снимаем флаг блокировки, если был.
	if msg.From != nil {
		if err := s.repo.SetBotBlocked(ctx, msg.From.ID, false); err != nil {
			slog.Error("telegram bot: clear bot_blocked", "error", err)
		}
	}

	var markup *telegram.InlineKeyboardMarkup
	if s.cfg.MiniAppURL != "" {
		markup = &telegram.InlineKeyboardMarkup{InlineKeyboard: [][]telegram.InlineKeyboardButton{{
			{Text: "Открыть в приложении", WebApp: &telegram.WebAppInfo{URL: s.cfg.MiniAppURL}},
		}}}
	}
	s.send(ctx, telegram.SendMessageParams{
		ChatID: msg.Chat.ID,
		Text: "Это бот Invoice Keeper. Чтобы привязать аккаунт, откройте профиль на сайте и нажмите " +
			"«Привязать Telegram» — бот пришлёт подтверждение. Команды: /help.",
		ParseMode:   "HTML",
		ReplyMarkup: markup,
	})
}

func (s *Service) handleStartLink(ctx context.Context, msg *telegram.Message, code string) {
	if msg.Chat.Type != "private" {
		s.reply(ctx, msg.Chat.ID, "Откройте ссылку привязки в личном чате с ботом.")
		return
	}
	if msg.From == nil {
		return
	}

	codeID, userID, err := s.repo.ClaimLinkCode(ctx, hashCode(code), msg.From.ID)
	if err != nil {
		if errors.Is(err, model.ErrTelegramCodeInvalid) {
			s.reply(ctx, msg.Chat.ID, "Ссылка привязки недействительна или устарела. Создайте новую в профиле на сайте.")
			return
		}
		slog.Error("telegram bot: claim link code", "error", err)
		return
	}

	// Повтор /start link при уже существующей привязке этого аккаунта к тому же пользователю — не ошибка.
	if link, err := s.repo.GetLinkByTGUserID(ctx, msg.From.ID); err == nil && link.UserID == userID {
		s.reply(ctx, msg.Chat.ID, "Этот Telegram уже привязан к вашему аккаунту.")
		return
	}

	user, err := s.users.GetUserByID(userID, ctx)
	if err != nil {
		slog.Error("telegram bot: resolve user for link", "error", err)
		s.reply(ctx, msg.Chat.ID, "Не удалось найти аккаунт. Попробуйте позже.")
		return
	}

	acct := html.EscapeString(user.FullName)
	login := html.EscapeString(user.Login)
	s.send(ctx, telegram.SendMessageParams{
		ChatID:    msg.Chat.ID,
		Text:      fmt.Sprintf("Привязать этот Telegram к аккаунту «%s» (%s)?", acct, login),
		ParseMode: "HTML",
		ReplyMarkup: &telegram.InlineKeyboardMarkup{InlineKeyboard: [][]telegram.InlineKeyboardButton{{
			{Text: "Да, привязать", CallbackData: "lc:" + codeID},
			{Text: "Отмена", CallbackData: "lx:" + codeID},
		}}},
	})
}

func (s *Service) handleUnlink(ctx context.Context, msg *telegram.Message) {
	if msg.Chat.Type != "private" || msg.From == nil {
		return
	}
	link, err := s.repo.GetLinkByTGUserID(ctx, msg.From.ID)
	if err != nil {
		s.reply(ctx, msg.Chat.ID, "К этому Telegram не привязан аккаунт.")
		return
	}
	if err := s.repo.DeleteLink(ctx, link.UserID); err != nil {
		slog.Error("telegram bot: unlink", "error", err)
		s.reply(ctx, msg.Chat.ID, "Не удалось отвязать. Попробуйте позже.")
		return
	}
	s.audit.Record(s.botCtx(ctx, link.UserID, ""), audit.Entry{
		Action:     audit.ActionTelegramUnlink,
		EntityType: audit.EntityTelegramLink,
		EntityID:   link.UserID,
	})
	s.reply(ctx, msg.Chat.ID, "Аккаунт отвязан. Кнопка «Оплачено», команды бота и вход в Mini App больше недоступны.")
}

func (s *Service) handleBind(ctx context.Context, msg *telegram.Message) {
	if msg.Chat.Type != "group" && msg.Chat.Type != "supergroup" {
		s.reply(ctx, msg.Chat.ID, "Команда /bind работает только в группе.")
		return
	}
	if msg.SenderChat != nil || msg.From == nil {
		s.reply(ctx, msg.Chat.ID, "Отправьте команду от своего имени, а не анонимно.")
		return
	}

	link, err := s.repo.GetLinkByTGUserID(ctx, msg.From.ID)
	if err != nil {
		s.reply(ctx, msg.Chat.ID, "Сначала привяжите Telegram в профиле на сайте, затем повторите /bind.")
		return
	}
	user, err := s.users.GetUserByID(link.UserID, ctx)
	if err != nil {
		slog.Error("telegram bot: bind: resolve user", "error", err)
		return
	}
	if user.Role != "admin" {
		s.reply(ctx, msg.Chat.ID, "Назначить командный чат может только администратор.")
		return
	}

	prev, hadPrev, err := s.repo.BindChat(ctx, msg.Chat.ID, msg.Chat.Type, msg.Chat.Title, link.UserID)
	if err != nil {
		slog.Error("telegram bot: bind chat", "error", err)
		s.reply(ctx, msg.Chat.ID, "Не удалось привязать чат. Попробуйте позже.")
		return
	}
	bindPayload := map[string]any{"title": msg.Chat.Title, "type": msg.Chat.Type}
	if hadPrev {
		bindPayload["previous_chat_id"] = prev.TGChatID
	}
	s.audit.Record(s.botCtx(ctx, link.UserID, user.Role), audit.Entry{
		Action:     audit.ActionTelegramTeamChatBind,
		EntityType: audit.EntityTelegramChat,
		EntityID:   strconv.FormatInt(msg.Chat.ID, 10),
		Payload:    bindPayload,
	})
	s.reply(ctx, msg.Chat.ID, "Готово. Этот чат теперь командный — напоминания будут приходить сюда.")
	if hadPrev {
		s.reply(ctx, prev.TGChatID, "Этот чат больше не командный: напоминания переключены на другую группу.")
	}
	if s.payments != nil {
		s.payments.ResetTeamQueue(ctx)
	}
}

func (s *Service) handleUnbind(ctx context.Context, msg *telegram.Message) {
	if msg.Chat.Type != "group" && msg.Chat.Type != "supergroup" {
		s.reply(ctx, msg.Chat.ID, "Команда /unbind работает только в группе.")
		return
	}
	if msg.SenderChat != nil || msg.From == nil {
		s.reply(ctx, msg.Chat.ID, "Отправьте команду от своего имени, а не анонимно.")
		return
	}
	link, err := s.repo.GetLinkByTGUserID(ctx, msg.From.ID)
	if err != nil {
		s.reply(ctx, msg.Chat.ID, "Эта команда доступна привязанному администратору.")
		return
	}
	user, err := s.users.GetUserByID(link.UserID, ctx)
	if err != nil {
		slog.Error("telegram bot: unbind: resolve user", "error", err)
		return
	}
	if user.Role != "admin" {
		s.reply(ctx, msg.Chat.ID, "Отвязать командный чат может только администратор.")
		return
	}

	if err := s.repo.DeactivateChat(ctx, msg.Chat.ID, "отвязан командой /unbind"); err != nil {
		if errors.Is(err, model.ErrTelegramNoTeamChat) {
			s.reply(ctx, msg.Chat.ID, "Этот чат не является командным.")
			return
		}
		slog.Error("telegram bot: unbind chat", "error", err)
		return
	}
	s.audit.Record(s.botCtx(ctx, link.UserID, user.Role), audit.Entry{
		Action:     audit.ActionTelegramTeamChatUnbind,
		EntityType: audit.EntityTelegramChat,
		EntityID:   strconv.FormatInt(msg.Chat.ID, 10),
	})
	s.reply(ctx, msg.Chat.ID, "Командный чат отвязан. Напоминания сюда больше не придут.")
}

func (s *Service) handleCallback(ctx context.Context, cb *telegram.CallbackQuery) {
	prefix, id, _ := strings.Cut(cb.Data, ":")
	switch prefix {
	case "lc":
		s.confirmLink(ctx, cb, id)
	case "lx":
		s.cancelLink(ctx, cb, id)
	case "p":
		s.handlePaidCallback(ctx, cb, id)
	default:
		s.answer(ctx, cb.ID, "Кнопка устарела.", false)
	}
}

// handlePaidCallback обрабатывает кнопку «Оплачено».
func (s *Service) handlePaidCallback(ctx context.Context, cb *telegram.CallbackQuery, occHex string) {
	if s.payments == nil {
		s.answer(ctx, cb.ID, "Кнопка недоступна.", false)
		return
	}
	link, err := s.repo.GetLinkByTGUserID(ctx, cb.From.ID)
	if err != nil {
		s.answer(ctx, cb.ID, "Привяжите Telegram в профиле на сайте.", true)
		return
	}
	var name, role string
	if u, uerr := s.users.GetUserByID(link.UserID, ctx); uerr == nil {
		name = u.FullName
		role = u.Role
	}
	bctx := s.botCtx(ctx, link.UserID, role)
	text, alert := s.payments.MarkPaidCallback(bctx, occHex, link.UserID, name)
	s.answer(ctx, cb.ID, text, alert)

	outcome := audit.OutcomeSuccess
	if alert {
		outcome = audit.OutcomeFailure
	}
	s.audit.Record(bctx, audit.Entry{
		Action:     audit.ActionBotPaidButton,
		Outcome:    outcome,
		EntityType: audit.EntityOccurrence,
		EntityID:   occurrenceIDFromHex(occHex),
		Payload:    map[string]any{"answer": text},
	})
}

// handleSummary отвечает read-only сводкой (/today, /week, /overdue).
func (s *Service) handleSummary(ctx context.Context, msg *telegram.Message, kind string) {
	if s.payments == nil {
		s.reply(ctx, msg.Chat.ID, "Сводка недоступна: движок напоминаний выключен.")
		return
	}
	if !s.summaryAllowed(ctx, msg) {
		s.reply(ctx, msg.Chat.ID,
			"Команда доступна в личке привязанному пользователю или в командном чате. "+
				"Привяжите Telegram в профиле на сайте.")
		return
	}
	var text string
	switch kind {
	case "today":
		text = s.payments.Today(ctx)
	case "week":
		text = s.payments.Week(ctx)
	case "overdue":
		text = s.payments.Overdue(ctx)
	}
	s.reply(ctx, msg.Chat.ID, text)
}

// summaryAllowed разрешает read-only команды только в активном командном чате или в личке привязанного пользователя.
func (s *Service) summaryAllowed(ctx context.Context, msg *telegram.Message) bool {
	switch msg.Chat.Type {
	case "group", "supergroup":
		chat, err := s.repo.GetActiveChat(ctx)
		return err == nil && chat.TGChatID == msg.Chat.ID
	case "private":
		if msg.From == nil {
			return false
		}
		_, err := s.repo.GetLinkByTGUserID(ctx, msg.From.ID)
		return err == nil
	default:
		return false
	}
}

func (s *Service) confirmLink(ctx context.Context, cb *telegram.CallbackQuery, codeID string) {
	userID, err := s.repo.ConfirmLinkCode(ctx, codeID, cb.From.ID)
	if err != nil {
		s.answer(ctx, cb.ID, "Ссылка недействительна или устарела.", true)
		s.editCallbackMessage(ctx, cb, "Привязка не выполнена: ссылка недействительна или устарела.")
		return
	}

	if err := s.repo.UpsertLink(ctx, userID, cb.From.ID, cb.From.Username); err != nil {
		if errors.Is(err, model.ErrTelegramAlreadyLinked) {
			s.audit.Record(s.botCtx(ctx, userID, ""), audit.Entry{
				Action:     audit.ActionTelegramLink,
				Outcome:    audit.OutcomeFailure,
				EntityType: audit.EntityTelegramLink,
				EntityID:   userID,
				Payload:    map[string]any{"reason": "already_linked"},
			})
			s.answer(ctx, cb.ID, "Этот Telegram уже привязан к другому аккаунту.", true)
			s.editCallbackMessage(ctx, cb, "Этот Telegram уже привязан к другому аккаунту. Отвяжите там командой /unlink.")
			return
		}
		slog.Error("telegram bot: confirm link upsert", "error", err)
		s.answer(ctx, cb.ID, "Ошибка привязки, попробуйте позже.", true)
		return
	}
	s.audit.Record(s.botCtx(ctx, userID, ""), audit.Entry{
		Action:     audit.ActionTelegramLink,
		EntityType: audit.EntityTelegramLink,
		EntityID:   userID,
		Payload:    map[string]any{"via": "bot", "tg_username": cb.From.Username},
	})
	s.answer(ctx, cb.ID, "Готово", false)
	s.editCallbackMessage(ctx, cb, "Готово. Telegram привязан: доступны кнопка «Оплачено», команды бота и вход в Mini App. Напоминания приходят в командный чат.")
}

func (s *Service) cancelLink(ctx context.Context, cb *telegram.CallbackQuery, codeID string) {
	if err := s.repo.CancelLinkCode(ctx, codeID, cb.From.ID); err != nil {
		slog.Error("telegram bot: cancel link", "error", err)
	}
	s.answer(ctx, cb.ID, "Отменено", false)
	s.editCallbackMessage(ctx, cb, "Привязка отменена.")
}

func (s *Service) handleMyChatMember(ctx context.Context, cm *telegram.ChatMemberUpdated) {
	wasPresent := cm.OldChatMember.IsPresent()
	isPresent := cm.NewChatMember.IsPresent()

	switch {
	case wasPresent && !isPresent:
		if err := s.repo.DeactivateChat(ctx, cm.Chat.ID, "бот удалён из чата"); err != nil &&
			!errors.Is(err, model.ErrTelegramNoTeamChat) {
			slog.Error("telegram bot: deactivate on removal", "error", err)
		} else if err == nil {
			slog.Error("telegram bot: removed from active team chat", "chat_id", cm.Chat.ID)
		}
	case !wasPresent && isPresent:
		if cm.Chat.Type == "group" || cm.Chat.Type == "supergroup" {
			s.reply(ctx, cm.Chat.ID, "Привет! Администратор, отправьте /bind, чтобы сделать этот чат командным.")
		}
	}
}

// reply отправляет короткое текстовое сообщение (HTML), ошибку только логирует.
func (s *Service) reply(ctx context.Context, chatID int64, text string) {
	s.send(ctx, telegram.SendMessageParams{ChatID: chatID, Text: text, ParseMode: "HTML"})
}

func (s *Service) send(ctx context.Context, p telegram.SendMessageParams) {
	cctx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()
	if _, err := s.client.SendMessage(cctx, p); err != nil {
		slog.Error("telegram bot: send message", "error", err, "chat_id", p.ChatID)
	}
}

func (s *Service) answer(ctx context.Context, callbackID, text string, alert bool) {
	cctx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()
	if err := s.client.AnswerCallbackQuery(cctx, telegram.AnswerCallbackParams{
		CallbackQueryID: callbackID, Text: text, ShowAlert: alert,
	}); err != nil {
		slog.Error("telegram bot: answer callback", "error", err)
	}
}

func (s *Service) editCallbackMessage(ctx context.Context, cb *telegram.CallbackQuery, text string) {
	if cb.Message == nil {
		return
	}
	cctx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()
	if _, err := s.client.EditMessageText(cctx, telegram.EditMessageTextParams{
		ChatID:    cb.Message.Chat.ID,
		MessageID: cb.Message.MessageID,
		Text:      text,
		ParseMode: "HTML",
	}); err != nil {
		slog.Debug("telegram bot: edit callback message", "error", err)
	}
}

// parseCommand разбирает "/cmd@bot arg..." на "/cmd" и остаток строки.
func parseCommand(text string) (cmd, arg string) {
	head, rest, _ := strings.Cut(text, " ")
	if at := strings.IndexByte(head, '@'); at >= 0 {
		head = head[:at]
	}
	return head, strings.TrimSpace(rest)
}

// newLinkCode — base64url от 18 случайных байт (24 символа без '=').
func newLinkCode() (string, error) {
	buf := make([]byte, 18)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// hashCode — hex(sha256(code)); в БД лежит только хеш.
func hashCode(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}

func (s *Service) helpText() string {
	var b strings.Builder
	b.WriteString("Команды бота Invoice Keeper:\n")
	b.WriteString("/start — начать, привязать аккаунт\n")
	b.WriteString("/unlink — отвязать аккаунт (в личке)\n")
	b.WriteString("/bind — сделать группу командным чатом (только админ, в группе)\n")
	b.WriteString("/unbind — отвязать командный чат (только админ, в группе)\n")
	b.WriteString("/today — платежи на сегодня\n")
	b.WriteString("/week — платежи на 7 дней\n")
	b.WriteString("/overdue — просроченные платежи\n")
	if s.nl != nil {
		b.WriteString("/ask &lt;вопрос&gt; — спросить ассистента (в личке можно просто написать вопрос)\n")
		b.WriteString("/new — начать новый диалог с ассистентом\n")
	}
	b.WriteString("/chatid — показать ID чата\n")
	b.WriteString("/help — эта справка")
	return b.String()
}
