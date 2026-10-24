package apiserver

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// paymentsSchedulerTick — период фоновой генерации периодов и пересчёта overdue.
const paymentsSchedulerTick = 5 * time.Minute

// startPaymentsScheduler запускает фоновый генератор периодов: сразу при старте
// и далее каждые 5 минут. Работает независимо от Telegram-бота: периоды нужны и
// для веб-интерфейса, и для сводок, а не только для напоминаний.
func (s *APIServer) startPaymentsScheduler(ctx context.Context, wg *sync.WaitGroup) {
	if s.scheduler == nil {
		return
	}

	run := func() {
		until := s.scheduler.Horizon(s.config.ReminderLeadDays)
		if err := s.scheduler.EnsureOccurrencesUntil(ctx, until); err != nil {
			slog.Error("payments scheduler: ensure occurrences", "error", err)
		}
		if err := s.scheduler.Sweep(ctx); err != nil {
			slog.Error("payments scheduler: sweep overdue", "error", err)
		}
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		slog.Info("payments scheduler: started", "tick", paymentsSchedulerTick)
		run()
		t := time.NewTicker(paymentsSchedulerTick)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				slog.Info("payments scheduler: stopped")
				return
			case <-t.C:
				run()
			}
		}
	}()
}
