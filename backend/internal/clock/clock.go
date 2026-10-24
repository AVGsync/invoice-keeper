// Package clock задаёт единый источник «сегодня» и календарную дату без зоны.
package clock

import (
	"fmt"
	"time"
)

// Date — календарная дата без времени и зоны.
type Date struct {
	Y int
	M time.Month
	D int
}

// String форматирует дату как "2006-01-02" — ровно так она уходит в SQL.
func (d Date) String() string {
	return fmt.Sprintf("%04d-%02d-%02d", d.Y, int(d.M), d.D)
}

// ParseDate разбирает строку формата "2006-01-02".
func ParseDate(s string) (Date, error) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return Date{}, fmt.Errorf("clock: parse date %q: %w", s, err)
	}
	return Date{Y: t.Year(), M: t.Month(), D: t.Day()}, nil
}

// DateOf выделяет календарную дату из момента времени в заданной зоне.
func DateOf(t time.Time, loc *time.Location) Date {
	t = t.In(loc)
	return Date{Y: t.Year(), M: t.Month(), D: t.Day()}
}

// AddDays возвращает дату, сдвинутую на n дней (n может быть отрицательным).
func (d Date) AddDays(n int) Date {
	t := time.Date(d.Y, d.M, d.D+n, 0, 0, 0, 0, time.UTC)
	return Date{Y: t.Year(), M: t.Month(), D: t.Day()}
}

// Compare возвращает -1, 0 или +1 (как strings.Compare), сравнивая даты хронологически.
func (d Date) Compare(o Date) int {
	switch {
	case d.Y != o.Y:
		return sign(d.Y - o.Y)
	case d.M != o.M:
		return sign(int(d.M) - int(o.M))
	case d.D != o.D:
		return sign(d.D - o.D)
	default:
		return 0
	}
}

// Before сообщает, что дата d строго раньше o.
func (d Date) Before(o Date) bool { return d.Compare(o) < 0 }

// After сообщает, что дата d строго позже o.
func (d Date) After(o Date) bool { return d.Compare(o) > 0 }

// Equal сообщает, что даты совпадают.
func (d Date) Equal(o Date) bool { return d.Compare(o) == 0 }

func sign(n int) int {
	switch {
	case n < 0:
		return -1
	case n > 0:
		return 1
	default:
		return 0
	}
}

// Clock — источник текущего времени и «сегодня».
type Clock interface {
	Now() time.Time
	Today() Date
	Location() *time.Location
}

// Real — реальные часы в конкретной зоне.
type Real struct {
	loc *time.Location
}

// New создаёт реальные часы в зоне loc.
func New(loc *time.Location) Clock {
	if loc == nil {
		loc = time.UTC
	}
	return Real{loc: loc}
}

func (r Real) Now() time.Time           { return time.Now().In(r.loc) }
func (r Real) Today() Date              { return DateOf(time.Now(), r.loc) }
func (r Real) Location() *time.Location { return r.loc }

// Fixed — детерминированные часы для тестов: всегда возвращают заданный момент.
type Fixed struct {
	T   time.Time
	Loc *time.Location
}

// NewFixed создаёт зафиксированные на момент t часы в зоне loc.
func NewFixed(t time.Time, loc *time.Location) Fixed {
	if loc == nil {
		loc = t.Location()
	}
	return Fixed{T: t, Loc: loc}
}

func (f Fixed) Now() time.Time {
	if f.Loc == nil {
		return f.T
	}
	return f.T.In(f.Loc)
}

func (f Fixed) Today() Date {
	loc := f.Loc
	if loc == nil {
		loc = f.T.Location()
	}
	return DateOf(f.T, loc)
}

func (f Fixed) Location() *time.Location {
	if f.Loc == nil {
		return f.T.Location()
	}
	return f.Loc
}
