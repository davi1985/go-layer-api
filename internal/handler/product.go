// Package handler is the HTTP layer of the application (the equivalent of a
// "controller" in other languages).
//
// SINGLE responsibility: receive the request, avoid leaking server internals,
// and return JSON with the right status code.
//
// Frontend analogy: it is the component that receives the "click/event" (the
// request), calls the service (use case) and renders the "UI" — which here
// is the JSON response.
package handler

import (
	"errors"   // errors.Is: compare an error with the domain sentinel
	"log"      // Go's standard logger: traces each step of this layer
	"net/http" // HTTP status constants (200, 400, 500...)

	"github.com/davi1985/go-api/internal/domain"  // Product entity and sentinel error
	"github.com/davi1985/go-api/internal/usecase" // business contract (interface)
	"github.com/gin-gonic/gin"                    // HTTP framework
)

// ProductHandler holds the methods the router registers.
// It keeps the use case (the layer it calls). Exported (uppercase) because
// the router package needs to reference *handler.ProductHandler.
type ProductHandler struct {
	productUsecase usecase.ProductUsecase // depends on the INTERFACE, not the concrete struct
}

// NewProductHandler is the standard factory: it takes a use case and returns
// the handler. The dependency comes from outside (injection), never created
// in here.
func NewProductHandler(productUsecase usecase.ProductUsecase) *ProductHandler {
	return &ProductHandler{productUsecase: productUsecase}
}

// GetProducts is the handler for GET /products (list).
// Gin injects *gin.Context, which carries the request, response and helpers.
func (h *ProductHandler) GetProducts(ctx *gin.Context) {
	log.Printf("[handler] GetProducts: fetching products")

	// ctx.Request.Context() propagates the HTTP context to the layers below.
	// If the client disconnects midway, the whole job is canceled.
	products, err := h.productUsecase.GetProducts(ctx.Request.Context())
	if err != nil {
		// ctx.Error registers the error in gin's log (visible in the terminal).
		// We do NOT send err.Error() in the JSON: leaking internal details is a
		// security hole. The client gets a generic message and 500.
		log.Printf("[handler] GetProducts: usecase error -> 500: %v", err)
		ctx.Error(err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	log.Printf("[handler] GetProducts: sending %d product(s)", len(products))
	// 200 OK with the product list. Gin serializes the slice to JSON.
	ctx.JSON(http.StatusOK, products)
}

// CreateProduct is the handler for POST /products (create).
func (h *ProductHandler) CreateProduct(ctx *gin.Context) {
	var product domain.Product // declares an EMPTY Product ("zero value")

	// ShouldBindJSON parses the body and fills `product`'s fields.
	// If the JSON is malformed, we reply 400 right away, without touching the
	// use case.
	if err := ctx.ShouldBindJSON(&product); err != nil {
		log.Printf("[handler] CreateProduct: invalid JSON -> 400: %v", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	log.Printf("[handler] CreateProduct: received %+v", product)

	created, err := h.productUsecase.CreateProduct(ctx.Request.Context(), product)
	if err != nil {
		// errors.Is compares the error CHAIN with the sentinel:
		// if it was a BUSINESS VALIDATION problem -> the client is wrong, 400.
		if errors.Is(err, domain.ErrInvalidProduct) {
			log.Printf("[handler] CreateProduct: business rule violation -> 400: %v", err)
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		// Otherwise it is a real internal error: log the detail, return generic 500.
		log.Printf("[handler] CreateProduct: internal error -> 500: %v", err)
		ctx.Error(err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	log.Printf("[handler] CreateProduct: responding 201 with %+v", created)
	// Success: 201 Created + the full product (already with the DB-generated id).
	ctx.JSON(http.StatusCreated, created)
}
