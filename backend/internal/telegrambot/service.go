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
	"strings"
	"sync"
	"time"

	"github.com/AVGsync/invoice-keeper/backend/internal/infrastructure/telegram"
	"github.com/AVGsync/invoice-keeper/backend/internal/model"
	"github.com/AVGsync/invoice-keeper/backend/internal/reminder"
)

// ErrBotDisabled — бот выключен (пустой токен/режим off), операция недоступна.
var ErrBotDisabled = errors.New("telegram bot is disabled")

// ErrMiniAppInvalidCredentials — неверный логин или пароль при входе в Mini App.
// Логин и пароль не различаются в ответе, чтобы не раскрывать существование аккаунта.
var ErrMiniAppInvalidCredentials = errors.New("invalid credentials")

// maxActiveCodes — сколько активных кодов привязки держим на пользователя (§3.1).
const maxActiveCodes = 3

// sendTimeout — таймаут одиночного вызова Bot API из обработчика (§5.4).
const sendTimeout = 5 * time.Second

// Параметры NL-ответов бота (docs/NL_QUERIES.md §4.4).
const (
	// nlMaxGoroutines — потолок одновременных NL-вопросов в боте, чтобы долгий
	// LLM не плодил горутины и не блокировал цикл апдейтов.
	nlMaxGoroutines = 4
	// nlAskTimeout — запас поверх бюджета ассистента (Go ждёт 25 с).
	nlAskTimeout = 30 * time.Second
	// nlTypingInterval — как часто повторять индикатор «печатает…» (он гаснет ~5 с).
	nlTypingInterval = 4 * time.Second
	// tgMessageLimit — лимит длины одного сообщения Telegram.
	tgMessageLimit = 4096
	// nlQuestionMaxRunes — предел длины вопроса (как в HTTP-контракте §9.1).
	nlQuestionMaxRunes = 500
)

// Config — параметры бота, нужные сервису.
type Config struct {
	Enabled        bool          // режим не off и задан токен
	BotUsername    string        // для deep link t.me/<username>
	MiniAppURL     string        // web_app-кнопка в личке, если задан
	LinkCodeTTL    time.Duration // TTL кода привязки
	BotToken       string        // для проверки подписи initData Mini App (§3.3)
	InitDataMaxAge time.Duration // максимальный возраст initData (auth_date)
}

// Service — сервис бота.
type Service struct {
	repo   Repository
	users  UserReader
	client BotAPI
	tokens TokenIssuer // выпуск access-токена Mini App; nil, когда бот выключен
	hasher Hasher      // проверка пароля при входе в Mini App; nil, когда бот выключен
	cfg    Config
	// payments — операции движка напоминаний (кнопка «Оплачено», команды-сводки).
	// nil, пока движок не подключён (AttachPayments) или бот выключен.
	payments *reminder.BotService

	// nl — NL-ассистент (порт NLAnswerer), подключается AttachNLAnswerer.
	// nil, когда фича выключена (NLQ_ENABLED=false): свободный текст и /ask тогда
	// обрабатываются как прежде.
	nl NLAnswerer
	// nlRootCtx/nlWG — корневой контекст и WaitGroup фоновых воркеров (StartNL):
	// NL-ответы считаются в фоне под ними, а не под request-ctx вебхука, и
	// завершаются при graceful shutdown (§5.3).
	nlRootCtx context.Context
	nlWG      *sync.WaitGroup
	// nlSem — семафор горутин NL-ответов; nlInflight — singleflight по userID.
	nlSem      chan struct{}
	nlInflight map[string]struct{}
	nlMu       sync.Mutex
}

// NewService собирает сервис бота. tokens и hasher нужны только входу в Mini App
// и могут быть nil, когда бот выключен (методы MiniApp* вернут ErrBotDisabled).
func NewService(repo Repository, users UserReader, client BotAPI, tokens TokenIssuer, hasher Hasher, cfg Config) *Service {
	if cfg.LinkCodeTTL <= 0 {
		cfg.LinkCodeTTL = 10 * time.Minute
	}
	if cfg.InitDataMaxAge <= 0 {
		cfg.InitDataMaxAge = 10 * time.Minute
	}
	return &Service{repo: repo, users: users, client: client, tokens: tokens, hasher: hasher, cfg: cfg}
}

// MiniAppResult — результат входа в Mini App: access-токен и профиль пользователя.
type MiniAppResult struct {
	AccessToken string
	ExpiresIn   int
	User        model.User
	// DMAvailable — может ли бот писать пользователю в личку (allows_write_to_pm из
	// initData). Фронт по нему решает, звать ли requestWriteAccess (§3.3).
	DMAvailable bool
}

