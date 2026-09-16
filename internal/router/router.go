// Package router centralizes ALL the application routes: "which URL calls
// which method of which handler". It is gin's "plumbing".
//
// Keeping it in its own package (instead of in main) has two benefits: main
// stays lean, and when the API grows, this is the only place that needs to
// know all the routes.
package router

import (
	"net/http" // status code for the health check

	"github.com/davi1985/go-api/internal/handler" // handlers the routes call
	"github.com/gin-gonic/gin"                    // HTTP framework
)

// New builds the *gin.Engine — gin's "app" — with all routes registered.
// It takes the handlers it needs (today, only product) and returns the
// engine ready to be used as the http.Handler in the server
// (http.Server.Handler).
//
// Frontend analogy: it is the routes file (e.g. React Router) where you map
// "path -> component".
func New(productHandler *handler.ProductHandler) *gin.Engine {
	// gin.Default() already ships two ready-made middlewares:
	//   - Logger:  logs each request to the terminal
	//   - Recovery: if a handler PANICS, returns 500 instead of crashing the
	//     process
	router := gin.Default()

	// Health check: a simple "I am alive" route. We use an inline function
	// (closure) because it needs no business layer at all.
	router.GET("/health", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	// Each route points to a method of the handler.
	router.GET("/products", productHandler.GetProducts)    // list all
	router.POST("/products", productHandler.CreateProduct) // create one

	return router // the engine, ready for the server
}
