package telegrambot

import (
	"context"
	"errors"
	"fmt"
	"html"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/AVGsync/invoice-keeper/backend/internal/infrastructure/telegram"
	"github.com/AVGsync/invoice-keeper/backend/internal/model"
	"github.com/AVGsync/invoice-keeper/backend/internal/service"
)

// handleFreeText обрабатывает свободный текст в личке как NL-вопрос (§4.4).
// Непривязанному пользователю — подсказка; при выключенной фиче молчим.
func (s *Service) handleFreeText(ctx context.Context, msg *telegram.Message, question string) {
	if s.nl == nil || msg.From == nil {
		return
	}
	link, err := s.repo.GetLinkByTGUserID(ctx, msg.From.ID)
	if err != nil {
		s.reply(ctx, msg.Chat.ID,
			"Привяжите Telegram в профиле на сайте, и я смогу отвечать на вопросы о платежах. Команды: /help.")
		return
	}
	s.dispatchNL(ctx, msg.Chat.ID, link.UserID, question)
}

// handleAsk обрабатывает команду /ask <вопрос> (§4.4, решение №8). В группе —
// только в командном чате; в любом чате — только привязанному пользователю,
// иначе короткая подсказка.
func (s *Service) handleAsk(ctx context.Context, msg *telegram.Message, arg string) {
	if s.nl == nil {
		if msg.Chat.Type == "private" {
			s.reply(ctx, msg.Chat.ID, "Ассистент сейчас недоступен.")
		}
		return
	}
	question := strings.TrimSpace(arg)
	if question == "" {
		s.reply(ctx, msg.Chat.ID, "Напишите вопрос после команды, например: /ask что просрочено?")
		return
	}

	switch msg.Chat.Type {
	case "group", "supergroup":
		// В группе отвечаем только в активном командном чате (решение №8).
		chat, err := s.repo.GetActiveChat(ctx)
		if err != nil || chat.TGChatID != msg.Chat.ID {
			s.reply(ctx, msg.Chat.ID, "Команда /ask работает в личке или в командном чате.")
			return
		}
	case "private":
		// ок
	default:
		return
	}

	if msg.From == nil {
		s.reply(ctx, msg.Chat.ID, "Отправьте вопрос от своего имени, а не анонимно.")
		return
	}
	link, err := s.repo.GetLinkByTGUserID(ctx, msg.From.ID)
	if err != nil {
		s.reply(ctx, msg.Chat.ID, "Привяжите Telegram в профиле на сайте, затем повторите вопрос.")
		return
	}
	s.dispatchNL(ctx, msg.Chat.ID, link.UserID, question)
}

// dispatchNL запускает NL-ответ в фоне, не блокируя цикл апдейтов. singleflight
// по userID и семафор горутин не дают долгому LLM накапливать работу (§4.4).
func (s *Service) dispatchNL(ctx context.Context, chatID int64, userID, question string) {
	if utf8.RuneCountInString(question) > nlQuestionMaxRunes {
		s.reply(ctx, chatID, "Вопрос слишком длинный, сократите до 500 символов.")
		return
	}

	s.nlMu.Lock()
	if _, busy := s.nlInflight[userID]; busy {
		s.nlMu.Unlock()
		s.reply(ctx, chatID, "Ещё думаю над прошлым вопросом, подождите немного.")
		return
	}
	select {
	case s.nlSem <- struct{}{}:
	default:
		s.nlMu.Unlock()
		s.reply(ctx, chatID, "Сейчас много вопросов, попробуйте через минуту.")
		return
	}
	s.nlInflight[userID] = struct{}{}
	s.nlMu.Unlock()

	// Фон под корневым контекстом (а не request-ctx вебхука): ответ до 30 с не
	// должен обрываться возвратом HTTP-обработчика и обязан дожиться при shutdown.
	root := s.nlRootCtx
	if root == nil {
		root = context.Background()
	}
	if s.nlWG != nil {
		s.nlWG.Add(1)
	}
	go func() {
		defer func() {
			if s.nlWG != nil {
				s.nlWG.Done()
			}
			s.nlMu.Lock()
			delete(s.nlInflight, userID)
			s.nlMu.Unlock()
			<-s.nlSem
		}()
		s.answerNL(root, chatID, userID, question)
	}()
}

