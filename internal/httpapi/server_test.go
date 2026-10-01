package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wmcbay13/ticket-platform/internal/store"
	"github.com/wmcbay13/ticket-platform/internal/telemetry"
)

type fakeRepo struct {
	err   error
	calls int
}

func (f *fakeRepo) Events(context.Context) ([]store.Event, error)      { return []store.Event{}, f.err }
func (f *fakeRepo) Event(context.Context, string) (store.Event, error) { return store.Event{}, f.err }
func (f *fakeRepo) Order(context.Context, string) (store.Order, error) { return store.Order{}, f.err }
func (f *fakeRepo) Reserve(_ context.Context, r store.Reservation, _ string) (store.Order, bool, error) {
	f.calls++
	return store.Order{ID: "00000000-0000-4000-8000-000000000000", Status: "pending"}, true, f.err
}
func handler(f *fakeRepo) http.Handler {
	return Handler(Config{Mode: "booking", Version: "test", Repository: f, Ping: func(context.Context) error { return nil }, Metrics: telemetry.New("booking", "test")})
}
func TestOrderRequestValidation(t *testing.T) {
	valid := `{"event_id":"test","quantity":1,"payment_scenario":"success"}`
	for _, tt := range []struct {
		body, key, content string
		code               int
	}{{valid, "", "application/json", 400}, {valid, "x", "text/plain", 415}, {`{`, "x", "application/json", 400}, {valid + ` {}`, "x", "application/json", 400}, {`{"event_id":"test","quantity":0,"payment_scenario":"success"}`, "x", "application/json", 400}, {`{"event_id":"test","quantity":1,"payment_scenario":"success","extra":1}`, "x", "application/json", 400}} {
		f := &fakeRepo{}
		r := httptest.NewRequest("POST", "/api/orders", strings.NewReader(tt.body))
		r.Header.Set("Idempotency-Key", tt.key)
		r.Header.Set("Content-Type", tt.content)
		w := httptest.NewRecorder()
		handler(f).ServeHTTP(w, r)
		if w.Code != tt.code || f.calls != 0 {
			t.Fatalf("code=%d calls=%d body=%s", w.Code, f.calls, w.Body.String())
		}
	}
}
func TestOrderResponses(t *testing.T) {
	for _, tt := range []struct {
		err  error
		code int
	}{{nil, 201}, {store.ErrSoldOut, 409}, {store.ErrConflict, 409}, {store.ErrNotFound, 404}, {errors.New("secret database details"), 503}} {
		f := &fakeRepo{err: tt.err}
		r := httptest.NewRequest("POST", "/api/orders", strings.NewReader(`{"event_id":"test","quantity":1,"payment_scenario":"success"}`))
		r.Header.Set("Idempotency-Key", "abc")
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler(f).ServeHTTP(w, r)
		if w.Code != tt.code {
			t.Fatalf("got %d wanted %d", w.Code, tt.code)
		}
		if strings.Contains(w.Body.String(), "secret database") {
			t.Fatal("leaked database details")
		}
		if w.Header().Get("X-Request-ID") == "" {
			t.Fatal("missing request id")
		}
	}
}
func TestReadinessDoesNotAffectLiveness(t *testing.T) {
	h := Handler(Config{Mode: "worker", Repository: &fakeRepo{}, Ping: func(context.Context) error { return errors.New("unavailable") }, Metrics: telemetry.New("worker", "test")})
	for path, code := range map[string]int{"/healthz": 200, "/readyz": 503} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != code {
			t.Fatalf("%s got %d", path, w.Code)
		}
	}
}
