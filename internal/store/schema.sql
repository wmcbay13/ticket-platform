CREATE TABLE IF NOT EXISTS events (
    id text PRIMARY KEY,
    title text NOT NULL,
    venue text NOT NULL,
    starts_at timestamptz NOT NULL,
    description text NOT NULL,
    price_cents integer NOT NULL CHECK (price_cents >= 0),
    capacity integer NOT NULL CHECK (capacity >= 0),
    available integer NOT NULL CHECK (available >= 0 AND available <= capacity)
);
CREATE TABLE IF NOT EXISTS orders (
    id uuid PRIMARY KEY,
    event_id text NOT NULL REFERENCES events(id),
    quantity integer NOT NULL CHECK (quantity BETWEEN 1 AND 10),
    total_cents integer NOT NULL CHECK (total_cents >= 0),
    status text NOT NULL CHECK (status IN ('pending', 'confirmed', 'failed')),
    payment_scenario text NOT NULL CHECK (payment_scenario IN ('success', 'decline', 'retry')),
    idempotency_key text NOT NULL UNIQUE,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS payment_jobs (
    order_id uuid PRIMARY KEY REFERENCES orders(id),
    attempts integer NOT NULL DEFAULT 0,
    available_at timestamptz NOT NULL DEFAULT now(),
    lease_until timestamptz,
    lease_token uuid,
    completed_at timestamptz,
    last_error text
);
CREATE INDEX IF NOT EXISTS payment_jobs_ready ON payment_jobs(available_at) WHERE completed_at IS NULL;
CREATE TABLE IF NOT EXISTS schema_migrations (version integer PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now());