// MiniAppAuth выполняет вход в Mini App по уже существующей привязке (§3.3).
// Проверяет подпись initData, находит привязку по tg_user_id и выпускает
// access-токен. Ошибки: telegram.ErrInitDataInvalid / telegram.ErrInitDataStale
// (неверная или устаревшая initData), model.ErrTelegramNotLinked (привязки нет),
// ErrBotDisabled (бот выключен).
func (s *Service) MiniAppAuth(ctx context.Context, initData string) (MiniAppResult, error) {
	id, err := s.validateInitData(initData)
	if err != nil {
		return MiniAppResult{}, err
	}

	link, err := s.repo.GetLinkByTGUserID(ctx, id.UserID)
	if err != nil {
		if errors.Is(err, model.ErrTelegramNotLinked) {
			return MiniAppResult{}, err
		}
		return MiniAppResult{}, fmt.Errorf("telegram service: miniapp auth: %w", err)
	}

	return s.issueMiniApp(ctx, link.UserID, id.AllowsWriteToPM)
}

// MiniAppLink привязывает Telegram к аккаунту по логину и паролю из Mini App и
// сразу выполняет вход (§3.3). Пароль вводится один раз. Ошибки:
// telegram.ErrInitDataInvalid / telegram.ErrInitDataStale, ErrMiniAppInvalidCredentials
// (неверный логин/пароль), model.ErrTelegramAlreadyLinked (этот Telegram уже за
// другим аккаунтом), ErrBotDisabled.
func (s *Service) MiniAppLink(ctx context.Context, initData, login, password string) (MiniAppResult, error) {
	id, err := s.validateInitData(initData)
	if err != nil {
		return MiniAppResult{}, err
	}

	// Несуществующий логин и неверный пароль не различаем (защита от перечисления).
	user, err := s.users.GetUserByLogin(login, ctx)
	if err != nil {
		return MiniAppResult{}, ErrMiniAppInvalidCredentials
	}
	if !s.hasher.CheckPassword(password, user.PasswordHash) {
		return MiniAppResult{}, ErrMiniAppInvalidCredentials
	}

	if err := s.repo.UpsertLink(ctx, user.ID, id.UserID, id.Username); err != nil {
		if errors.Is(err, model.ErrTelegramAlreadyLinked) {
			return MiniAppResult{}, err
		}
		return MiniAppResult{}, fmt.Errorf("telegram service: miniapp link: %w", err)
	}

	return s.issueMiniApp(ctx, user.ID, id.AllowsWriteToPM)
}

// validateInitData — общая проверка подписи initData для обоих Mini App-потоков.
// Сырые initData не логируются ни на каком уровне (§3.3).
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

// AttachPayments подключает операции движка напоминаний (§6.3). Вызывается из
// склейки apiserver после сборки движка.
func (s *Service) AttachPayments(p *reminder.BotService) {
	s.payments = p
}

// AttachNLAnswerer подключает NL-ассистента (порт NLAnswerer, §4.4). Вызывается
// из склейки apiserver при NLQ_ENABLED=true. Инициализирует семафор горутин и
// карту singleflight.
func (s *Service) AttachNLAnswerer(a NLAnswerer) {
	s.nl = a
	s.nlSem = make(chan struct{}, nlMaxGoroutines)
	s.nlInflight = make(map[string]struct{})
}

// StartNL задаёт корневой контекст и WaitGroup для фоновых NL-ответов.
// Вызывается из startTelegram с rootCtx и общим wg, чтобы ответы (до 30 с) не
// висели на request-ctx вебхука и завершались при остановке (§5.3).
func (s *Service) StartNL(ctx context.Context, wg *sync.WaitGroup) {
	s.nlRootCtx = ctx
	s.nlWG = wg
}

// ---- REST-операции (вызываются из HTTP-хендлера) ----

// LinkStatus — состояние привязки для GET /me/telegram.
type LinkStatus struct {
	Linked     bool
	Username   string
	DMEnabled  bool
	BotBlocked bool
	LinkedAt   time.Time
}

// CreateLinkCode выдаёт deep link для привязки Telegram. Возвращает ErrBotDisabled,
// если бот выключен, и model.ErrTelegramTooManyCodes при превышении лимита кодов.
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
	return deepLink, int(s.cfg.LinkCodeTTL.Seconds()), nil
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

// Unlink удаляет привязку пользователя. Возвращает model.ErrTelegramNotLinked,
// если привязки не было.
func (s *Service) Unlink(ctx context.Context, userID string) error {
	if err := s.repo.DeleteLink(ctx, userID); err != nil {
		if errors.Is(err, model.ErrTelegramNotLinked) {
			return err
		}
		return fmt.Errorf("telegram service: unlink: %w", err)
	}
	return nil
}

