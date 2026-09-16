# Architecture: layers and how they connect

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

## File → layer map

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

## Dependency rule (imports)

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

## Each layer in detail

Every line of the files is commented in the code itself. Here is the **why**
behind each layer.

### domain — the pure core

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

### config — no magic values in the code

`Config` aggregates ports and the connection string read from environment
variables. `getEnv(key, fallback)` provides defaults for local development.

**In practice:** nobody should have to open the code to know which port the
API uses. Document everything in `.env.example`.

### db — the connection pool

`sql.Open` only registers the driver; `PingContext` actually validates. Here
we also configure the **pool** (`SetMaxOpenConns`, `SetMaxIdleConns`,
`SetConnMaxLifetime`) so the app does not overwhelm Postgres under
concurrency.

**In practice:** manually creating one connection per request is a classic
mistake for people coming from other languages. **A single pool** is created
at boot and shared by all goroutines.

### repository — interface + implementation in the same package

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

### usecase — where the business rule lives

The handler is deliberately "dumb": it parses and returns JSON. The `usecase`
decides **when and how** things happen:

1. **Validates** the product (business rule before touching the DB);
2. **Persists** by calling the repository;
3. **Enriches** the result (e.g. fills in the `ID`) and returns it.

The `ProductUsecase` interface also exists so the handler can receive a mock
in tests.

### handler — the HTTP boundary

- Receives `*gin.Context`.
- `ShouldBindJSON` parses the body (format validation).
- Calls the use case and turns the result into status + JSON.
- **Error handling**: `errors.Is(err, domain.ErrInvalidProduct)` → 400;
  anything else → log (`ctx.Error`) + generic 500 (no internal detail leak).

### router — the route table

A simple, centralized map:

```go
router.GET("/products", productHandler.GetProducts)
router.POST("/products", productHandler.CreateProduct)
```

The returned `*gin.Engine` implements `http.Handler`, so it is passed to the
official `http.Server`.

### main — the composition root

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

## Full request flow

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