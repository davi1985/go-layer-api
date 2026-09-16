# Go fundamentals (frontend → Go)

These are the language pieces that show up in **every file** of this project.
Read them once, then come back whenever something confuses you. Go has no
classes, no exceptions, no async/await — but you already know how to code, so
think of this as a translation table.

## Packages and imports (≈ ES modules + export)

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

## Struct + method with receiver (≈ class/type + method)

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

## Pointers: `&` and `*` (≈ references, but explicit)

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

## Interfaces are STRUCTURAL (≈ TS interface, without `implements`)

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

## Errors are values, not exceptions (≈ no try/catch)

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
- Convention in this project: every layer logs its own steps with a `[layer]`
  prefix (`[handler]`, `[usecase]`, `[repository]`), so doing a request in
  Postman shows you the request descending the pyramid in the terminal. The
  rule that never changes: **the client JSON stays generic** — a 500 never
  leaks internal details, no matter how much we log locally.

## Multiple return values: the `(result, error)` duo

A function can declare several return types inside parentheses — basically a
**tuple**. The convention in Go (and in this project, everywhere) is
`(value, error)`:

```go
// declared in the repository:
func CreateProduct(ctx context.Context, product domain.Product) (int, error)
// calling it:
id, err := u.repo.CreateProduct(ctx, product)
```

- On success: `id` is useful, `err` is `nil`.
- On failure: `id` is its **zero value** (`0`), `err` describes the problem.
- You must capture **both**. If you only want one, discard the other with `_`
  (see "Blank identifier" further below).

Analogy: like `Promise.allSettled([value, error])` — the error travels
alongside the value, never as an exception.

## `context.Context` — the request-lifetime "extension cord"

`context` carries state down the chain: cancellation (client disconnected,
timeout, Ctrl+C). **Convention:** it is always the **first parameter** of
functions that do I/O (database, HTTP).

```go
func GetProducts(ctx context.Context) (...) // handler passes ctx.Request.Context()
// -> use case forwards it
// -> repository passes it to db.QueryContext(ctx, query)
// the driver aborts the query if ctx is canceled
```

## `defer` — "run when the function finishes" (≈ finally)

```go
rows, _ := r.db.QueryContext(ctx, query)
defer rows.Close() // runs at the end of the function, even on return/panic
```

## Goroutines (≈ light threads, no async/await)

In `cmd/api/main.go` the server's `ListenAndServe()` **blocks**. We run it in
parallel with `go func() { ... }()` while the main flow waits for the
shutdown signal.

## Channels: what `<-ctx.Done()` does (the shutdown gate)

A **channel** is a typed pipe between goroutines. `<-` before a channel means
"receive from it":

```go
<-ctx.Done() // blocks this goroutine until the context is finished
```

- `ctx.Done()` returns a channel that the context **closes** when it is
  canceled (Ctrl+C, SIGTERM, client disconnect).
- Receiving from a closed channel returns immediately — so this one line
  *blocks the main flow* until shutdown is requested, then lets it continue.
- Important: blocking only suspends **this** goroutine. The HTTP server
  goroutine keeps running in parallel — the app keeps answering requests while
  main waits.

Analogy: `await` on a long-lived promise — "sleep until someone pulls the plug"
— except Go's block does not freeze the rest of the process.

## Slices: `[]T`, `make`, `append`, and the `nil` trap

```go
var s []string         // nil slice -> becomes `null` in JSON
s = make([]string, 0)  // empty slice -> becomes `[]` in JSON ✅
s = append(s, "a")     // push
```

That is why the repository uses `products := make([]domain.Product, 0)`.

## Zero value (≈ undefined, but always defined)

Every type has an automatic "zero value": `int` → `0`, `string` → `""`,
`*T` → `nil`, `struct` → everything zero. That is why
`var product domain.Product` is already a valid product (with `""`, `0`, `0`).

## Struct tags: how a struct maps to JSON (≈ serializer config)

```go
type Product struct {
	ID    int     `json:"id"`    // this field is "id" in JSON
	Name  string  `json:"name"`  // uppercase Go name, lowercase JSON key
	Price float64 `json:"price"`
}
```

- The backtick string after the type is a **tag**: a mini-configuration
  attached to the field. `encoding/json` reads it when encoding/decoding.
- No tag → the JSON key would be the Go name (`"ID"`, `"Name"`, `"Price"`).
  The tags pin the wire format.
- Rule to remember: a field **must be exported** (uppercase Go name) to be
  marshaled at all; the tag only controls the JSON key name.
- Under the hood, gin's `ShouldBindJSON` and `ctx.JSON` build on Go's
  reflection + `encoding/json`. Analogy: the field mapping/serverless
  serializer config that describes your API contract in `interface Product`.

## The blank identifier `_` and blank imports ("ignore me")

`_` is a valid name you can use **anywhere a value is required** to say "I
don't need it".

```go
_ "github.com/lib/pq"                       // blank IMPORT: register, don't use
product, _ := u.repo.GetProducts(ctx)       // discard a return you ignore
```

Two places in this project:

1. **Ignoring a return value** — a multi-value function where you only care
   about some of the results.
2. **Blank import** (`db/conn.go`) — importing a package *only for its side
   effects*. `lib/pq` registers a driver with `database/sql`; we never call it
   by name. Go forbids importing a package you never reference, so `_` is the
   escape hatch. Read more in the architecture doc's repository section.

## `const` + backtick strings: fixed SQL, written literally

```go
const query = `INSERT INTO product (product_name, price) VALUES ($1, $2) RETURNING id`
```

- `const` = immutable (like `const` in JS).
- Backticks are Go's **raw string**: everything between them is literal — no
  escape sequences, newlines allowed. Perfect for multi-line SQL. (Like a
  template literal *without* interpolation.)
