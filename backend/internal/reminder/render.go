package reminder

import (
	"fmt"
	"html"
	"strings"

	"github.com/AVGsync/invoice-keeper/backend/internal/clock"
	"github.com/AVGsync/invoice-keeper/backend/internal/infrastructure/telegram"
	"github.com/AVGsync/invoice-keeper/backend/internal/model"
)

// maxTitleLen — заголовок обрезается, чтобы не раздувать сообщение (§5.5).
const maxTitleLen = 200

// demoPrefix помечает сообщения, отправленные в режиме демо-заглушки
// (REMINDER_SOURCE=stub), чтобы их не путали с настоящими напоминаниями, если
// бот с общим токеном случайно запущен локально (риск §11, решение №7).
const demoPrefix = "[DEMO] "

// reminderText собирает текст напоминания (HTML). kind — due_soon или overdue.
func reminderText(occ model.ReminderOccurrence, today clock.Date, kind string) string {
	var b strings.Builder
	if kind == model.NotificationKindOverdue {
		b.WriteString("Просрочен платёж\n")
	} else {
		b.WriteString("Напоминание об оплате\n")
	}
	b.WriteString("<b>")
	b.WriteString(html.EscapeString(truncate(occ.Title, maxTitleLen)))
	b.WriteString("</b>\n")
	b.WriteString("Сумма: ")
	b.WriteString(html.EscapeString(FormatMoney(occ.Amount, occ.Currency)))
	b.WriteString("\n")
	if kind == model.NotificationKindOverdue {
		b.WriteString("Срок был: ")
	} else {
		b.WriteString("Срок: ")
	}
	b.WriteString(formatDate(occ.DueDate))
	b.WriteString(" (")
	b.WriteString(html.EscapeString(relativeDay(today, occ.DueDate)))
	b.WriteString(")\n")
	b.WriteString("Ответственный: ")
	b.WriteString(html.EscapeString(assigneeName(occ)))
	return b.String()
}

// paidText — текст после отметки «Оплачено» (§5.5). actorName может быть пустым
// (при reconcile мы не знаем, кто отметил, и показываем только дату).
func paidText(occ model.ReminderOccurrence, actorName string) string {
	var who string
	if actorName != "" {
		who = html.EscapeString(actorName) + ", "
	}
	date := "—"
	if occ.PaidDate != nil {
		date = formatDate(*occ.PaidDate)
	}
	return fmt.Sprintf("Оплачено: %s%s\n<b>%s</b>\nСумма: %s",
		who, date,
		html.EscapeString(truncate(occ.Title, maxTitleLen)),
		html.EscapeString(FormatMoney(occ.Amount, occ.Currency)))
}

// endedText — текст, когда платёж завершён (§5.6).
func endedText(occ model.ReminderOccurrence) string {
	return fmt.Sprintf("Платёж завершён\n<b>%s</b>",
		html.EscapeString(truncate(occ.Title, maxTitleLen)))
}

// reminderKeyboard — клавиатура напоминания: кнопка «Оплачено» (callback p:<hex>)
// и, если возможно построить ссылку, кнопка «Открыть» на Mini App.
// callback_data = p:<uuid без дефисов>.
func reminderKeyboard(occID string, cfg Config) *telegram.InlineKeyboardMarkup {
	row := []telegram.InlineKeyboardButton{
		{Text: "✅ Оплачено", CallbackData: "p:" + uuidToHex(occID)},
	}
	if url, ok := miniAppStartURL(cfg, occID); ok {
		row = append(row, telegram.InlineKeyboardButton{Text: "Открыть", URL: url})
	}
	return &telegram.InlineKeyboardMarkup{InlineKeyboard: [][]telegram.InlineKeyboardButton{row}}
}

// miniAppStartURL строит Direct Link Mini App в домене t.me, чтобы Telegram
// открыл Mini App с переданной initData (§3.3, решения №6, №8). Параметр
// startapp интерпретируют только ссылки t.me, поэтому plain-https на /tg здесь
// не годится: он открылся бы во внешнем браузере без Telegram.WebApp.
//
//	t.me/<bot>/<short_name>?startapp=occ_<hex>  — зарегистрированное Mini App (BotFather /newapp)
//	t.me/<bot>?startapp=occ_<hex>               — основное Mini App бота
//
// Без TELEGRAM_BOT_USERNAME ссылку собрать нельзя — кнопка не добавляется.
func miniAppStartURL(cfg Config, occID string) (string, bool) {
	if cfg.BotUsername == "" {
		return "", false
	}
	base := "https://t.me/" + cfg.BotUsername
	if cfg.MiniAppShortName != "" {
		base += "/" + cfg.MiniAppShortName
	}
	return base + "?startapp=occ_" + uuidToHex(occID), true
}

func assigneeName(occ model.ReminderOccurrence) string {
	if occ.AssigneeID == nil || strings.TrimSpace(occ.AssigneeName) == "" {
		return "не назначен"
	}
	return occ.AssigneeName
}

