# go-api

REST API to manage products, written in **Go** with **Gin** and **PostgreSQL**,
using a layered architecture with manual dependency injection.

> Written for developers who already know how to code (the author is a senior
> frontend developer) but are learning Go. The docs below explain each Go
> concept with a frontend (JS/TS) analogy.

## Stack

- Go standard library (`net/http`, `database/sql`)
- Gin HTTP framework
- PostgreSQL (via `lib/pq`)
- Docker Compose for the local database

## Quick start

Prerequisites: Go 1.26+, Docker.

```bash
# start the database and apply the schema
docker compose up -d
docker compose exec go_db psql -U root -d postgres \
  -f /dev/stdin < internal/db/migrations/000001_create_products.sql

# run the API
go run ./cmd/api
```

Test it:

```bash
curl -i localhost:3000/health
curl -i -X POST localhost:3000/products \
  -H 'Content-Type: application/json' \
  -d '{"name":"Keyboard","price":199.90}'
curl -i localhost:3000/products
```

## Configuration

Environment variables (see `.env.example`):

| Variable       | Default                                                       | Description            |
| -------------- | ------------------------------------------------------------- | ---------------------- |
| `SERVER_PORT`  | `3000`                                                        | HTTP server port       |
| `DATABASE_URL` | `postgres://root:root@localhost:5432/postgres?sslmode=disable` | Postgres connection string |

## Project layout

```
cmd/api/                 entrypoint: config, dependency injection, graceful shutdown
internal/config/         env-based configuration
internal/domain/         entities and validation (no framework dependencies)
internal/repository/     data contract (interface) + Postgres implementation
internal/usecase/        business rules
internal/handler/        HTTP layer: parsing, status codes, JSON
internal/router/         URL → handler routes
internal/db/             connection pool
internal/db/migrations/  schema versioning
```

## Documentation

The source files are commented line by line. Deeper guides live under `docs/`:

| Guide                                                                          | What it covers                                                              |
| ------------------------------------------------------------------------------ | --------------------------------------------------------------------------- |
| [Go fundamentals (frontend → Go)](docs/go-fundamentals.md)                     | Go concepts you will meet, explained with JS/TS analogies                    |
| [Architecture: layers and how they connect](docs/architecture.md)              | Every layer in detail, dependency rules, and the full request flow           |
| [Replication guide: build the same thing elsewhere](docs/replication-guide.md) | Start a new project, add a new resource, and test without a database        |
| [Glossary (Go + project dictionary)](docs/glossary.md)                         | Every term you meet, from `defer` to `RETURNING`, with meanings and analogies |