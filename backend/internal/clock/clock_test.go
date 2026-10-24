package clock

import (
	"testing"
	"time"
)

func TestDateString(t *testing.T) {
	cases := []struct {
		d    Date
		want string
	}{
		{Date{2026, time.October, 24}, "2026-10-24"},
		{Date{2026, time.January, 1}, "2026-01-01"},
		{Date{2026, time.December, 31}, "2026-12-31"},
		{Date{999, time.March, 5}, "0999-03-05"},
	}
	for _, c := range cases {
		if got := c.d.String(); got != c.want {
			t.Errorf("String(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}

func TestParseDate(t *testing.T) {
	d, err := ParseDate("2026-10-24")
	if err != nil {
		t.Fatalf("ParseDate: unexpected error: %v", err)
	}
	want := Date{2026, time.October, 24}
	if !d.Equal(want) {
		t.Errorf("ParseDate = %v, want %v", d, want)
	}
	if _, err := ParseDate("24.10.2026"); err == nil {
		t.Error("ParseDate: expected error on bad format")
	}
	if got := d.String(); got != "2026-10-24" {
		t.Errorf("round-trip = %q, want %q", got, "2026-10-24")
	}
}

func TestAddDays(t *testing.T) {
	cases := []struct {
		in   Date
		n    int
		want Date
	}{
		{Date{2026, time.October, 24}, 3, Date{2026, time.October, 27}},
		{Date{2026, time.October, 30}, 3, Date{2026, time.November, 2}},   // через месяц
		{Date{2026, time.December, 31}, 1, Date{2027, time.January, 1}},   // через год
		{Date{2027, time.January, 1}, -1, Date{2026, time.December, 31}},  // назад через год
		{Date{2024, time.February, 28}, 1, Date{2024, time.February, 29}}, // високосный
		{Date{2026, time.February, 28}, 1, Date{2026, time.March, 1}},     // невисокосный
	}
	for _, c := range cases {
		if got := c.in.AddDays(c.n); !got.Equal(c.want) {
			t.Errorf("%v.AddDays(%d) = %v, want %v", c.in, c.n, got, c.want)
		}
	}
}

func TestCompare(t *testing.T) {
	a := Date{2026, time.October, 24}
	b := Date{2026, time.October, 25}
	if !a.Before(b) || !b.After(a) || a.Equal(b) {
		t.Errorf("ordering a<b failed: a=%v b=%v", a, b)
	}
	if a.Compare(a) != 0 {
		t.Errorf("Compare self != 0")
	}
}

// TestTodayTimezoneBoundary проверяет, что «сегодня» зависит от зоны: в момент, когда в UTC ещё 23:30 24-го, в Москве (UTC+3) уже 02:30 25-го.
func TestTodayTimezoneBoundary(t *testing.T) {
	msk, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		t.Fatalf("load Europe/Moscow: %v", err)
	}
	instant := time.Date(2026, time.October, 24, 23, 30, 0, 0, time.UTC)

	utcToday := NewFixed(instant, time.UTC).Today()
	if want := (Date{2026, time.October, 24}); !utcToday.Equal(want) {
		t.Errorf("UTC Today = %v, want %v", utcToday, want)
	}

	mskToday := NewFixed(instant, msk).Today()
	if want := (Date{2026, time.October, 25}); !mskToday.Equal(want) {
		t.Errorf("MSK Today = %v, want %v", mskToday, want)
	}
}

func TestRealClockLocation(t *testing.T) {
	c := New(nil)
	if c.Location() != time.UTC {
		t.Errorf("New(nil).Location() = %v, want UTC", c.Location())
	}
	msk, _ := time.LoadLocation("Europe/Moscow")
	c = New(msk)
	if c.Location() != msk {
		t.Errorf("New(msk).Location() mismatch")
	}
	if c.Now().Location() != msk {
		t.Errorf("Now().Location() mismatch")
	}
}
