# Glossary: dictionary of the Go and project words

One-stop shop for every term you will meet while reading this project. Each
entry says what it is, what it means **in this project**, and (when useful) the
frontend (JS/TS) analogy. Items are grouped by tema and alphabetically within
each group.

## Go language basics

| Term | Meaning | In this project | Frontend analogy |
| ---- | ------- | --------------- | ---------------- |
| **Package** | A folder of `.go` files sharing the same `package` name. The unit of organization and visibility. | `config`, `db`, `domain`, `repository`, `usecase`, `handler`, `router`, `main` | An ES module (file/folder) with its own exports. |
| **Import** | Brings another package's exported symbols into this file. | `import "fmt"`, `import "github.com/gin-gonic/gin"` | `import { x } from "..."` / `require`. |
| **Module** | The whole Go project + its dependencies; declared in `go.mod` (Go's `package.json`). | `module github.com/davi1985/go-api` | `package.json` + `node_modules`. |
| **Struct** | A custom "object type" made of named fields. | `Product`, `Config`, `http.Server` | A `class`/`interface`/`type` with fields. |
| **Field** | A named piece of data inside a struct. | `ID`, `Name`, `Price` of `Product` | A property of an object. |
| **Exported / public** | Symbols written with an **uppercase** first letter; usable by other packages. | `Load()`, `Config`, `ProductRepository` | `export`ed symbols of a module. |
| **Unexported / private** | Symbols written with a **lowercase** first letter; visible only inside their package. | `getEnv`, `postgresProductRepository` | A helper not exported from the module. |
| **Receiver** | The "this" of a method: `(r *postgresProductRepository)` before the method name. | `func (r *postgresProductRepository) GetProducts(...)` | `this`/`self` in class methods (but explicit). |
| **Pointer** | A value that holds the memory *address* of another value. `&x` = address of `x`; `*T` = pointer to `T`. | `&product.ID` in `Scan`, `db *sql.DB` | References (always implicit in JS objects). |
| **Zero value** | Automatic default every value has before assignment: `0`, `""`, `nil`. | `var product domain.Product` starts as `{0, "", 0}` | `undefined`, but always "defined". |
| **Slice** | A dynamic array (`[]T`). Built with `make`, grown with `append`. | `products := make([]domain.Product, 0)` | JS `Array` (push). |
| **Map** | A key→value collection (`map[K]V`). | `gin.H{"error": "..."}` is `map[string]any` | JS `Object` (but typed keys). |
| **Function** | A piece of code with a name, params and (possibly) multiple returns. | `main`, `run`, `Load` | JS function. |
| **Method** | A function with a receiver — behaves like an instance method. | `product.Validate()`, `repo.GetProducts(ctx)` | Class method. |
| **Return** | Goes back to the caller, optionally with values. | `return products, nil` | `return` / `yield`? Just `return`. |

## Errors, the Go way

| Term | Meaning | In this project | Frontend analogy |
| ---- | ------- | --------------- | ---------------- |
| **Error as a value** | A function that can fail returns an actual value of type `error`; you check it with `if err != nil`. No `throw`. | `(products, err) := ...; if err != nil { ... }` | `Promise.allSettled` / `try-catch` (but manual). |
| **`nil`** | "No value / nothing". Zero of pointers, slices, maps, interfaces, errors. | `return nil` means "no error". | `null`. |
| **Wrap** (`%w`) | `fmt.Errorf("...: %w", err)` embeds the original error while adding context. | Every layer adds its message keeping the cause. | Stack trace "cause chain". |
| **`errors.Is`** | Walks the whole wrap-chain to check if a specific error is inside. | `errors.Is(err, domain.ErrInvalidProduct)` → 400 vs 500. | `instanceof` over the cause chain. |
| **Sentinel error** | A fixed, reusable error used as comparison key. | `var ErrInvalidProduct = errors.New(...)` | A constant used as error code/flag. |
| **Zero value on failure** | On error, the value return is left as zero (`0`, `nil`) so the caller sees nothing bogus. | `return 0, err` on DB failure. | Returning `undefined` on failure. |

## Concurrency & process

| Term | Meaning | In this project | Frontend analogy |
| ---- | ------- | --------------- | ---------------- |
| **Goroutine** | A lightweight concurrent function; started with `go`. Costs little; thousands are fine. | `go func() { server.ListenAndServe() }()` | A "thread" (or an async task) — but it really runs in parallel. |
| **Channel** | A typed pipe between goroutines: send with `ch <- v`, receive with `v := <-ch`. | `<-ctx.Done()` blocks until the app is told to stop. | `await` on a promise / message port. |
| **Block** (verb) | A goroutine *stops* until something happens (e.g. a channel yields). It does not freeze the process. | `<-ctx.Done()` blocks `main` only; HTTP goroutine keeps running. | `await` (but only for that task). |
| **Defer** | Schedules a call to run when the function returns — even on error/panic. | `defer conn.Close()`, `defer rows.Close()`, `defer stop()` | `finally` block (but per-function, not per-try). |
| **Context** | Carries cancellation/timeout/deadline through a chain of calls; **first parameter** of I/O functions. | `ctx` goes handler → use case → repository → `QueryContext`. | Request-scoped `AbortSignal` passed everywhere. |
| **Cancel** | When a context is canceled, everything reading it stops; blocked `<-ctx.Done()` proceeds. | Ctrl+C cancels `ctx`; the DB query is aborted too. | `AbortController.abort()`. |
| **Deadline / Timeout** | A context with a time limit; after it, the work "times out". | `context.WithTimeout(..., 5*time.Second)` for ping and shutdown. | `AbortSignal.timeout(ms)`. |
| **Graceful shutdown** | Stop accepting new work, let in-flight work finish within a window, then exit. | `server.Shutdown(shutdownCtx)` with 5s. | Draining active requests during deploy. |
| **Signal** (OS) | The OS telling your process something (Ctrl+C = SIGINT; `kill`/docker stop = SIGTERM). | `signal.NotifyContext(... SIGINT, SIGTERM)` listens for them. | — (browser apps don't get signals). |

## Standard library pieces used here

| Term | Meaning | In this project |
| ---- | ------- | --------------- |
| **`net/http`** | Go's official HTTP server/client package. | Low-level server (`http.Server`), status constants (`http.StatusOK`). |
| **`database/sql`** | Go's official SQL abstraction over any DB driver. | Pool, `QueryContext`, `QueryRowContext`, `Scan`. |
| **`encoding/json`** | Serializes structs ↔ JSON using struct tags. | Called under the hood by gin's `ctx.JSON` / `ShouldBindJSON`. |
| **`fmt`** | Formatting/printing: `Printf`, `Errorf`, `Sprintf` with verbs (`%s %d %v %w`). | Error wrapping (`%w`), server log (`%s`). |
| **`log`** | Standard logger with timestamps. | `log.Println`, `log.Printf`, `log.Fatal`. |
| **`os`** | Access to environment, args, files. | `os.Getenv` in `config`. |
| **`os/signal` + `syscall`** | Capture OS signals. | `signal.NotifyContext` for graceful shutdown. |
| **`time`** | Durations, clocks, timeouts. | `10 * time.Second`, `5 * time.Minute`. |
| **`errors`** | Error helpers. | `errors.New`, `errors.Is`. |
| **`context`** | Cancellation/timeout propagation. | Seen in every layer. |
| **`fmt.Errorf`** | Creates an error, optionally wrapping `%w`. | `failed to open database: %w`. |

## The project's architecture vocabulary

| Term | Meaning | In this project | Frontend analogy |
| ---- | ------- | --------------- | ---------------- |
| **Layered / clean architecture** | Code organized in layers with one-directional dependencies (top→down). | `handler → usecase → repository → db`; `domain` imports nobody. | Feature/folder modularization + separation of concerns. |
| **Domain** | The core business entities and rules; pure, no frameworks. | `Product` + `Validate()`. | The `types`/entities your whole app shares. |
| **Use case** | An orchestrated business flow: validate → persist → respond. | `productUsecase.CreateProduct`. | A service/action function (e.g. a Redux thunk or service class). |
| **Repository** | The "storage abstraction": contract (interface) + concrete implementation. | `ProductRepository` interface + `postgresProductRepository`. | A repository/service module hiding the fetch layer behind an interface. |
| **Contract / interface** | A fixed set of method signatures a type must have. | `ProductRepository`, `ProductUsecase`. | A TS `interface` (though Go checks it structurally). |
| **Implementation** | The concrete code that fulfills a contract. | `postgresProductRepository`. | The class behind a TS interface. |
| **Handler / controller** | HTTP boundary: parse request, call use case, answer JSON. | `ProductHandler.GetProducts`. | An API route/service "controller" in a framework. |
| **Router** | The map "URL → handler method". | `router.New`: `GET /products` → `GetProducts`. | React Router `path → component` map. |
| **Bootstrap / composition root** | The single place that creates everything and wires it together. | `cmd/api/main.go` `run()`. | The root component / main entry that mounts the app and passes props. |
| **Dependency injection** | Giving a layer its dependencies from outside (constructor) instead of letting it create/import them. | `NewProductHandler(usecase)`, etc. | Passing props/hooks down instead of using globals. |
| **Factory** | A function whose job is to build and return a ready object. | `NewProductRepository`, `NewProductUsecase`. | A helper that constructs an object (e.g. a factory function). |
| **Contract-first** | Depend on the interface, not the concrete type. | Handler/usecase hold interfaces → easily mockable. | Design by TS interface + adapter. |

## Database & Postgres terms

| Term | Meaning | In this project |
| ---- | ------- | --------------- |
| **DSN** | "Data Source Name": the connection string with user, password, host, db. | `DATABASE_URL` env var. |
| **Driver** | The library that speaks the DB's wire protocol for `database/sql`. | `github.com/lib/pq` (registered via blank import). |
| **Connection pool** | A set of reused connections shared by all goroutines. | Configured with `SetMaxOpenConns` etc. in `db.Connect`. |
| **Ping** | A minimal "are you alive?" query to validate the connection. | `PingContext` right after `sql.Open`. |
| **Query / QueryRow** | Run SQL that returns many rows / a single row. | `QueryContext` (SELECT), `QueryRowContext` (INSERT…RETURNING). |
| **Rows** | The cursor over SELECT results; must be closed. | `rows.Next()`, `defer rows.Close()`, `rows.Err()`. |
| **Scan** | Copies the row's columns into Go variables/fields (by memory address). | `rows.Scan(&product.ID, ...)`. |
| **Parameterized SQL** | Values go in as placeholders (`$1, $2`), not concatenated — prevents SQL injection. | `VALUES ($1, $2) RETURNING id`. |
| **RETURNING** | Postgres feature: returns a value from the row you just inserted. | `RETURNING id` — the generated id comes straight back. |
| **Migration** | A versioned SQL file that evolves the schema. | `internal/db/migrations/000001_create_products.sql`. |
| **SERIAL** | Postgres auto-increment integer column (like AUTO_INCREMENT). | `id` column of `product` — the DB generates it. |

## HTTP & gin terms

| Term | Meaning | In this project | Frontend analogy |
| ---- | ------- | --------------- | ---------------- |
| **`gin.Engine`** | "The app" in gin: holds routes + middleware and answers HTTP. | Built in `router.New`. Implements `http.Handler`. | A server instance / app object in Express. |
| **Endpoint / route** | A URL+verb pair that triggers a handler. | `GET /products`, `POST /products`, `GET /health`. | An API route. |
| **`gin.Context`** | Bundles request + response + helpers for one HTTP call. | First param of every handler. | The `req`/`res` pair (or one ctx object). |
| **`gin.H`** | Shorthand for a JSON object sent to the client (`map[string]any`). | `gin.H{"error": "internal server error"}`. | An object literal. |
| **`ShouldBindJSON`** | Parses the JSON body into a Go struct, validating the format. | `ctx.ShouldBindJSON(&product)`. | `await res.json()` + validate. |
| **`ctx.JSON(status, body)`** | Serializes body to JSON and sends it with the status code. | `ctx.JSON(http.StatusCreated, created)`. | `res.status(201).json(data)`. |
| **Status code** | The HTTP status the response carries. | `200 OK`, `201 Created`, `400 Bad Request`, `500`. | Same (HTTP). |
| **Middleware** | Function that runs before/around the handler (logging, auth, recovery). | gin's Logger + Recovery (built into `gin.Default()`). | Express middleware / HOC wrapping a component. |
| **Health check** | A trivial endpoint that says "alive". | `GET /health → {"status":"healthy"}`. | Liveness probe. |

## Basic developer tools

| Term | Meaning | In this project |
| ---- | ------- | --------------- |
| **`go.mod` / `go.sum`** | Module manifest and dependency checksums (like `package.json` + lockfile). | Dependencies: gin, lib/pq. |
| **`go vet`** | Static analysis for suspicious code. | Run via `go vet ./...`. |
| **`gofmt`** | Officially enforced code formatter. | `gofmt -l .` checks formatting. |
| **`go build` / `go run`** | Compile the code / compile and run it. | `go run ./cmd/api`. |
| **`go doc`** | Shows a symbol's documentation. | `go doc net/http` etc. |
| **Struct tag** | Metadata string on a field, read via reflection. | `json:"name"` field tags. |
| **Reflection** | Runtime inspection of types/values found in the compiler. | Used by `encoding/json` and gin under the hood. |
| **Environment variable** | Config supplied from outside the code (OS level). | `SERVER_PORT`, `DATABASE_URL` in `config`. |