// answerNL спрашивает ассистента, показывая «печатает…», и отправляет ответ.
func (s *Service) answerNL(root context.Context, chatID int64, userID, question string) {
	askCtx, cancel := context.WithTimeout(root, nlAskTimeout)
	defer cancel()

	// Индикатор «печатает…» в отдельной горутине; дожидаемся её завершения перед
	// выходом, чтобы фоновых обращений к Bot API не оставалось после ответа.
	stop := make(chan struct{})
	typingDone := make(chan struct{})
	go func() {
		defer close(typingDone)
		s.typingLoop(root, chatID, stop)
	}()

	ans, err := s.nl.Ask(askCtx, userID, "bot", question)
	close(stop)
	<-typingDone
	if err != nil {
		s.reply(root, chatID, nlErrorText(err))
		return
	}
	for _, chunk := range renderNLAnswer(ans) {
		s.send(root, telegram.SendMessageParams{
			ChatID:                chatID,
			Text:                  chunk,
			ParseMode:             "HTML",
			DisableWebPagePreview: true,
		})
	}
}

// typingLoop повторяет индикатор «печатает…», пока ждём ответа.
func (s *Service) typingLoop(ctx context.Context, chatID int64, stop <-chan struct{}) {
	send := func() {
		cctx, cancel := context.WithTimeout(ctx, sendTimeout)
		defer cancel()
		_ = s.client.SendChatAction(cctx, telegram.SendChatActionParams{ChatID: chatID, Action: "typing"})
	}
	send()
	t := time.NewTicker(nlTypingInterval)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ctx.Done():
			return
		case <-t.C:
			send()
		}
	}
}

// nlErrorText — короткое человекочитаемое сообщение об ошибке ассистента (§4.4).
func nlErrorText(err error) string {
	switch {
	case errors.Is(err, service.ErrAssistantRateLimited):
		return "Слишком много вопросов, попробуйте через минуту."
	case errors.Is(err, service.ErrAssistantTimeout):
		return "Долго думаю над ответом, попробуйте ещё раз."
	case errors.Is(err, service.ErrAssistantBadQuestion):
		return "Не понял вопрос. Спросите, например: что просрочено?"
	default:
		return "Ассистент недоступен, попробуйте позже или команду /overdue."
	}
}

// renderNLAnswer собирает текст ответа: сам ответ + компактный рендер блоков +
// подсказки. Весь текст из данных HTML-экранируется (§8.4), результат бьётся на
// сообщения по лимиту Telegram (4096).
func renderNLAnswer(ans model.AssistantAnswer) []string {
	var b strings.Builder
	if t := strings.TrimSpace(ans.Text); t != "" {
		b.WriteString(html.EscapeString(t))
	}
	for _, block := range ans.Blocks {
		if rendered := renderBlock(block); rendered != "" {
			b.WriteString("\n\n")
			b.WriteString(rendered)
		}
	}
	for _, note := range ans.Notes {
		if n := strings.TrimSpace(note); n != "" {
			b.WriteString("\n\n<i>")
			b.WriteString(html.EscapeString(n))
			b.WriteString("</i>")
		}
	}
	if len(ans.Followups) > 0 {
		b.WriteString("\n\nМожно спросить:")
		for _, f := range ans.Followups {
			if f = strings.TrimSpace(f); f != "" {
				b.WriteString("\n• ")
				b.WriteString(html.EscapeString(f))
			}
		}
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		out = "Готово."
	}
	return splitForTelegram(out, tgMessageLimit)
}