- `$1, $2` are not Go — they are Postgres parameter placeholders the driver
  fills in at runtime. Never concatenate values into the query string (SQL
  injection) — see the repository section in `architecture.md`.

## `if` with an init statement: declare + check in one line

```go
if value := os.Getenv(key); value != "" {
	return value
}
```

The small statement before `;` runs first, then the condition is checked. The
scope of `value` is **only that `if`** (and its optional `else`), so helpers
don't leak into the rest of the function. Analogy: block-scoped `let`.

## `fmt` verbs: the formatting placeholders (≈ template literals)

`fmt` is Go's formatting package. `Errorf`, `Printf`, `Sprintf` take a format
string with **verbs** that substitute the arguments:

| Verb | Means                                     | Example output    |
| ---- | ----------------------------------------- | ----------------- |
| `%s` | a string                                  | `"Keyboard"`      |
| `%d` | an integer                                | `42`              |
| `%v` | default format for *anything* ("value")   | `[Keyboard 42]`   |
| `%w` | **wrap** an error (only valid in `Errorf`) | (goes into the error chain) |

```go
fmt.Errorf("failed to open database: %w", err)                    // keeps original error
fmt.Errorf("%w: %s", domain.ErrInvalidProduct, err.Error())       // sentinel + detail
log.Printf("server listening on %s", server.Addr)                 // plugs a value in
```

`%w` is the one that matters for error handling: it **wraps** the original
error so `errors.Is` can still find it downstream.

## Composite literals: build a struct in one go (≈ typed object literal)

```go
server := &http.Server{
	Addr:         ":" + cfg.ServerPort,
	Handler:      router.New(productHandler),
	ReadTimeout:  10 * time.Second,
	WriteTimeout: 10 * time.Second,
}
```

`TypeName{ field: value, ... }` creates the instance inline. The compiler
checks the field names — no typos, no arbitrary keys. The leading `&` takes
the address of the freshly built value. The factories do the same, smaller:
`return &postgresProductRepository{db: db}`.

## The `log` package: Go's `console.*`

| Go                             | Analogy in JS                        |
| ------------------------------ | ------------------------------------ |
| `log.Println("...")`           | `console.log` (with newline)         |
| `log.Printf("... %s", x)`      | `console.log` + template substitution |
| `log.Fatal(err)`               | `console.error(err)` + `process.exit(1)` |

Every line gets a timestamp automatically. `log.Fatal` prints **and ends the
process** (exit code 1) — used in `main` for boot-time failures that leave no
recoverable app.

## Closures / inline functions (≈ arrow functions)

A function value can be written **inline** and passed around:

```go
go func() { ... }()                                       // "go" + () at the end
router.GET("/health", func(ctx *gin.Context) { ... })     // passed as an argument
```

- The `func(...) { ... }` is a **closure**: it can read/write variables from the
  surrounding scope. `router.GET` calls it later, when a request hits `/health`.
- No `function` keyword, no `=>` — just `func`, the parameters, and the body.
  Analogy: an arrow function handed to `.then()` or to a component prop.
- Note the trailing `()` after `go func(){}()`: it **defines and immediately
  invokes** the function inside a new goroutine.

## `time.Duration`: time as a number with units

```go
ReadTimeout: 10 * time.Second
SetConnMaxLifetime(5 * time.Minute)
```

`time.Duration` is just an `int64` count of **nanoseconds**, but you write it
with readable units: `10 * time.Second` literally reads "ten seconds". It is
plain arithmetic: multiply the unit by how many you want. Analogy: `ms` in JS,
but self-documenting and impossible to mistake for some other unit.

## Cheat sheet: concept → where you saw it → where it's explained

| Go concept                       | Where it appears in the repo                          | Section above                          |
| -------------------------------- | ----------------------------------------------------- | -------------------------------------- |
| packages / imports / export      | every `.go` file                                      | Packages and imports                   |
| struct + receiver method         | `domain/product.go`, `handler/product.go`             | Struct + method with receiver          |
| pointers `&` / `*`               | repositories, handler, `main`                         | Pointers: `&` and `*`                  |
| structural interfaces            | `repository/product.go`, `usecase/product.go`         | Interfaces are STRUCTURAL              |
| multiple returns `(T, error)`    | repositories, use case, `db/conn.go`                  | Multiple return values                 |
| errors: `%w`, `errors.Is`, `nil` | `main`, `db`, repositories, handler, use case         | Errors are values                      |
| `context.Context`                | every layer (first parameter)                         | `context.Context`                      |
| `defer`                          | `main`, `db/conn.go`, `product_postgres.go`           | `defer`                                |
| goroutines `go func(){}()`       | `main.go`                                             | Goroutines                             |
| channels `<-ctx.Done()`          | `main.go`                                             | Channels                               |
| slices `make`/`append`/nil trap  | `product_postgres.go`                                 | Slices                                 |
| zero value                       | `handler/product.go`, `usecase`, repositories         | Zero value                             |
| struct tags `json:"..."`         | `domain/product.go`                                   | Struct tags                            |
| blank `_` + blank import         | `db/conn.go`, `product_postgres.go`                   | Blank identifier                       |
| `const` + backtick raw strings   | `product_postgres.go`                                 | `const` + backtick strings             |
| `if` with init statement         | `config/config.go`                                    | `if` with an init statement            |
| `fmt` verbs `%s %d %v %w`        | everywhere errors/logs are formatted                  | `fmt` verbs                            |
| composite literals `Type{...}`   | `main.go`, repository factory                         | Composite literals                     |
| `log` package                    | `main.go`                                             | `log` package                          |
| closures / inline functions      | `main.go`, `router/router.go`                         | Closures / inline functions            |
| `time.Duration`                  | `main.go`, `db/conn.go`                               | `time.Duration`                        |