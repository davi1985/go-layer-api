// Package usecase holds the BUSINESS RULES of the application.
//
// Frontend analogy: it is the "use case" — the sequence of steps that make
// sense for the business. E.g. "create product" = validate + save.
//
// The use case knows NO HTTP (gin) and NO database (SQL). It only
// ORCHESTRATES: receives data from the handler, applies rules, hands it to
// the repository, and returns the result.
//
// That is what allows swapping the HTTP framework or the database without
// touching this layer.
package usecase

import (
	"context" // context passed from the handler down to the repository
	"fmt"     // to wrap errors with %w (keep the original cause)
	"log"     // Go's standard logger: traces each step of this layer

	"github.com/davi1985/go-api/internal/domain"     // entities/validation
	"github.com/davi1985/go-api/internal/repository" // data contract (interface)
)

// ProductUsecase is the contract the handler sees. The handler depends on
// the interface, never on the concrete struct — so we can build a mock to
// test the handler without a real database.
//
// Notice the method names match the handler's: signatures barely change
// between layers; what changes is the type of data going in and out.
type ProductUsecase interface {
	// Lists products. Same signature as the repository (no extra rule yet).
	GetProducts(ctx context.Context) ([]domain.Product, error)
	// Creates a product. The return carries the full product, WITH the
	// generated id.
	CreateProduct(ctx context.Context, product domain.Product) (domain.Product, error)
}

// productUsecase is the concrete implementation (PRIVATE — lowercase).
// It keeps its dependency (the repository) as a struct field.
type productUsecase struct {
	repo repository.ProductRepository // dependency "injected" via the constructor
}

// NewProductUsecase is the factory: it takes any repository that satisfies
// the contract and returns a ready-to-use use case.
func NewProductUsecase(repo repository.ProductRepository) ProductUsecase {
	return &productUsecase{repo: repo}
}

// GetProducts forwards the call to the repository. Today there is no extra
// rule here, but this is exactly where one would go (e.g. cache or
// authorization).
func (u *productUsecase) GetProducts(ctx context.Context) ([]domain.Product, error) {
	log.Printf("[usecase] GetProducts: delegating to the repository")
	return u.repo.GetProducts(ctx)
}

// CreateProduct is the full "use case" of creating a product.
// `(u *productUsecase)` is the receiver; `u` is the use case instance.
func (u *productUsecase) CreateProduct(ctx context.Context, product domain.Product) (domain.Product, error) {
	// 1) VALIDATION — the business rule runs BEFORE touching the database.
	log.Printf("[usecase] CreateProduct: validating product")
	if err := product.Validate(); err != nil {
		// %w "embeds" the original error inside ErrInvalidProduct. Result:
		// errors.Is(err, domain.ErrInvalidProduct) works over in the handler
		// (like an error "instanceof" over the chain of causes).
		log.Printf("[usecase] CreateProduct: validation failed: %v", err)
		return domain.Product{}, fmt.Errorf("%w: %s", domain.ErrInvalidProduct, err.Error())
	}
	log.Printf("[usecase] CreateProduct: validation passed")

	// 2) PERSISTENCE — delegates to the repository. The database generates the id.
	id, err := u.repo.CreateProduct(ctx, product)
	if err != nil {
		// Real infrastructure error: returned for the handler to decide the status.
		log.Printf("[usecase] CreateProduct: repository error: %v", err)
		return domain.Product{}, err
	}

	// 3) ENRICHES the product with the generated id and returns it to the handler.
	log.Printf("[usecase] CreateProduct: database generated id=%d, enriching", id)
	product.ID = id
	return product, nil
}