// formatDate — дата в формате dd.mm.yyyy.
func formatDate(d clock.Date) string {
	return fmt.Sprintf("%02d.%02d.%04d", d.D, int(d.M), d.Y)
}

// relativeDay — «сегодня», «завтра», «через N дн.», «просрочено на N дн.».
func relativeDay(today, due clock.Date) string {
	diff := dayDiff(today, due) // due − today
	switch {
	case diff == 0:
		return "сегодня"
	case diff == 1:
		return "завтра"
	case diff == -1:
		return "вчера"
	case diff > 1:
		return "через " + pluralDays(diff)
	default:
		return "просрочено на " + pluralDays(-diff)
	}
}

// pluralDays склоняет «день/дня/дней».
func pluralDays(n int) string {
	word := "дней"
	switch {
	case n%100 >= 11 && n%100 <= 14:
		word = "дней"
	case n%10 == 1:
		word = "день"
	case n%10 >= 2 && n%10 <= 4:
		word = "дня"
	}
	return fmt.Sprintf("%d %s", n, word)
}

func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}

// FormatMoney форматирует десятичную строку суммы по валюте без float (§5.5).
// RUB -> «1 500,00 ₽», USD -> «$1,500.00», EUR -> «1 500,00 €».
func FormatMoney(amount, currency string) string {
	neg, intPart, frac := splitAmount(amount)

	var grouped string
	switch currency {
	case "USD":
		grouped = groupThousands(intPart, ',')
	default: // RUB, EUR и прочее — группировка пробелом
		grouped = groupThousands(intPart, ' ')
	}

	var dec byte = ','
	if currency == "USD" {
		dec = '.'
	}
	body := grouped + string(dec) + frac
	if neg {
		body = "-" + body
	}

	switch currency {
	case "USD":
		return "$" + body
	case "EUR":
		return body + " €"
	case "RUB":
		return body + " ₽"
	default:
		if currency == "" {
			return body
		}
		return body + " " + currency
	}
}

// splitAmount разбирает "1500.5"/"1500"/"-10.999" на знак, целую часть и ровно
// две цифры дробной (с округлением вниз/обрезанием). Без float.
func splitAmount(amount string) (neg bool, intPart, frac string) {
	s := strings.TrimSpace(amount)
	if s == "" {
		return false, "0", "00"
	}
	if strings.HasPrefix(s, "-") {
		neg = true
		s = s[1:]
	}
	intPart, fracRaw, _ := strings.Cut(s, ".")
	if intPart == "" {
		intPart = "0"
	}
	// Оставляем только цифры в целой части (защита от мусора).
	intPart = strings.TrimLeft(digitsOnly(intPart), "0")
	if intPart == "" {
		intPart = "0"
	}
	fracRaw = digitsOnly(fracRaw)
	switch {
	case len(fracRaw) >= 2:
		frac = fracRaw[:2]
	case len(fracRaw) == 1:
		frac = fracRaw + "0"
	default:
		frac = "00"
	}
	return neg, intPart, frac
}

func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// groupThousands группирует целую часть по три разряда разделителем sep.
func groupThousands(intPart string, sep byte) string {
	n := len(intPart)
	if n <= 3 {
		return intPart
	}
	var b strings.Builder
	head := n % 3
	if head > 0 {
		b.WriteString(intPart[:head])
	}
	for i := head; i < n; i += 3 {
		if b.Len() > 0 {
			b.WriteByte(sep)
		}
		b.WriteString(intPart[i : i+3])
	}
	return b.String()
}

// dayDiff возвращает количество календарных дней (to − from).
func dayDiff(from, to clock.Date) int {
	// Опираемся на AddDays: ищем сдвиг, но проще — через подсчёт по времени.
	// AddDays нормализует через time, поэтому используем вспомогательный счёт.
	return daysSinceEpoch(to) - daysSinceEpoch(from)
}

// daysSinceEpoch — число дней от фиксированной даты (алгоритм без time.Time,
// чтобы не зависеть от зоны). Пролептический григорианский календарь.
func daysSinceEpoch(d clock.Date) int {
	y := d.Y
	m := int(d.M)
	if m <= 2 {
		y--
		m += 12
	}
	era := y / 400
	yoe := y - era*400
	doy := (153*(m-3)+2)/5 + d.D - 1
	doe := yoe*365 + yoe/4 - yoe/100 + doy
	return era*146097 + doe - 719468
}

// uuidToHex убирает дефисы из uuid (36 -> 32 символа) для callback_data.
func uuidToHex(id string) string {
	return strings.ReplaceAll(id, "-", "")
}

// hexToUUID восстанавливает uuid с дефисами из 32 hex-символов. Пустая строка,
// если формат не тот (callback_data не доверяем, §6.3).
func hexToUUID(hex string) string {
	if len(hex) != 32 {
		return ""
	}
	for _, r := range hex {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return ""
		}
	}
	return hex[0:8] + "-" + hex[8:12] + "-" + hex[12:16] + "-" + hex[16:20] + "-" + hex[20:32]
}
