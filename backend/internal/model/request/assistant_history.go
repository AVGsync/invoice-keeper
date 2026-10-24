package request

import (
	"errors"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// conversationRe повторяет CHECK таблицы assistant_turns: область диалога.
var conversationRe = regexp.MustCompile(`^(web|miniapp|tg:dm|tg:-?[0-9]{1,20})$`)

// AssistantHistoryQuery — разобранные параметры GET/DELETE истории.
type AssistantHistoryQuery struct {
	// Channel — web | miniapp | bot (для DELETE ещё all).
	Channel string
	// Conversation — область диалога; пусто при All.
	Conversation string
	// Limit — для GET (1..50, дефолт 20).
	Limit int
	// All — DELETE channel=all: все разговоры пользователя (включая бот).
	All bool
}

// ParseAssistantHistoryGet разбирает параметры GET /assistant/history.
func ParseAssistantHistoryGet(q url.Values) (AssistantHistoryQuery, error) {
	out, err := parseHistoryChannelConversation(q, false)
	if err != nil {
		return out, err
	}
	out.Limit = 20
	if raw := strings.TrimSpace(q.Get("limit")); raw != "" {
		n, convErr := strconv.Atoi(raw)
		if convErr != nil || n < 1 || n > 50 {
			return out, errors.New("limit must be between 1 and 50")
		}
		out.Limit = n
	}
	return out, nil
}

// ParseAssistantHistoryDelete разбирает параметры DELETE /assistant/history.
func ParseAssistantHistoryDelete(q url.Values) (AssistantHistoryQuery, error) {
	return parseHistoryChannelConversation(q, true)
}

// parseHistoryChannelConversation разбирает channel и conversation.
func parseHistoryChannelConversation(q url.Values, allowAll bool) (AssistantHistoryQuery, error) {
	var out AssistantHistoryQuery

	ch := strings.TrimSpace(q.Get("channel"))
	if ch == "" {
		ch = "web"
	}
	switch ch {
	case "web", "miniapp", "bot":
	case "all":
		if allowAll {
			out.Channel = "all"
			out.All = true
			return out, nil
		}
		fallthrough
	default:
		if allowAll {
			return out, errors.New("channel must be web, miniapp, bot or all")
		}
		return out, errors.New("channel must be web, miniapp or bot")
	}
	out.Channel = ch

	conv := strings.TrimSpace(q.Get("conversation"))
	switch ch {
	case "web", "miniapp":
		if conv != "" && conv != ch {
			return out, errors.New("conversation is invalid")
		}
		out.Conversation = ch
	case "bot":
		// Для бота дефолт tg:dm; иначе — валидная область tg:dm | tg:<chat_id>.
		if conv == "" {
			conv = "tg:dm"
		}
		if !conversationRe.MatchString(conv) || !strings.HasPrefix(conv, "tg:") {
			return out, errors.New("conversation is invalid")
		}
		out.Conversation = conv
	}
	return out, nil
}