// renderBlock компактно (≤ 10 строк) рендерит один блок данных. Блоки приходят
// разобранным JSON, поэтому поля читаются защищённо.
func renderBlock(block map[string]any) string {
	const maxLines = 10
	switch asString(block["type"]) {
	case "occurrence_list":
		items := asSlice(block["items"])
		if len(items) == 0 {
			return ""
		}
		var lines []string
		if title := asString(block["title"]); title != "" {
			lines = append(lines, "<b>"+html.EscapeString(title)+"</b>")
		}
		for _, it := range itemsUpTo(items, maxLines) {
			m := asMap(it)
			line := "• " + html.EscapeString(asString(m["title"]))
			if disp := asString(m["display"]); disp != "" {
				line += " — " + html.EscapeString(disp)
			}
			if due := asString(m["due_date"]); due != "" {
				line += ", срок " + html.EscapeString(due)
			}
			lines = append(lines, line)
		}
		return joinWithMore(lines, len(items), maxLines)
	case "payment_list":
		items := asSlice(block["items"])
		if len(items) == 0 {
			return ""
		}
		var lines []string
		if title := asString(block["title"]); title != "" {
			lines = append(lines, "<b>"+html.EscapeString(title)+"</b>")
		}
		for _, it := range itemsUpTo(items, maxLines) {
			m := asMap(it)
			line := "• " + html.EscapeString(asString(m["title"]))
			if disp := asString(m["display"]); disp != "" {
				line += " — " + html.EscapeString(disp)
			}
			if who := asString(m["assignee_name"]); who != "" {
				line += ", " + html.EscapeString(who)
			}
			lines = append(lines, line)
		}
		return joinWithMore(lines, len(items), maxLines)
	case "people_list":
		items := asSlice(block["items"])
		if len(items) == 0 {
			return ""
		}
		var lines []string
		if title := asString(block["title"]); title != "" {
			lines = append(lines, "<b>"+html.EscapeString(title)+"</b>")
		}
		for _, it := range itemsUpTo(items, maxLines) {
			m := asMap(it)
			line := "• " + html.EscapeString(asString(m["full_name"]))
			if persona := asString(m["persona"]); persona != "" {
				line += " — " + html.EscapeString(persona)
			}
			lines = append(lines, line)
		}
		return joinWithMore(lines, len(items), maxLines)
	case "currency_totals":
		by := asMap(block["by_currency"])
		if len(by) == 0 {
			return ""
		}
		var lines []string
		if title := asString(block["title"]); title != "" {
			lines = append(lines, "<b>"+html.EscapeString(title)+"</b>")
		}
		// Фиксированный порядок валют, чтобы вывод был стабилен.
		for _, cur := range []string{"RUB", "USD", "EUR"} {
			v, ok := by[cur]
			if !ok {
				continue
			}
			m := asMap(v)
			disp := asString(m["display"])
			if disp == "" {
				disp = asString(m["total"])
			}
			lines = append(lines, "• "+cur+": "+html.EscapeString(disp))
		}
		if len(lines) == 0 {
			return ""
		}
		return strings.Join(lines, "\n")
	default:
		return ""
	}
}

func joinWithMore(lines []string, total, maxLines int) string {
	if total > maxLines {
		lines = append(lines, fmt.Sprintf("… и ещё %d", total-maxLines))
	}
	return strings.Join(lines, "\n")
}

func itemsUpTo(items []any, n int) []any {
	if len(items) > n {
		return items[:n]
	}
	return items
}

// splitForTelegram бьёт текст на сообщения ≤ limit, предпочитая разрыв по строке.
func splitForTelegram(text string, limit int) []string {
	if utf8.RuneCountInString(text) <= limit {
		return []string{text}
	}
	var out []string
	var cur strings.Builder
	curLen := 0
	flush := func() {
		if curLen > 0 {
			out = append(out, strings.TrimRight(cur.String(), "\n"))
			cur.Reset()
			curLen = 0
		}
	}
	for _, line := range strings.Split(text, "\n") {
		lineLen := utf8.RuneCountInString(line)
		switch {
		case lineLen >= limit:
			// Одна строка длиннее лимита: режем по рунам.
			flush()
			for _, part := range splitRunes(line, limit) {
				out = append(out, part)
			}
		case curLen+lineLen+1 > limit:
			flush()
			cur.WriteString(line)
			curLen = lineLen
		default:
			if curLen > 0 {
				cur.WriteByte('\n')
				curLen++
			}
			cur.WriteString(line)
			curLen += lineLen
		}
	}
	flush()
	return out
}

func splitRunes(s string, limit int) []string {
	var out []string
	runes := []rune(s)
	for len(runes) > 0 {
		n := limit
		if n > len(runes) {
			n = len(runes)
		}
		out = append(out, string(runes[:n]))
		runes = runes[n:]
	}
	return out
}

// --- защищённое чтение разобранного JSON ---

func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func asSlice(v any) []any {
	if s, ok := v.([]any); ok {
		return s
	}
	return nil
}

func asMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return nil
}
