# Replication guide: build the same structure elsewhere

Two paths: **starting from scratch** and **adding a new resource** to an
existing project. Both use the same checklist.

## Start a new project from scratch

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

## Add a new resource (e.g. "Category")

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

## Test the use case without a database (why the interfaces exist)

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

## Golden rules and common pitfalls

1. **Dependencies point inward** — `handler → usecase → repository → db`,
   and `domain` imports no framework.
2. **`context` as the first parameter** in any I/O function, always.
3. **Errors**: propagate with `%w`, return a generic message to the client
   (never raw `err.Error()` on a 500). While learning, log the flow at each
   layer with `[layer]` prefixes so you can trace requests in the terminal.
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

## Possible next steps

- Pagination in `GET /products` (e.g. `?limit&offset`).
- A real migration tool (e.g. `golang-migrate`, `atlas`, `goose`) — today the
  migrations are SQL files applied by hand.
- `GET /products/:id`, `PUT`, `DELETE`.
- Middlewares (CORS, JWT auth) in `router`.
- Unit tests for the use case with a stub (see
  [Test the use case without a database](#test-the-use-case-without-a-database-why-the-interfaces-exist))
  and handler tests with `httptest`.