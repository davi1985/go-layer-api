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
- Convention: log in the **handler**, never expose details in the client JSON.

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