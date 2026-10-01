package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/wmcbay13/ticket-platform/internal/store"
	"github.com/wmcbay13/ticket-platform/internal/telemetry"
)

type Repository interface {
	Events(context.Context) ([]store.Event, error)
	Event(context.Context, string) (store.Event, error)
	Order(context.Context, string) (store.Order, error)
	Reserve(context.Context, store.Reservation, string) (store.Order, bool, error)
}
type Config struct {
	Mode, Version string
	Repository    Repository
	Ping          func(context.Context) error
	Metrics       *telemetry.Metrics
	DemoFail      bool
}

func Handler(c Config) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { write(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		if c.Ping(ctx) != nil {
			write(w, 503, map[string]string{"error": "database unavailable"})
			return
		}
		write(w, 200, map[string]string{"status": "ready"})
	})
	mux.Handle("GET /metrics", promhttp.HandlerFor(c.Metrics.Registry, promhttp.HandlerOpts{}))
	if c.Mode == "catalog" {
		mux.HandleFunc("GET /api/events", func(w http.ResponseWriter, r *http.Request) {
			events, err := c.Repository.Events(r.Context())
			if err != nil {
				fail(w, r, err)
				return
			}
			write(w, 200, events)
		})
		mux.HandleFunc("GET /api/events/{id}", func(w http.ResponseWriter, r *http.Request) {
			event, err := c.Repository.Event(r.Context(), r.PathValue("id"))
			if err != nil {
				fail(w, r, err)
				return
			}
			write(w, 200, event)
		})
	}
	if c.Mode == "booking" {
		mux.HandleFunc("GET /api/orders/{id}", func(w http.ResponseWriter, r *http.Request) {
			id := r.PathValue("id")
			if len(id) != 36 {
				write(w, 400, map[string]string{"error": "invalid order id"})
				return
			}
			order, err := c.Repository.Order(r.Context(), id)
			if err != nil {
				fail(w, r, err)
				return
			}
			write(w, 200, order)
		})
		mux.HandleFunc("POST /api/orders", func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get("Idempotency-Key")
			if strings.TrimSpace(key) == "" || len(key) > 128 {
				write(w, 400, map[string]string{"error": "Idempotency-Key is required (maximum 128 characters)"})
				return
			}
			if r.Header.Get("Content-Type") != "application/json" {
				write(w, 415, map[string]string{"error": "Content-Type must be application/json"})
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, 4096)
			var reservation store.Reservation
			dec := json.NewDecoder(r.Body)
			dec.DisallowUnknownFields()
			if err := dec.Decode(&reservation); err != nil {
				write(w, 400, map[string]string{"error": "invalid JSON body"})
				return
			}
			if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
				write(w, 400, map[string]string{"error": "body must contain one JSON object"})
				return
			}
			if err := reservation.Validate(); err != nil {
				write(w, 400, map[string]string{"error": err.Error()})
				return
			}
			order, created, err := c.Repository.Reserve(r.Context(), reservation, key)
			if err != nil {
				fail(w, r, err)
				return
			}
			code := 200
			if created {
				code = 201
				c.Metrics.Orders.WithLabelValues(reservation.PaymentScenario).Inc()
			}
			w.Header().Set("Location", "/api/orders/"+order.ID)
			write(w, code, order)
		})
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		id := store.UUID()
		w.Header().Set("X-Request-ID", id)
		w.Header().Set("X-App-Version", c.Version)
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		r = r.WithContext(ctx)
		rw := &responseWriter{ResponseWriter: w, status: 200}
		if c.DemoFail && strings.HasPrefix(r.URL.Path, "/api/") {
			write(rw, 503, map[string]string{"error": "intentional canary failure"})
		} else {
			mux.ServeHTTP(rw, r)
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			route := r.Pattern
			if route == "" {
				route = "unmatched"
			}
			c.Metrics.Requests.WithLabelValues(r.Method, route, strconv.Itoa(rw.status)).Inc()
			c.Metrics.Duration.WithLabelValues(r.Method, route).Observe(time.Since(start).Seconds())
			slog.Info("request", "request_id", id, "method", r.Method, "route", route, "status", rw.status, "duration_ms", time.Since(start).Milliseconds())
		}
	})
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (w *responseWriter) WriteHeader(code int) { w.status = code; w.ResponseWriter.WriteHeader(code) }
func write(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, r *http.Request, err error) {
	code := 503
	message := "service temporarily unavailable"
	switch {
	case errors.Is(err, store.ErrNotFound):
		code = 404
		message = "not found"
	case errors.Is(err, store.ErrSoldOut), errors.Is(err, store.ErrConflict):
		code = 409
		message = err.Error()
	default:
		slog.Error("database request failed", "error", err, "request_id", w.Header().Get("X-Request-ID"))
	}
	write(w, code, map[string]string{"error": message})
}
