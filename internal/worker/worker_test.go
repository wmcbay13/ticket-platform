package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wmcbay13/ticket-platform/internal/store"
	"github.com/wmcbay13/ticket-platform/internal/telemetry"
)

type fakeStore struct {
	completed, retried bool
	confirmed          bool
	err                error
}

func (f *fakeStore) Claim(context.Context, time.Duration) (store.Job, error) {
	return store.Job{}, store.ErrNotFound
}
func (f *fakeStore) Retry(context.Context, store.Job, time.Duration, string) error {
	f.retried = true
	return f.err
}
func (f *fakeStore) Complete(_ context.Context, _ store.Job, confirmed bool) error {
	f.completed = true
	f.confirmed = confirmed
	return f.err
}
func (f *fakeStore) Backlog(context.Context) (float64, float64, error) { return 0, 0, nil }
func TestPaymentScenarios(t *testing.T) {
	for _, tt := range []struct {
		scenario         string
		attempts         int
		retry, confirmed bool
	}{{"success", 1, false, true}, {"decline", 1, false, false}, {"retry", 1, true, false}, {"retry", 2, false, true}, {"success", 6, false, false}} {
		f := &fakeStore{}
		w := Worker{Store: f, Metrics: telemetry.New("worker", "test"), Delay: time.Millisecond, MaxAttempts: 5}
		if err := w.Process(context.Background(), store.Job{Scenario: tt.scenario, Attempts: tt.attempts}); err != nil {
			t.Fatal(err)
		}
		if f.retried != tt.retry || f.confirmed != tt.confirmed || f.completed == tt.retry {
			t.Fatalf("scenario=%+v result=%+v", tt, f)
		}
	}
}
func TestInterruptedPaymentLeavesLeaseForRecovery(t *testing.T) {
	f := &fakeStore{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	w := Worker{Store: f, Metrics: telemetry.New("worker", "test"), Delay: time.Second, MaxAttempts: 5}
	if err := w.Process(ctx, store.Job{Scenario: "success", Attempts: 1}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if f.completed || f.retried {
		t.Fatal("interrupted job was mutated")
	}
}
