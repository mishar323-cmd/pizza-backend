# pizza-backend

Go HTTP API for the Дело в пицце order/admin/payment flow.

## Stack
- Go 1.23, stdlib `net/http`
- pgx/v5 → PostgreSQL 16
- golang-jwt/v5 for admin auth (bcrypt, 7-day tokens)
- YooKassa REST (54-ФЗ receipts), Telegram Bot API

## Run locally

```sh
cp .env.example .env   # then fill in DB url, JWT_SECRET, YooKassa, Telegram
go run ./cmd/server    # listens on :8080
```

Needs Postgres reachable at `$DATABASE_URL`. The fastest way to run one:
```sh
docker run -d --name pizza-pg -e POSTGRES_PASSWORD=pizza -p 5432:5432 postgres:16-alpine
DATABASE_URL=postgres://postgres:pizza@localhost:5432/postgres?sslmode=disable go run ./cmd/server
```

Migrations are embedded (`internal/db/migrations/`) and run automatically at startup.

## Docker

```sh
docker build -t pizza-backend .
docker run --rm -p 8080:8080 --env-file .env pizza-backend
```

## CI / release

- `ci.yml`: vet + build + test on every push/PR
- `release.yml`: on push to `main` or `dev`, builds image and pushes to `ghcr.io/mishar323-cmd/pizza-backend:{latest|dev|sha-XXXX}`. On `main`, also triggers `pizza-infra` deploy via `repository_dispatch`.

## Branches

- `main` — production
- `dev` — staging tag, no auto-deploy
