package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/AVGsync/invoice-keeper/backend/internal/clock"
	"github.com/AVGsync/invoice-keeper/backend/internal/model"
)

// maxScheduleIterations ограничивает генерацию дат, чтобы кривой шаблон не ушёл
// в бесконечный цикл.
const maxScheduleIterations = 1000

// Горизонт генерации периодов: today + horizonDays. Константа, без нового env.
const horizonDays = 31

// ScheduleRecurrentRepository — чтение активных шаблонов для планировщика.
type ScheduleRecurrentRepository interface {
	ListActiveForSchedule(ctx context.Context) ([]model.RecurrentPayment, error)
	GetRecurrentPaymentByID(id string, ctx context.Context) (model.RecurrentPayment, error)
}

// SchedulePaymentRepository — запись сгенерированных периодов и пересчёт overdue.
type SchedulePaymentRepository interface {
	InsertGenerated(rows []model.Payment, ctx context.Context) (int, error)
	SweepOverdue(today string, ctx context.Context) (int64, error)
}

// PaymentScheduler генерирует периоды из шаблонов и переводит просроченные в
// overdue. Реализует reminder.OccurrenceEnsurer.
type PaymentScheduler struct {
	recRepo ScheduleRecurrentRepository
	payRepo SchedulePaymentRepository
	clk     clock.Clock
}

func NewPaymentScheduler(
	recRepo ScheduleRecurrentRepository,
	payRepo SchedulePaymentRepository,
	clk clock.Clock,
) *PaymentScheduler {
	return &PaymentScheduler{recRepo: recRepo, payRepo: payRepo, clk: clk}
}

// Horizon возвращает верхнюю границу генерации: today + 31 (или больше, если
// окно напоминаний шире).
func (s *PaymentScheduler) Horizon(leadDays int) clock.Date {
	days := horizonDays
	if leadDays > days {
		days = leadDays
	}
	return s.clk.Today().AddDays(days)
}

// EnsureOccurrencesUntil гарантирует наличие периодов до until для всех активных
// шаблонов (satisfies reminder.OccurrenceEnsurer).
func (s *PaymentScheduler) EnsureOccurrencesUntil(ctx context.Context, until clock.Date) error {
	payments, err := s.recRepo.ListActiveForSchedule(ctx)
	if err != nil {
		return fmt.Errorf("scheduler: list active: %w", err)
	}
	for _, rp := range payments {
		if err := s.ensure(ctx, rp, until); err != nil {
			slog.Error("scheduler: ensure occurrences", "recurrent_payment_id", rp.ID, "error", err)
		}
	}
	return nil
}

// EnsureFor генерирует периоды одного шаблона (после создания/обновления).
func (s *PaymentScheduler) EnsureFor(ctx context.Context, rpID string) error {
	rp, err := s.recRepo.GetRecurrentPaymentByID(rpID, ctx)
	if err != nil {
		return fmt.Errorf("scheduler: get recurrent payment: %w", err)
	}
	if rp.StoppedAt != nil {
		return nil
	}
	return s.ensure(ctx, rp, s.Horizon(0))
}

func (s *PaymentScheduler) ensure(ctx context.Context, rp model.RecurrentPayment, until clock.Date) error {
	if rp.AssigneeID == nil {
		slog.Warn("scheduler: recurrent payment has no assignee, skip", "recurrent_payment_id", rp.ID)
		return nil
	}

	start, err := clock.ParseDate(rp.StartDate)
	if err != nil {
		return fmt.Errorf("scheduler: parse start_date: %w", err)
	}

	dates := DueDates(rp, start.AddDays(-1), until)
	if len(dates) == 0 {
		return nil
	}

	today := s.clk.Today()
	rows := make([]model.Payment, 0, len(dates))
	for _, d := range dates {
		status := "pending"
		if d.Before(today) {
			status = "overdue"
		}
		rpID := rp.ID
		rows = append(rows, model.Payment{
			UserID:             *rp.AssigneeID,
			RecurrentPaymentID: &rpID,
			Amount:             rp.Amount,
			Currency:           rp.Currency,
			Description:        rp.Description,
			DueDate:            d.String(),
			Status:             status,
		})
	}

	if _, err := s.payRepo.InsertGenerated(rows, ctx); err != nil {
		return fmt.Errorf("scheduler: insert generated: %w", err)
	}
	return nil
}

// Sweep переводит просроченные периоды в overdue на «сегодня».
func (s *PaymentScheduler) Sweep(ctx context.Context) error {
	if _, err := s.payRepo.SweepOverdue(s.clk.Today().String(), ctx); err != nil {
		return fmt.Errorf("scheduler: sweep overdue: %w", err)
	}
	return nil
}

// DueDates — чистая функция графика (D9). Возвращает даты d строго после after и
// не позже until, начиная с якоря start_date.
//   - monthly: период k в месяце start+k*interval на min(day_offset, последний день месяца);
//   - weekly:  start_date + 7*interval*k.
func DueDates(rp model.RecurrentPayment, after, until clock.Date) []clock.Date {
	start, err := clock.ParseDate(rp.StartDate)
	if err != nil {
		return nil
	}
	interval := rp.Interval
	if interval <= 0 {
		interval = 1
	}

	var out []clock.Date
	switch rp.Frequency {
	case "weekly":
		for k := 0; k < maxScheduleIterations; k++ {
			d := start.AddDays(7 * interval * k)
			if d.After(until) {
				break
			}
			if d.After(after) {
				out = append(out, d)
			}
		}
	case "monthly":
		day := start.D
		for k := 0; k < maxScheduleIterations; k++ {
			d := monthlyDate(start.Y, int(start.M), day, k*interval)
			if d.After(until) {
				break
			}
			if d.After(after) {
				out = append(out, d)
			}
		}
	}
	return out
}

// monthlyDate возвращает дату в месяце (baseYear, baseMonth + addMonths) на день
// min(day, последний день месяца). Несуществующий день → последний день месяца.
func monthlyDate(baseYear, baseMonth, day, addMonths int) clock.Date {
	// Нормализуем месяц.
	m := baseMonth - 1 + addMonths
	y := baseYear + m/12
	m = m % 12
	if m < 0 {
		m += 12
		y--
	}
	month := time.Month(m + 1)
	last := lastDayOfMonth(y, month)
	if day > last {
		day = last
	}
	return clock.Date{Y: y, M: month, D: day}
}

// lastDayOfMonth — число дней в месяце с учётом високосных лет.
func lastDayOfMonth(year int, month time.Month) int {
	// День 0 следующего месяца == последний день текущего.
	t := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC)
	return t.Day()
}
