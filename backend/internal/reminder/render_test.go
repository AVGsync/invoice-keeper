package reminder

import (
	"strings"
	"testing"

	"github.com/AVGsync/invoice-keeper/backend/internal/clock"
	"github.com/AVGsync/invoice-keeper/backend/internal/model"
)

func TestFormatMoney(t *testing.T) {
	cases := []struct {
		amount, currency, want string
	}{
		{"1500.00", "RUB", "1 500,00 ₽"},
		{"1500", "USD", "$1,500.00"},
		{"1500.5", "EUR", "1 500,50 €"},
		{"0", "RUB", "0,00 ₽"},
		{"1234567.89", "USD", "$1,234,567.89"},
		{"20.00", "USD", "$20.00"},
		{"999.999", "RUB", "999,99 ₽"}, // обрезаем до двух знаков без округления
	}
	for _, c := range cases {
		if got := FormatMoney(c.amount, c.currency); got != c.want {
			t.Errorf("FormatMoney(%q,%q) = %q, want %q", c.amount, c.currency, got, c.want)
		}
	}
}

func TestReminderTextEscapesAndTruncates(t *testing.T) {
	today := clock.Date{Y: 2026, M: 10, D: 24}
	o := model.ReminderOccurrence{
		ID: "x", Title: "<b>&evil</b>" + strings.Repeat("щ", 300),
		Amount: "100.00", Currency: "RUB", DueDate: today.AddDays(3),
		AssigneeName: "Иван <Петров>",
	}
	a := "u1"
	o.AssigneeID = &a
	txt := reminderText(o, today, model.NotificationKindDueSoon)

	if strings.Contains(txt, "<b>&evil") {
		t.Errorf("спецсимволы заголовка не экранированы: %q", txt)
	}
	if !strings.Contains(txt, "&lt;b&gt;") {
		t.Errorf("ожидалось экранирование < и >: %q", txt)
	}
	if !strings.Contains(txt, "Иван &lt;Петров&gt;") {
		t.Errorf("имя ответственного не экранировано: %q", txt)
	}
	if !strings.Contains(txt, "через 3 дня") {
		t.Errorf("нет относительной даты «через 3 дня»: %q", txt)
	}
}

func TestReminderTextNoAssignee(t *testing.T) {
	today := clock.Date{Y: 2026, M: 10, D: 24}
	o := model.ReminderOccurrence{Title: "T", Amount: "1.00", Currency: "RUB", DueDate: today.AddDays(1)}
	txt := reminderText(o, today, model.NotificationKindDueSoon)
	if !strings.Contains(txt, "не назначен") {
		t.Errorf("ожидалось «не назначен»: %q", txt)
	}
	if !strings.Contains(txt, "завтра") {
		t.Errorf("ожидалось «завтра»: %q", txt)
	}
}

func TestRelativeDay(t *testing.T) {
	today := clock.Date{Y: 2026, M: 10, D: 24}
	cases := map[int]string{
		0:  "сегодня",
		1:  "завтра",
		-1: "вчера",
		3:  "через 3 дня",
		5:  "через 5 дней",
		21: "через 21 день",
		-2: "просрочено на 2 дня",
	}
	for diff, want := range cases {
		if got := relativeDay(today, today.AddDays(diff)); got != want {
			t.Errorf("relativeDay diff=%d = %q, want %q", diff, got, want)
		}
	}
}

func TestUUIDHexRoundTrip(t *testing.T) {
	id := "11111111-2222-3333-4444-555555555555"
	hex := uuidToHex(id)
	if len(hex) != 32 {
		t.Fatalf("hex длиной %d, want 32", len(hex))
	}
	if back := hexToUUID(hex); back != id {
		t.Errorf("round trip: %q != %q", back, id)
	}
	if hexToUUID("short") != "" {
		t.Error("ожидалась пустая строка на некорректном hex")
	}
	if hexToUUID(strings.Repeat("z", 32)) != "" {
		t.Error("ожидалась пустая строка на не-hex символах")
	}
}

// Кнопка «Открыть» ведёт на Direct Link Mini App в домене t.me (а не plain-https
// на /tg, который startapp не понимает) — решения №6, №8, finding.
func TestReminderKeyboard_MiniAppDirectLink(t *testing.T) {
	id := "11111111-2222-3333-4444-555555555555"
	hex := uuidToHex(id)

	// Без username кнопки «Открыть» нет (ссылку не собрать).
	mk := reminderKeyboard(id, Config{})
	if n := len(mk.InlineKeyboard[0]); n != 1 {
		t.Fatalf("без username ожидалась только кнопка «Оплачено», got %d кнопок", n)
	}

	// Только username => основное Mini App бота.
	mk = reminderKeyboard(id, Config{BotUsername: "ik_dev_bot"})
	open := mk.InlineKeyboard[0][1]
	if want := "https://t.me/ik_dev_bot?startapp=occ_" + hex; open.URL != want {
		t.Errorf("URL = %q, want %q", open.URL, want)
	}
	if open.WebApp != nil {
		t.Error("в группе web_app-кнопка запрещена; ожидалась обычная URL-кнопка")
	}

	// username + short_name => конкретное зарегистрированное Mini App.
	mk = reminderKeyboard(id, Config{BotUsername: "ik_dev_bot", MiniAppShortName: "keeper"})
	if want := "https://t.me/ik_dev_bot/keeper?startapp=occ_" + hex; mk.InlineKeyboard[0][1].URL != want {
		t.Errorf("URL = %q, want %q", mk.InlineKeyboard[0][1].URL, want)
	}
}

func TestFormatDate(t *testing.T) {
	if got := formatDate(clock.Date{Y: 2026, M: 1, D: 5}); got != "05.01.2026" {
		t.Errorf("formatDate = %q, want 05.01.2026", got)
	}
}
