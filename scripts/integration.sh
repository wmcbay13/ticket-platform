#!/usr/bin/env bash
source "$(dirname "$0")/common.sh"
name="ticket-integration-$$"
cleanup() { docker rm -f "$name" >/dev/null 2>&1 || true; }
trap cleanup EXIT
docker run -d --name "$name" -e POSTGRES_PASSWORD=integration-only -e POSTGRES_DB=tickets -p 127.0.0.1::5432 "$POSTGRES_IMAGE" >/dev/null
for attempt in $(seq 1 60); do if docker exec "$name" pg_isready -U postgres -d tickets >/dev/null 2>&1; then break; fi; sleep 1; done
port="$(docker port "$name" 5432/tcp | cut -d: -f2)"
export TEST_DATABASE_URL="postgres://postgres:integration-only@127.0.0.1:$port/tickets?sslmode=disable"
go test -race -count=1 ./...
