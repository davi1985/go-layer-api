# go-api — Product API in Go

REST API for products written in **Go** with **Gin** and **PostgreSQL**, using
a layered architecture with manual dependency injection.

This README was written for someone who **already knows how to code** (the
author is a senior frontend developer) but is **learning Go**. So every
concept comes with a frontend (JS/TS) analogy and a step-by-step guide to
**replicate this structure in another application** — not just understand
this one.

---

## Table of contents

1. [What you need to run](#1-what-you-need-to-run)
2. [Go fundamentals you will come across (frontend → Go)](#2-go-fundamentals-you-will-come-across-frontend--go)
3. [Architecture: the layers and how they connect](#3-architecture-the-layers-and-how-they-connect)
4. [Each layer in detail](#4-each-layer-in-detail)
5. [Full request flow](#5-full-request-flow)
6. [How to replicate it in another application](#6-how-to-replicate-it-in-another-application)
7. [Golden rules and common pitfalls](#7-golden-rules-and-common-pitfalls)

---

## 1. What you need to run

Prerequisites: Go 1.26+, Docker (for the database) or an accessible Postgres.

```bash
# 1) Start the Postgres database (background)
docker compose up -d

# 2) Check the database is ready
docker compose ps

# 3) Create the product table (apply the migration)
docker compose exec go_db psql -U root -d postgres \
  -f /dev/stdin < internal/db/migrations/000001_create_products.sql

# 4) Run the API
go run ./cmd/api
```

Quick test:

```bash
curl -i localhost:3000/health
curl -i -X POST localhost:3000/products \
  -H 'Content-Type: application/json' \
  -d '{"name":"Keyboard","price":199.90}'
curl -i localhost:3000/products
```

Configuration comes from environment variables (see `.env.example`):

| Variable        | Default                                                   | What it does                |
| --------------- | --------------------------------------------------------- | --------------------------- |
| `SERVER_PORT`   | `3000`                                                    | HTTP server port            |
| `DATABASE_URL`  | `postgres://root:root@localhost:5432/postgres?sslmode=disable` | Postgres connection string  |

---

## 2. Go fundamentals you will come across (frontend → Go)

These are the language pieces that show up in **every file**. Read them once,
then come back here whenever something confuses you.

### 2.1 Packages and imports (≈ ES modules + export)

- A `package` is a folder (or a set of files) that shares the same
  `package name` on the first line.
- A symbol (function/type/field) starting with an **uppercase** letter is
  **exported** (public). **Lowercase** means private, visible only inside the
  package.
- Package names are **singular and lowercase**: `usecase`, `handler`,
  `domain`.

```go
package config // every file under internal/config/ starts like this

type Config struct { ... }      // exported (other packages use it)
func Load() *Config { ... }     // exported
func getEnv(...) string { ... } // private (only the config package uses it)
```

### 2.2 Struct + method with receiver (≈ class/type + method)

A `struct` is an "object". A method in Go is a function with a **receiver**
(who receives the call) — like `this`, but explicit:

```go
type Product struct {
	ID    int     // field
	Name  string
	Price float64
}

// (p *Product) is the receiver. `p` plays the role of `this`.
func (p *Product) Validate() error { ... }
```

Usage: `product.Validate()`. The receiver can be a **value** (`p Product`) or
a **pointer** (`p *Product`). Use a pointer when the method mutates fields
or when the struct is large (avoids copying data all the time).

### 2.3 Pointers: `&` and `*` (≈ references, but explicit)

In JS you never think about it: objects are always references. In Go you
choose.

- `&x` → "address of x" (creates a pointer to x)
- `*T` → "pointer to T" (type); `*ptr` → "the value at that address"

```go
var x int = 10
var p *int = &x // p points to x
*p = 20         // changes x (x becomes 20)
```

Why it matters here: `rows.Scan(&product.ID, ...)` needs the address to write
values into the fields; `db *sql.DB` in the repository is a pointer to the
connection pool (nobody wants to copy that).

### 2.4 Interfaces are STRUCTURAL (≈ TS interface, without `implements`)

In Go, a type **implements an interface automatically** if it has the methods
with the right signatures. There is no explicit `implements`.

```go
type ProductRepository interface {
	GetProducts(ctx context.Context) ([]domain.Product, error)
}

// postgresProductRepository only needs to HAVE a GetProducts method with the
// same signature. Automatically it "is" a ProductRepository.
type postgresProductRepository struct{ db *sql.DB }

func (r *postgresProductRepository) GetProducts(ctx context.Context) ([]domain.Product, error) {
	// implementation
}
```

This is the foundation of dependency injection: the `usecase` says "give me
anything that fulfills this contract". A test can pass a **mock** instead.

> Codebase idiom: *"accept interfaces, return structs"* — whoever **uses** a
> dependency receives the **interface**; whoever **creates** it returns the
> **concrete type**. In this project the factories (`NewProductRepository`,
> `NewProductUsecase`) return the interface to make test mocks easier — a
> common, acceptable deviation in clean architecture; just be aware of what
> you are doing.

### 2.5 Errors are values, not exceptions (≈ no try/catch)

Go has no `throw`. A function that can fail returns:
`(result, error)`. You **always** check the error by hand.

```go
products, err := u.repo.GetProducts(ctx)
if err != nil {                  // the equivalent of `if (err) throw err`
	return nil, fmt.Errorf("failed: %w", err)
}
```

- `fmt.Errorf("...%w", err)` **wraps** the original error, preserving the cause.
- `errors.Is(err, SentinelErr)` checks whether the **chain** contains a
  specific error (useful to decide between 400 and 500).
- `nil` means "no error".
- Convention: log in the **handler**, never expose details in the client JSON.

### 2.6 `context.Context` — the request-lifetime "extension cord"

`context` carries state down the chain: cancellation (client disconnected,
timeout, Ctrl+C). **Convention:** it is always the **first parameter** of
functions that do I/O (database, HTTP).

```go
func GetProducts(ctx context.Context) (...) // handler passes ctx.Request.Context()
// -> use case forwards it
// -> repository passes it to db.QueryContext(ctx, query)
// the driver aborts the query if ctx is canceled
```

### 2.7 `defer` — "run when the function finishes" (≈ finally)

```go
rows, _ := r.db.QueryContext(ctx, query)
defer rows.Close() // runs at the end of the function, even on return/panic
```

### 2.8 Goroutines (≈ light threads, no async/await)

In `cmd/api/main.go` the server's `ListenAndServe()` **blocks**. We run it in
parallel with `go func() { ... }()` while the main flow waits for the
shutdown signal.

### 2.9 Slices: `[]T`, `make`, `append`, and the `nil` trap

```go
var s []string         // nil slice -> becomes `null` in JSON
s = make([]string, 0)  // empty slice -> becomes `[]` in JSON ✅
s = append(s, "a")     // push
```

That is why the repository uses `products := make([]domain.Product, 0)`.

### 2.10 Zero value (≈ undefined, but always defined)

Every type has an automatic "zero value": `int` → `0`, `string` → `""`,
`*T` → `nil`, `struct` → everything zero. That is why
`var product domain.Product` is already a valid product (with `""`, `0`, `0`).

---

## 3. Architecture: the layers and how they connect

The project follows a **layered architecture** (clean architecture style).
The main rule:

> **The dependency flow is ONE-DIRECTIONAL: top down.**
> Handler calls Use case, Use case calls Repository, Repository uses the DB.
> **Never** the other way around. And `domain` imports nobody.

```
                ┌─────────────────────────────────────────────────┐
                │  cmd/api (entrypoint)                            │
                │  reads config, connects DB, INJECTS DEPENDENCIES │
                │  and starts the HTTP server                      │
                └──────────────────────┬──────────────────────────┘
                                       │ injects handler into the router
                                       ▼
                ┌─────────────────────────────────────────────────┐
                │  router  (internal/router)                       │
                │  map: "URL → handler method"                     │
                └──────────────────────┬──────────────────────────┘
                                       │ calls the route's handler
                                       ▼
                ┌─────────────────────────────────────────────────┐
   HTTP ───────►│  handler (internal/handler)                      │
                │  HTTP layer: body parsing, status codes, JSON    │
                └──────────────────────┬──────────────────────────┘
                                       │ uses the use case (interface)
                                       ▼
                ┌─────────────────────────────────────────────────┐
                │  usecase (internal/usecase)                      │
                │  BUSINESS RULES: validate › orchestrate › reply  │
                └──────────────────────┬──────────────────────────┘
                                       │ uses the repository (interface)
                                       ▼
                ┌─────────────────────────────────────────────────┐
                │  repository (internal/repository)                │
                │  CONTRACT (interface) + Postgres SQL             │
                └──────────────────────┬──────────────────────────┘
                                       │ uses the *sql.DB
                                       ▼
                ┌─────────────────────────────────────────────────┐
                │  db (internal/db) → Postgres connection pool      │
                └─────────────────────────────────────────────────┘

  domain (internal/domain) = pure entities, imports NOTHING from above.
  config (internal/config) = configuration read from environment variables.
```

**Why separate into layers?**

1. **Testability**: each layer tests the one below it with a mock (swap the
   real database for a fake that fulfills the same interface).
2. **Swapability**: swapping `gin` for another HTTP framework, or raw SQL for
   an ORM, does not touch `usecase`/`domain`.
3. **Single responsibility**: the handler knows no SQL; the repository knows
   nothing about HTTP.

### File → layer map

| File                                            | Layer              | What it holds                    |
| ----------------------------------------------- | ------------------ | -------------------------------- |
| `cmd/api/main.go`                               | Bootstrap          | config, db, DI, http.Server      |
| `internal/config/config.go`                     | Configuration      | environment variables            |
| `internal/domain/product.go`                    | Domain             | entity + validation + sentinel error |
| `internal/db/conn.go`                           | DB infrastructure  | opens/configures the pool        |
| `internal/repository/product.go`                | Data contract      | `ProductRepository` interface    |
| `internal/repository/product_postgres.go`       | Contract impl      | real SQL + factory               |
| `internal/usecase/product.go`                   | Business rules     | interface + implementation       |
| `internal/handler/product.go`                   | HTTP               | parsing, status codes, JSON      |
| `internal/router/router.go`                     | HTTP               | URL → handler map                |
| `internal/db/migrations/*.sql`                  | Schema             | database versioning              |

**Dependency rule (imports):**

```
main      imports: config, db, handler, repository, router, usecase
router    imports: handler
handler   imports: domain, usecase
usecase   imports: domain, repository
repository imports: domain (and database/sql in the implementation)
db        imports: only libraries (lib/pq)
domain    imports: only the standard library (errors)
config    imports: only the standard library (os)
```

If some day you see a `usecase` importing `gin`, or a `domain` importing
`database/sql`, it is an inverted dependency — refactor it.

---

## 4. Each layer in detail

Every line of the files is commented in the code itself. Here is the **why**
behind each layer.

### 4.1 `domain` — the pure core

It holds the `Product` entity and its rule (`Validate`). It imports nothing
beyond the standard library.

```go
func (p *Product) Validate() error {
	if p.Name == "" {
		return errors.New("product name is required")
	}
	if p.Price < 0 {
		return errors.New("price cannot be negative")
	}
	return nil
}
```

`ErrInvalidProduct` is a **sentinel error**: it lets the handler distinguish
"the client sent bad data (400)" from "it broke on the server (500)". That is
Go's `errors.Is` idiom — no exception class per error type.

**In practice:** start every feature in `domain`. First what data the entity
has, then what rules it applies by itself.

### 4.2 `config` — no magic values in the code

`Config` aggregates ports and the connection string read from environment
variables. `getEnv(key, fallback)` provides defaults for local development.

**In practice:** nobody should have to open the code to know which port the
API uses. Document everything in `.env.example`.

### 4.3 `db` — the connection pool

`sql.Open` only registers the driver; `PingContext` actually validates. Here
we also configure the **pool** (`SetMaxOpenConns`, `SetMaxIdleConns`,
`SetConnMaxLifetime`) so the app does not overwhelm Postgres under
concurrency.

**In practice:** manually creating one connection per request is a classic
mistake for people coming from other languages. **A single pool** is created
at boot and shared by all goroutines.

### 4.4 `repository` — interface + implementation in the same package

Two files, same folder, same `package repository`:

- `product.go` — the **interface** (contract). Whoever consumes it (use case)
  depends on this.
- `product_postgres.go` — the **implementation**
  (`postgresProductRepository`) with parameterized SQL. The lowercase type
  name signals "not for external use".

`database/sql` idioms:

- **Never concatenate values into the query** → use `$1, $2` (parameters).
  Otherwise you have SQL injection.
- `QueryContext` → SELECT (0 or N rows), iterate with `rows.Next()`.
- `QueryRowContext` → SELECT/INSERT returning **1 row**.
- `Scan(&field)` copies the column into the field (in SELECT order).
- `defer rows.Close()` after every Query that opens rows.
- `rows.Err()` checked **after** the loop.

> `_ "github.com/lib/pq"` (blank import) runs the driver's `init()`, which
> registers itself in `database/sql`. You never call lib/pq directly — only
> `sql.Open("postgres", dsn)`.

### 4.5 `usecase` — where the business rule lives

The handler is deliberately "dumb": it parses and returns JSON. The `usecase`
decides **when and how** things happen:

1. **Validates** the product (business rule before touching the DB);
2. **Persists** by calling the repository;
3. **Enriches** the result (e.g. fills in the `ID`) and returns it.

The `ProductUsecase` interface also exists so the handler can receive a mock
in tests.

### 4.6 `handler` — the HTTP boundary

- Receives `*gin.Context`.
- `ShouldBindJSON` parses the body (format validation).
- Calls the use case and turns the result into status + JSON.
- **Error handling**: `errors.Is(err, domain.ErrInvalidProduct)` → 400;
  anything else → log (`ctx.Error`) + generic 500 (no internal detail leak).

### 4.7 `router` — the route table

A simple, centralized map:

```go
router.GET("/products", productHandler.GetProducts)
router.POST("/products", productHandler.CreateProduct)
```

The returned `*gin.Engine` implements `http.Handler`, so it is passed to the
official `http.Server`.

### 4.8 `main` — the composition root

The **single** place that knows all layers and assembles the pyramid:

```go
productRepository := repository.NewProductRepository(conn) // SQL -> repo
productUsecase    := usecase.NewProductUsecase(productRepository) // repo -> usecase
productHandler    := handler.NewProductHandler(productUsecase) // usecase -> handler
server.Handler     = router.New(productHandler) // handler -> routes -> server
```

It is also main that guarantees **graceful shutdown**: a cancellable context
with `signal.NotifyContext`, an `http.Server` with timeouts, and
`server.Shutdown` giving in-flight requests up to 5s to finish.

---

## 5. Full request flow

### `GET /products`

```
Client
  │ 1. GET /products
  ▼
http.Server (cmd/api) ──2──► router ──3──► ProductHandler.GetProducts
                                              │ 4. ctx.Request.Context()
                                              ▼
                                        ProductUsecase.GetProducts
                                              │ 5. forwards ctx
                                              ▼
                                        ProductRepository.GetProducts (interface)
                                              │ 6. calls the impl
                                              ▼
                                        postgresProductRepository.GetProducts
                                              │ 7. SELECT ... (cancellable via ctx)
                                              ▼
                                              Postgres
  ◄── 200 OK [ {id, name, price}, ... ] ──────────────┘
```

### `POST /products`

```
Client
  │ 1. POST /products  body: {"name":"Keyboard","price":199.90}
  ▼
http.Server ──2──► router ──3──► ProductHandler.CreateProduct
     4. ShouldBindJSON (malformed -> 400)
     5. ProductUsecase.CreateProduct
          6. Product.Validate()           (rule: empty name/price<0 -> 400)
          7. repo.CreateProduct(ctx, ...) (INSERT ... RETURNING id)
     8. product.ID = id
  ◄── 201 Created  {id: 1, name: "Keyboard", price: 199.90}
```

Notice: **format** validation (JSON) happens in the handler; **business**
validation happens in the use case. Two layers, two kinds of "400".

---

## 6. How to replicate it in another application

Two paths: **starting from scratch** and **adding a new resource** to an
existing project. Both use the same checklist.

### 6.1 Start a new project from scratch

```bash
# creates a module (Go's "package.json")
go mod init github.com/youruser/your-api

# dependencies
go get github.com/gin-gonic/gin
go get github.com/lib/pq
```

Create the folder structure:

```bash
├── cmd/api/main.go
├── internal/
│   ├── config/config.go
│   ├── db/conn.go
│   ├── domain/
│   ├── repository/
│   ├── usecase/
│   ├── handler/
│   └── router/router.go
```

Then just follow the layer order below (each item is the skeleton of code
already commented in this project — copy and adapt).

1. **`config`** → `Config` + `Load()` + `getEnv(key, fallback)`.
2. **`db`** → `Connect(ctx, dsn)` opening the pool and doing `PingContext`.
3. **`domain`** → your entity + `Validate()` + sentinel errors.
4. **`repository`** → interface + implementation with parameterized SQL.
5. **`usecase`** → interface + implementation orchestrating
   validate→persist→enrich.
6. **`handler`** → struct with the use case interface + a handler per
   operation.
7. **`router`** → `New(...)` mapping URL → handler.
8. **`main`** → assembles everything and starts the server with graceful
   shutdown.
9. **`.env.example`** + **docker-compose.yml** + **migrations**.
10. **Compile early and often**: `go run ./cmd/api`, `go vet ./...`,
    `gofmt -l .`

### 6.2 Add a new resource (e.g. "Category")

Checklist used by any feature:

- [ ] **1. domain/category.go** — entity + validation.
- [ ] **2. repository/category.go** — interface (contract).
- [ ] **3. repository/category_postgres.go** — SQL + factory `NewCategoryRepository`.
- [ ] **4. usecase/category.go** — rules (validate before persisting).
- [ ] **5. handler/category.go** — HTTP operations.
- [ ] **6. router** — register the new routes.
- [ ] **7. main** — inject the new dependencies.
- [ ] **8. migrations** — a new SQL file (never edit a previous one).

Quick skeleton of that feature (notice the symmetry with `Product`):

```go
// internal/domain/category.go
package domain

type Category struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func (c *Category) Validate() error {
	if c.Name == "" {
		return errors.New("category name is required")
	}
	return nil
}
```

```go
// internal/repository/category.go
package repository

type CategoryRepository interface {
	GetCategories(ctx context.Context) ([]domain.Category, error)
	CreateCategory(ctx context.Context, c domain.Category) (int, error)
}
```

```go
// internal/usecase/category.go
package usecase

type CategoryUsecase interface {
	CreateCategory(ctx context.Context, c domain.Category) (domain.Category, error)
}

type categoryUsecase struct {
	repo repository.CategoryRepository
}

func NewCategoryUsecase(repo repository.CategoryRepository) CategoryUsecase {
	return &categoryUsecase{repo: repo}
}

func (u *categoryUsecase) CreateCategory(ctx context.Context, c domain.Category) (domain.Category, error) {
	if err := c.Validate(); err != nil {
		return domain.Category{}, fmt.Errorf("%w: %s", domain.ErrInvalidCategory, err.Error())
	}
	id, err := u.repo.CreateCategory(ctx, c)
	if err != nil {
		return domain.Category{}, err
	}
	c.ID = id
	return c, nil
}
```

```go
// internal/handler/category.go (skeleton only)
type CategoryHandler struct {
	categoryUsecase usecase.CategoryUsecase
}

func NewCategoryHandler(u usecase.CategoryUsecase) *CategoryHandler {
	return &CategoryHandler{categoryUsecase: u}
}

func (h *CategoryHandler) CreateCategory(ctx *gin.Context) {
	var c domain.Category
	if err := ctx.ShouldBindJSON(&c); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	created, err := h.categoryUsecase.CreateCategory(ctx.Request.Context(), c)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidCategory) {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		ctx.Error(err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	ctx.JSON(http.StatusCreated, created)
}
```

```go
// cmd/api/main.go — the new dependency wires
categoryRepo    := repository.NewCategoryRepository(conn)
categoryUsecase := usecase.NewCategoryUsecase(categoryRepo)
categoryHandler := handler.NewCategoryHandler(categoryUsecase)
// router.New(categoryHandler, ...) and register the routes
```

### 6.3 Testing the use case without a database (why the interfaces exist)

```go
// usecase/product_test.go (example)
type productRepoStub struct {
	createErr error
	nextID    int
}

// satisfies the ProductRepository interface in the test
func (s *productRepoStub) GetProducts(ctx context.Context) ([]domain.Product, error) {
	return []domain.Product{{ID: 1, Name: "x", Price: 1}}, nil
}
func (s *productRepoStub) CreateProduct(ctx context.Context, p domain.Product) (int, error) {
	if s.createErr != nil {
		return 0, s.createErr
	}
	return s.nextID, nil
}
```

Because `productRepoStub` has the interface's methods, `NewProductUsecase`
accepts it. **No database, no Postgres, no Docker** — this is why the
business layer depends on an interface, not a concrete struct.

---

## 7. Golden rules and common pitfalls

1. **Dependencies point inward** — `handler → usecase → repository → db`,
   and `domain` imports no framework.
2. **`context` as the first parameter** in any I/O function, always.
3. **Errors**: propagate with `%w`, log in the handler, return a generic
   message to the client (never raw `err.Error()` on a 500).
4. **Parameterized SQL** (`$1, $2`) to avoid injection; `defer rows.Close()`
   after every `Query`; `rows.Err()` after the loop.
5. **Business validation in the use case**, parsing in the handler. Two kinds
   of "400".
6. **Singular, lowercase package names**. It is `usecase`, not `usecases`.
7. **JSON for empty lists**: use `make([]T, 0)` (not `var s []T`), otherwise
   the client receives `null`.
8. **Never `panic` at runtime** for expected situations (DB down, bad
   request). `panic` is for bugs; errors are values.
9. **Always run**: `go vet ./...`, `gofmt -l .`, `go build ./...` before
   closing a task.
10. **One connection pool** at boot — never open a connection per request.

### Possible next steps

- Pagination in `GET /products` (e.g. `?limit&offset`).
- A real migration tool (e.g. `golang-migrate`, `atlas`, `goose`) — today the
  migrations are SQL files applied by hand.
- `GET /products/:id`, `PUT`, `DELETE`.
- Middlewares (CORS, JWT auth) in `router`.
- Unit tests for the use case with a stub (see [6.3](#63-testing-the-use-case-without-a-database-why-the-interfaces-exist))
  and handler tests with `httptest`.