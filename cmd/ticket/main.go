package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/wmcbay13/ticket-platform/internal/httpapi"
	"github.com/wmcbay13/ticket-platform/internal/store"
	"github.com/wmcbay13/ticket-platform/internal/telemetry"
	"github.com/wmcbay13/ticket-platform/internal/worker"
)

var version = "development"

func main() {
	mode := flag.String("mode", "catalog", "catalog, booking, worker, or migrate")
	flag.Parse()
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if *mode != "catalog" && *mode != "booking" && *mode != "worker" && *mode != "migrate" {
		slog.Error("invalid mode")
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		slog.Error("DATABASE_URL is required")
		os.Exit(1)
	}
	maxConns := int32(5)
	if value := os.Getenv("DB_MAX_CONNS"); value != "" {
		n, err := strconv.Atoi(value)
		if err != nil || n < 1 || n > 20 {
			slog.Error("DB_MAX_CONNS must be 1..20")
			os.Exit(1)
		}
		maxConns = int32(n)
	}
	db, err := store.Open(ctx, dsn, maxConns)
	if err != nil {
		slog.Error("database configuration failed", "error", err)
		os.Exit(1)
	}
	defer db.Pool.Close()
	if *mode == "migrate" {
		mctx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
		if err = db.Migrate(mctx); err != nil {
			slog.Error("migration failed", "error", err)
			os.Exit(1)
		}
		slog.Info("migrations complete")
		return
	}
	v := os.Getenv("APP_VERSION")
	if v == "" {
		v = version
	}
	m := telemetry.New(*mode, v)
	handler := httpapi.Handler(httpapi.Config{Mode: *mode, Version: v, Repository: db, Ping: db.Pool.Ping, Metrics: m, DemoFail: os.Getenv("DEMO_FAIL") == "true"})
	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	server := &http.Server{Addr: addr, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	workerDone := make(chan struct{})
	if *mode == "worker" {
		go func() {
			defer close(workerDone)
			worker.Worker{Store: db, Metrics: m, Delay: 3 * time.Second, Lease: 30 * time.Second, Poll: 250 * time.Millisecond, MaxAttempts: 5}.Run(ctx)
		}()
	} else {
		close(workerDone)
	}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	slog.Info("service starting", "mode", *mode, "version", v, "address", addr)
	if err = server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("http server failed", "error", err)
		stop()
	}
	<-workerDone
}
