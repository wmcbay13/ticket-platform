package store

import (
	"context"
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed schema.sql
var schema string

//go:embed seed.sql
var seed string

var (
	ErrNotFound  = errors.New("not found")
	ErrSoldOut   = errors.New("not enough tickets available")
	ErrConflict  = errors.New("idempotency key already used with different details")
	ErrLeaseLost = errors.New("payment lease no longer owned")
)

type Event struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Venue       string    `json:"venue"`
	StartsAt    time.Time `json:"starts_at"`
	Description string    `json:"description"`
	PriceCents  int       `json:"price_cents"`
	Capacity    int       `json:"capacity"`
	Available   int       `json:"available"`
}

type Order struct {
	ID              string    `json:"id"`
	EventID         string    `json:"event_id"`
	Quantity        int       `json:"quantity"`
	TotalCents      int       `json:"total_cents"`
	Status          string    `json:"status"`
	PaymentScenario string    `json:"payment_scenario"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type Reservation struct {
	EventID         string `json:"event_id"`
	Quantity        int    `json:"quantity"`
	PaymentScenario string `json:"payment_scenario"`
}

func (r Reservation) Validate() error {
	if r.EventID == "" || len(r.EventID) > 100 {
		return errors.New("event_id is required (maximum 100 characters)")
	}
	if r.Quantity < 1 || r.Quantity > 10 {
		return errors.New("quantity must be between 1 and 10")
	}
	if r.PaymentScenario != "success" && r.PaymentScenario != "decline" && r.PaymentScenario != "retry" {
		return errors.New("payment_scenario must be success, decline, or retry")
	}
	return nil
}

type Store struct{ Pool *pgxpool.Pool }

func Open(ctx context.Context, dsn string, maxConns int32) (*Store, error) {
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	config.MaxConns = maxConns
	config.MinConns = 0
	config.MaxConnLifetime = 30 * time.Minute
	config.ConnConfig.ConnectTimeout = 5 * time.Second
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}
	return &Store{Pool: pool}, nil
}

func (s *Store) Migrate(ctx context.Context) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(78001)"); err != nil {
		return err
	}
	// Version 1 is additive and can run safely before old and new pods coexist.
	if _, err = tx.Exec(ctx, schema); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, seed); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "INSERT INTO schema_migrations(version) VALUES (1) ON CONFLICT DO NOTHING"); err != nil {
		return err
	}
	for _, role := range []string{"catalog", "booking", "worker"} {
		password := os.Getenv(strings.ToUpper(role) + "_PASSWORD")
		if password == "" {
			continue
		} // integration tests use their isolated database owner
		// Passwords are generated hex strings; never interpolate arbitrary input into SQL.
		if len(password) < 32 {
			return errors.New("role passwords must contain at least 32 hex characters")
		}
		if _, err := hex.DecodeString(password); err != nil {
			return errors.New("role passwords must be hex")
		}
		var exists bool
		if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname=$1)", role).Scan(&exists); err != nil {
			return err
		}
		verb := "CREATE"
		if exists {
			verb = "ALTER"
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf("%s ROLE %s LOGIN PASSWORD '%s'", verb, pgx.Identifier{role}.Sanitize(), password)); err != nil {
			return err
		}
	}
	if os.Getenv("CATALOG_PASSWORD") != "" {
		if _, err = tx.Exec(ctx, `GRANT USAGE ON SCHEMA public TO catalog, booking, worker;
GRANT SELECT ON events TO catalog;
GRANT SELECT, INSERT, UPDATE ON events, orders, payment_jobs TO booking;
GRANT SELECT, UPDATE ON events, orders, payment_jobs TO worker;`); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

const eventColumns = "id,title,venue,starts_at,description,price_cents,capacity,available"
const orderColumns = "id::text,event_id,quantity,total_cents,status,payment_scenario,created_at,updated_at"

func scanEvent(row pgx.Row) (Event, error) {
	var e Event
	err := row.Scan(&e.ID, &e.Title, &e.Venue, &e.StartsAt, &e.Description, &e.PriceCents, &e.Capacity, &e.Available)
	if errors.Is(err, pgx.ErrNoRows) {
		err = ErrNotFound
	}
	return e, err
}
func scanOrder(row pgx.Row) (Order, error) {
	var o Order
	err := row.Scan(&o.ID, &o.EventID, &o.Quantity, &o.TotalCents, &o.Status, &o.PaymentScenario, &o.CreatedAt, &o.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		err = ErrNotFound
	}
	return o, err
}
func (s *Store) Events(ctx context.Context) ([]Event, error) {
	rows, err := s.Pool.Query(ctx, "SELECT "+eventColumns+" FROM events ORDER BY starts_at")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := []Event{}
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}
func (s *Store) Event(ctx context.Context, id string) (Event, error) {
	return scanEvent(s.Pool.QueryRow(ctx, "SELECT "+eventColumns+" FROM events WHERE id=$1", id))
}
func (s *Store) Order(ctx context.Context, id string) (Order, error) {
	return scanOrder(s.Pool.QueryRow(ctx, "SELECT "+orderColumns+" FROM orders WHERE id::text=$1", id))
}
func same(o Order, r Reservation) bool {
	return o.EventID == r.EventID && o.Quantity == r.Quantity && o.PaymentScenario == r.PaymentScenario
}

func (s *Store) Reserve(ctx context.Context, r Reservation, key string) (Order, bool, error) {
	if err := r.Validate(); err != nil {
		return Order{}, false, err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Order{}, false, err
	}
	defer tx.Rollback(ctx)
	// Serialize only matching idempotency keys, including concurrent first requests.
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", key); err != nil {
		return Order{}, false, err
	}
	o, err := scanOrder(tx.QueryRow(ctx, "SELECT "+orderColumns+" FROM orders WHERE idempotency_key=$1", key))
	if err == nil {
		if !same(o, r) {
			return Order{}, false, ErrConflict
		}
		return o, false, tx.Commit(ctx)
	}
	if !errors.Is(err, ErrNotFound) {
		return Order{}, false, err
	}
	var price int
	err = tx.QueryRow(ctx, "UPDATE events SET available=available-$2 WHERE id=$1 AND available >= $2 RETURNING price_cents", r.EventID, r.Quantity).Scan(&price)
	if errors.Is(err, pgx.ErrNoRows) {
		var exists bool
		if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM events WHERE id=$1)", r.EventID).Scan(&exists); err != nil {
			return Order{}, false, err
		}
		if !exists {
			return Order{}, false, ErrNotFound
		}
		return Order{}, false, ErrSoldOut
	}
	if err != nil {
		return Order{}, false, err
	}
	id := UUID()
	o, err = scanOrder(tx.QueryRow(ctx, "INSERT INTO orders(id,event_id,quantity,total_cents,status,payment_scenario,idempotency_key) VALUES($1,$2,$3,$4,'pending',$5,$6) RETURNING "+orderColumns, id, r.EventID, r.Quantity, price*r.Quantity, r.PaymentScenario, key))
	if err != nil {
		return Order{}, false, err
	}
	if _, err = tx.Exec(ctx, "INSERT INTO payment_jobs(order_id) VALUES($1)", id); err != nil {
		return Order{}, false, err
	}
	return o, true, tx.Commit(ctx)
}

type Job struct {
	OrderID  string
	Scenario string
	Attempts int
	Token    string
}

func (s *Store) Claim(ctx context.Context, lease time.Duration) (Job, error) {
	var j Job
	token := UUID()
	err := s.Pool.QueryRow(ctx, `WITH candidate AS (
 SELECT order_id FROM payment_jobs WHERE completed_at IS NULL AND available_at <= now()
 AND (lease_until IS NULL OR lease_until <= now()) ORDER BY available_at
 FOR UPDATE SKIP LOCKED LIMIT 1
) UPDATE payment_jobs p SET attempts=attempts+1,lease_until=now()+($1 * interval '1 second'),lease_token=$2
FROM candidate c,orders o WHERE p.order_id=c.order_id AND o.id=p.order_id
RETURNING p.order_id::text,o.payment_scenario,p.attempts,p.lease_token::text`, lease.Seconds(), token).Scan(&j.OrderID, &j.Scenario, &j.Attempts, &j.Token)
	if errors.Is(err, pgx.ErrNoRows) {
		err = ErrNotFound
	}
	return j, err
}
func (s *Store) Retry(ctx context.Context, j Job, delay time.Duration, message string) error {
	tag, err := s.Pool.Exec(ctx, `UPDATE payment_jobs SET lease_until=NULL,lease_token=NULL,available_at=now()+($3 * interval '1 second'),last_error=$4 WHERE order_id=$1 AND lease_token=$2 AND lease_until>now() AND completed_at IS NULL`, j.OrderID, j.Token, delay.Seconds(), message)
	if err == nil && tag.RowsAffected() != 1 {
		return ErrLeaseLost
	}
	return err
}
func (s *Store) Complete(ctx context.Context, j Job, confirmed bool) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var id string
	err = tx.QueryRow(ctx, `UPDATE payment_jobs SET completed_at=now(),lease_until=NULL,lease_token=NULL WHERE order_id=$1 AND lease_token=$2 AND lease_until>now() AND completed_at IS NULL RETURNING order_id::text`, j.OrderID, j.Token).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrLeaseLost
	}
	if err != nil {
		return err
	}
	status := "failed"
	if confirmed {
		status = "confirmed"
	}
	var event string
	var quantity int
	err = tx.QueryRow(ctx, "UPDATE orders SET status=$2,updated_at=now() WHERE id=$1 AND status='pending' RETURNING event_id,quantity", id, status).Scan(&event, &quantity)
	if err != nil {
		return err
	}
	if !confirmed {
		if _, err = tx.Exec(ctx, "UPDATE events SET available=available+$2 WHERE id=$1", event, quantity); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
func (s *Store) Backlog(ctx context.Context) (float64, float64, error) {
	var count, age float64
	err := s.Pool.QueryRow(ctx, `SELECT count(*)::float8,COALESCE(EXTRACT(EPOCH FROM now()-min(o.created_at)),0)::float8 FROM payment_jobs p JOIN orders o ON o.id=p.order_id WHERE p.completed_at IS NULL`).Scan(&count, &age)
	return count, age, err
}
func UUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
func IsUnavailable(err error) bool {
	var pgerr *pgconn.PgError
	return errors.As(err, &pgerr) && (pgerr.Code == "53300" || pgerr.Code == "57P01")
}
