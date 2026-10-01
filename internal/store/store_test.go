package store

import (
	"context"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestReservationValidation(t *testing.T) {
	for _, r := range []Reservation{{}, {EventID: "x", Quantity: 0, PaymentScenario: "success"}, {EventID: "x", Quantity: 11, PaymentScenario: "success"}, {EventID: "x", Quantity: 1, PaymentScenario: "real"}} {
		if r.Validate() == nil {
			t.Fatalf("accepted invalid reservation: %+v", r)
		}
	}
	if err := (Reservation{EventID: "x", Quantity: 1, PaymentScenario: "success"}).Validate(); err != nil {
		t.Fatal(err)
	}
}
func testDB(t *testing.T) *Store {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is required for PostgreSQL integration tests")
	}
	s, err := Open(context.Background(), dsn, 10)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Pool.Close)
	if err = s.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Pool.Exec(context.Background(), "TRUNCATE payment_jobs,orders,events CASCADE"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Pool.Exec(context.Background(), "INSERT INTO events VALUES('test','Test','Here',now(),'Test event',1000,20,20)"); err != nil {
		t.Fatal(err)
	}
	return s
}
func TestConcurrentReservationsNeverOversell(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	var accepted atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 80; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, err := s.Reserve(ctx, Reservation{"test", 1, "success"}, UUID())
			if err == nil {
				accepted.Add(1)
			} else if !errors.Is(err, ErrSoldOut) {
				t.Errorf("reserve: %v", err)
			}
		}()
	}
	wg.Wait()
	if accepted.Load() != 20 {
		t.Fatalf("accepted %d, wanted 20", accepted.Load())
	}
	event, err := s.Event(ctx, "test")
	if err != nil || event.Available != 0 {
		t.Fatalf("inventory: %+v %v", event, err)
	}
	var orders, jobs int
	if err = s.Pool.QueryRow(ctx, "SELECT (SELECT count(*) FROM orders),(SELECT count(*) FROM payment_jobs)").Scan(&orders, &jobs); err != nil {
		t.Fatal(err)
	}
	if orders != 20 || jobs != 20 {
		t.Fatalf("orders/jobs=%d/%d", orders, jobs)
	}
}
func TestConcurrentIdempotency(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	key := UUID()
	r := Reservation{"test", 2, "success"}
	var wg sync.WaitGroup
	ids := make(chan string, 40)
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			o, _, err := s.Reserve(ctx, r, key)
			if err != nil {
				t.Error(err)
				return
			}
			ids <- o.ID
		}()
	}
	wg.Wait()
	close(ids)
	id := ""
	for got := range ids {
		if id != "" && got != id {
			t.Fatalf("duplicate order ids: %s %s", id, got)
		}
		id = got
	}
	event, _ := s.Event(ctx, "test")
	if event.Available != 18 {
		t.Fatalf("available=%d", event.Available)
	}
	r.Quantity = 3
	if _, _, err := s.Reserve(ctx, r, key); !errors.Is(err, ErrConflict) {
		t.Fatalf("conflicting reuse: %v", err)
	}
}
func TestLeaseExpiryAndDuplicateCompletion(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	o, _, err := s.Reserve(ctx, Reservation{"test", 3, "decline"}, UUID())
	if err != nil {
		t.Fatal(err)
	}
	first, err := s.Claim(ctx, 30*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Claim(ctx, time.Second); !errors.Is(err, ErrNotFound) {
		t.Fatalf("claimed live lease: %v", err)
	}
	time.Sleep(60 * time.Millisecond)
	second, err := s.Claim(ctx, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if second.Attempts != 2 || first.Token == second.Token {
		t.Fatalf("bad reclaim: %+v", second)
	}
	if err = s.Complete(ctx, first, false); !errors.Is(err, ErrLeaseLost) {
		t.Fatalf("stale completion: %v", err)
	}
	if err = s.Complete(ctx, second, false); err != nil {
		t.Fatal(err)
	}
	if err = s.Complete(ctx, second, false); !errors.Is(err, ErrLeaseLost) {
		t.Fatalf("duplicate completion: %v", err)
	}
	event, _ := s.Event(ctx, "test")
	if event.Available != 20 {
		t.Fatalf("released inventory=%d", event.Available)
	}
	got, _ := s.Order(ctx, o.ID)
	if got.Status != "failed" {
		t.Fatalf("status=%s", got.Status)
	}
	if _, err = s.Claim(ctx, time.Second); !errors.Is(err, ErrNotFound) {
		t.Fatalf("reclaimed completed job: %v", err)
	}
}
func TestRetryAndConfirmedInventory(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	o, _, err := s.Reserve(ctx, Reservation{"test", 4, "retry"}, UUID())
	if err != nil {
		t.Fatal(err)
	}
	j, err := s.Claim(ctx, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Retry(ctx, j, 0, "transient"); err != nil {
		t.Fatal(err)
	}
	if err = s.Complete(ctx, j, true); !errors.Is(err, ErrLeaseLost) {
		t.Fatalf("retried lease remained valid: %v", err)
	}
	j, err = s.Claim(ctx, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Complete(ctx, j, true); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Order(ctx, o.ID)
	event, _ := s.Event(ctx, "test")
	if got.Status != "confirmed" || event.Available != 16 {
		t.Fatalf("order=%+v event=%+v", got, event)
	}
}
func TestUnknownEventAndAtomicFailure(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	if _, _, err := s.Reserve(ctx, Reservation{"missing", 1, "success"}, UUID()); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := s.Pool.Exec(ctx, "ALTER TABLE payment_jobs ADD CONSTRAINT reject_jobs CHECK (false) NOT VALID"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = s.Pool.Exec(ctx, "ALTER TABLE payment_jobs DROP CONSTRAINT reject_jobs") })
	if _, _, err := s.Reserve(ctx, Reservation{"test", 2, "success"}, UUID()); err == nil {
		t.Fatal("expected job insertion failure")
	}
	event, _ := s.Event(ctx, "test")
	if event.Available != 20 {
		t.Fatal("inventory changed after transaction rollback")
	}
	var count int
	_ = s.Pool.QueryRow(ctx, "SELECT count(*) FROM orders").Scan(&count)
	if count != 0 {
		t.Fatal("order survived rollback")
	}
}
