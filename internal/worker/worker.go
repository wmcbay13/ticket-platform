package worker

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/wmcbay13/ticket-platform/internal/store"
	"github.com/wmcbay13/ticket-platform/internal/telemetry"
)

type Repository interface {
	Claim(context.Context, time.Duration) (store.Job, error)
	Retry(context.Context, store.Job, time.Duration, string) error
	Complete(context.Context, store.Job, bool) error
	Backlog(context.Context) (float64, float64, error)
}
type Worker struct {
	Store              Repository
	Metrics            *telemetry.Metrics
	Delay, Lease, Poll time.Duration
	MaxAttempts        int
}

func (w Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.Poll)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			count, age, err := w.Store.Backlog(ctx)
			if err == nil {
				w.Metrics.Backlog.Set(count)
				w.Metrics.Oldest.Set(age)
			}
			j, err := w.Store.Claim(ctx, w.Lease)
			if errors.Is(err, store.ErrNotFound) {
				continue
			}
			if err != nil {
				if ctx.Err() == nil {
					slog.Error("claim failed", "error", err)
				}
				continue
			}
			if err = w.Process(ctx, j); err != nil && ctx.Err() == nil {
				slog.Error("payment processing interrupted", "order_id", j.OrderID, "error", err)
			}
		}
	}
}
func (w Worker) Process(ctx context.Context, j store.Job) error {
	slog.Info("payment claimed", "order_id", j.OrderID, "attempt", j.Attempts)
	timer := time.NewTimer(w.Delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
	}
	if j.Scenario == "retry" && j.Attempts == 1 && j.Attempts < w.MaxAttempts {
		if err := w.Store.Retry(ctx, j, 2*time.Second, "simulated transient failure"); err != nil {
			return err
		}
		w.Metrics.Retries.Inc()
		return nil
	}
	confirmed := j.Scenario != "decline" && j.Attempts <= w.MaxAttempts
	if err := w.Store.Complete(ctx, j, confirmed); err != nil {
		return err
	}
	outcome := "failed"
	if confirmed {
		outcome = "confirmed"
	}
	w.Metrics.Payments.WithLabelValues(outcome).Inc()
	slog.Info("payment completed", "order_id", j.OrderID, "outcome", outcome)
	return nil
}