// GetTeamChat возвращает активный командный чат. Если активного нет, но чат
// когда-то был привязан, возвращает последний деактивированный (IsActive=false)
// с last_error — чтобы админ в UI видел, ПОЧЕМУ напоминания встали (§5.4).
// model.ErrTelegramNoTeamChat возвращается только если чата не было вовсе.
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
// model.ErrTelegramNoTeamChat — чат не привязан; ErrBotDisabled — бот выключен;
// прочие ошибки — ошибка Bot API.
func (s *Service) SendTeamChatTest(ctx context.Context) error {
	if !s.cfg.Enabled {
		return ErrBotDisabled
	}
	chat, err := s.repo.GetActiveChat(ctx)
	if err != nil {
		return err
	}
	cctx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()
	_, err = s.client.SendMessage(cctx, telegram.SendMessageParams{
		ChatID:    chat.TGChatID,
		Text:      "Проверка связи: бот Invoice Keeper на месте. Напоминания будут приходить сюда.",
		ParseMode: "HTML",
	})
	if err != nil {
		return fmt.Errorf("telegram service: team chat test: %w", err)
	}
	return nil
}

// SeedTeamChat вставляет начальный командный чат из TELEGRAM_TEAM_CHAT_ID при
// старте, если активного ещё нет (§4). Пустой id игнорируется вызывающим.
func (s *Service) SeedTeamChat(ctx context.Context, tgChatID int64) error {
	if _, err := s.repo.GetActiveChat(ctx); err == nil {
		return nil // активный чат уже есть
	} else if !errors.Is(err, model.ErrTelegramNoTeamChat) {
		return fmt.Errorf("telegram service: seed team chat: %w", err)
	}
	// Тип группы на старте неизвестен; группа — безопасное значение, уточнится
	// при первом /bind или сервисном апдейте.
	if err := s.repo.SeedChat(ctx, tgChatID, "group"); err != nil {
		return fmt.Errorf("telegram service: seed team chat: %w", err)
	}
	return nil
}

// ---- Приём апдейтов ----

// HandleUpdate — единая точка обработки апдейта для polling и webhook (§6.1).
// Дедуп по update_id; повторный апдейт игнорируется. Ошибки обработки
// логируются, наружу не пробрасываются: webhook всегда отвечает 200.
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
	// Сервисное migrate_to_chat_id: группа стала супергруппой.
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
		// Свободный текст — NL-вопрос только в личке (§4.4, решение №8).
		if msg.Chat.Type == "private" {
			s.handleFreeText(ctx, msg, text)
		}
		return
	}
	cmd, arg := parseCommand(text)

	switch cmd {
	case "/ask":
		s.handleAsk(ctx, msg, arg)
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
		// Незнакомую команду молча пропускаем в группах, в личке подсказываем.
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

	// Повтор /start link при уже существующей привязке этого аккаунта к тому же
	// пользователю — не ошибка (§3.1).
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
	s.reply(ctx, msg.Chat.ID, "Аккаунт отвязан. Кнопка «Оплачено», команды бота и вход в Mini App больше недоступны.")
}

func (s *Service) handleBind(ctx context.Context, msg *telegram.Message) {
	if msg.Chat.Type != "group" && msg.Chat.Type != "supergroup" {
		s.reply(ctx, msg.Chat.ID, "Команда /bind работает только в группе.")
		return
	}
	// Анонимный админ или канал: from.id отсутствует/непривязываем.
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
	s.reply(ctx, msg.Chat.ID, "Готово. Этот чат теперь командный — напоминания будут приходить сюда.")
	if hadPrev {
		s.reply(ctx, prev.TGChatID, "Этот чат больше не командный: напоминания переключены на другую группу.")
	}
	// Отложенные из-за отсутствия чата напоминания отправятся на ближайшем тике.
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

// handlePaidCallback обрабатывает кнопку «Оплачено» (§6.3). Авторизация по
// from.id: кнопку может нажать только привязанный пользователь, права
// (ответственный или админ) проверяет Payer.
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
	var name string
	if u, uerr := s.users.GetUserByID(link.UserID, ctx); uerr == nil {
		name = u.FullName
	}
	text, alert := s.payments.MarkPaidCallback(ctx, occHex, link.UserID, name)
	s.answer(ctx, cb.ID, text, alert)
}

// handleSummary отвечает read-only сводкой (/today, /week, /overdue). Сводка —
// это суммы, валюты и сроки по всем неоплаченным платежам компании, поэтому
// доступна только (а) в личке привязанному пользователю или (б) в активном
// командном чате. В чужой группе и в личке без привязки — отказ (решение №1).
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

// summaryAllowed разрешает read-only команды только в активном командном чате
// или в личке привязанного пользователя (§6.2, решение №1).
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
			s.answer(ctx, cb.ID, "Этот Telegram уже привязан к другому аккаунту.", true)
			s.editCallbackMessage(ctx, cb, "Этот Telegram уже привязан к другому аккаунту. Отвяжите там командой /unlink.")
			return
		}
		slog.Error("telegram bot: confirm link upsert", "error", err)
		s.answer(ctx, cb.ID, "Ошибка привязки, попробуйте позже.", true)
		return
	}
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
		// Бота удалили из чата: деактивируем, если это был командный чат.
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

// ---- Вспомогательное ----

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
	}
	b.WriteString("/chatid — показать ID чата\n")
	b.WriteString("/help — эта справка")
	return b.String()
}
