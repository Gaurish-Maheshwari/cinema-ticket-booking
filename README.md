# Cinema Ticket Booking

A small Go backend for booking cinema seats without double-booking. Seats are held temporarily in Redis, then confirmed or released.

## How it works

1. A user **holds** a seat, which is reserved for 2 minutes.
2. The user **confirms** it, which makes the booking permanent, or **releases** it.
3. If neither happens, the hold expires and the seat becomes free again.

Holds use Redis `SET NX` with a TTL, which is atomic. Even with 100k users racing for one seat, exactly one wins.

## Tech stack

- Go (`net/http`, 1.22+ routing)
- Redis 7 (via `go-redis`)
- Docker Compose (Redis + Redis Commander)
- Plain HTML frontend in `static/`

## Project structure

```
cmd/main.go                    # entrypoint, routes
internal/adapters/redis/       # Redis client setup
internal/booking/              # domain, service, handlers, stores, tests
internal/utils/                # JSON response helper
static/index.html              # simple UI
docker-compose.yaml            # Redis + Redis Commander
```

## Getting started

**Prerequisites:** Go 1.22+ and Docker.

```bash
# 1. Start Redis
docker compose up -d

# 2. Install dependencies
go mod tidy

# 3. Run the server (from the repo root)
go run ./cmd
```

Open http://localhost:8080.
Redis Commander (browse keys): http://localhost:8081

Stop Redis with `docker compose down`.

## API

| Method | Endpoint | Body | Description |
|--------|----------|------|-------------|
| GET | `/movies` | none | List movies |
| GET | `/movies/{movieID}/seats` | none | List held/booked seats |
| POST | `/movies/{movieID}/seats/{seatID}/hold` | `{"user_id": "u1"}` | Hold a seat for 2 minutes |
| PUT | `/sessions/{sessionID}/confirm` | `{"user_id": "u1"}` | Confirm a held seat |
| DELETE | `/sessions/{sessionID}` | `{"user_id": "u1"}` | Release a held seat |

Status codes: `201` held, `200` confirmed, `204` released, `400` bad request, `403` not your session, `404` session not found or expired, `409` seat already taken.

Example:

```bash
curl -X POST localhost:8080/movies/inception/seats/A1/hold \
  -H "Content-Type: application/json" \
  -d '{"user_id": "u1"}'
```

## Redis keys

```
seat:{movieID}:{seatID}  ->  booking JSON (has TTL while held, none once confirmed)
session:{sessionID}      ->  seat key (reverse lookup)
```

## Tests

Redis must be running:

```bash
go test ./internal/booking -run TestConcurrentBooking -v -count=1
```

The test fires 100k concurrent bookings at one seat and asserts exactly one succeeds.

## Notes

- `memory_store.go` and `concurrent_store.go` are earlier in-memory versions and are not used by the app.
- The Redis address (`localhost:6379`) is hardcoded in `cmd/main.go`.