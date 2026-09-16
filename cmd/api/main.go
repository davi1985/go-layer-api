// Package main is the program entry point.
//
// In Go, an executable always starts in the main() function of a package
// named main. This file has three responsibilities:
//
//  1. Load the configuration (server port, database connection)
//  2. Wire the application layers together (dependency injection)
//  3. Start the HTTP server with graceful shutdown
//
// Frontend analogy: it is the index.js/main.tsx that mounts your app tree.
package main

import (
	"context"   // carries the cancellable context that represents the process lifetime
	"errors"    // errors.Is to tell a real error apart from a manual shutdown
	"log"       // Go's standard logger (prints to the terminal)
	"net/http"  // Go's official package with the low-level HTTP server
	"os/signal" // captures OS signals (Ctrl+C, kill)
	"syscall"   // signal constants (SIGINT, SIGTERM)
	"time"      // durations, used for the server timeouts

	"github.com/davi1985/go-api/internal/config"     // configuration layer (environment variables)
	"github.com/davi1985/go-api/internal/db"         // database layer (opens the Postgres pool)
	"github.com/davi1985/go-api/internal/handler"    // HTTP layer (receives and answers requests)
	"github.com/davi1985/go-api/internal/repository" // data access layer (contract + SQL)
	"github.com/davi1985/go-api/internal/router"     // layer that defines the API routes
	"github.com/davi1985/go-api/internal/usecase"    // business rules layer
)

// main takes no arguments and returns nothing; the Go runtime calls it.
// Any fatal error here terminates the whole process.
func main() {
	// Reads the environment variables and returns a filled *Config.
	cfg := config.Load()

	// We keep almost all logic in run() because main cannot return an error.
	// If run() returns something other than nil, we decide what to do here.
	if err := run(cfg); err != nil {
		// log.Fatal prints the error and exits the process with code 1.
		log.Fatal(err)
	}
}

// run actually assembles the application. Returning an error lets main
// simply decide how to react to it.
//
// Separating main from run gives you something in return: you can test
// run() with a fake config without starting the binary. Good Go practice.
func run(cfg *config.Config) error {
	// log.SetFlags configures Go's global logger.
	//  - LstdFlags:  date + time on every line
	//  - Lshortfile: add "file.go:line" so you can see WHERE the log came from
	// When you are learning and testing manually (Postman), being able to
	// trace a log line back to the code is pure gold.
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Creates a context that is CANCELED automatically when the program
	// receives SIGINT (Ctrl+C) or SIGTERM (used by `docker stop`, `kill`...).
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	// `defer` schedules an action to run at the END of the function (even on
	// error). Here we guarantee the signal handler registration is released
	// when run() finishes.
	defer stop()

	// Opens the connection to Postgres. DSN = connection string (DB URL).
	conn, err := db.Connect(ctx, cfg.DBSource)
	if err != nil {
		return err
	}
	// `defer conn.Close()` also runs at the end, closing the connection pool.
	defer conn.Close()

	// =====================================================================
	// MANUAL DEPENDENCY INJECTION
	// =====================================================================
	// We build the "pyramid" bottom-up. Each layer receives, in its
	// constructor (NewX), the dependency from the layer below:
	//
	//     repository (SQL) -> usecase (business rules) -> handler (HTTP)
	//
	// Frontend analogy: passing props between components instead of letting
	// each one import whatever it wants from anywhere (globals).
	productRepository := repository.NewProductRepository(conn)
	productUsecase := usecase.NewProductUsecase(productRepository)
	productHandler := handler.NewProductHandler(productUsecase)

	// Creates Go's official HTTP server. Gin is not the server: gin produces
	// a *gin.Engine, which implements Go's http.Handler interface. We pass
	// that engine here, so we get timeouts for free.
	server := &http.Server{
		Addr:         ":" + cfg.ServerPort,       // address and port it listens on (e.g. ":3000")
		Handler:      router.New(productHandler), // gin routes that serve the requests
		ReadTimeout:  10 * time.Second,           // max time to READ the request body
		WriteTimeout: 10 * time.Second,           // max time to WRITE the response
		IdleTimeout:  60 * time.Second,           // lifetime of an idle (keep-alive) connection
	}

	// `go func(){}` starts a goroutine (a lightweight Go thread) that runs in
	// parallel. It is needed because ListenAndServe() BLOCKS: it keeps
	// accepting requests forever. With the goroutine, the main flow continues
	// and waits for the shutdown signal on `<-ctx.Done()`.
	go func() {
		log.Printf("server listening on %s", server.Addr)
		// If the server stops with a REAL error (i.e. not the ErrServerClosed
		// raised by a manual shutdown), we log it and call stop() to cancel
		// the context, which terminates the app.
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("server error: %v", err)
			stop() // triggers the ctx cancellation, "waking up" the <-ctx.Done()
		}
	}()

	// <-ctx.Done() BLOCKS the function until the context is canceled
	// (Ctrl+C, SIGTERM, or the stop() called from the goroutine above).
	<-ctx.Done()
	log.Println("shutting down server...")

	// Creates a context with a 5s deadline for the shutdown.
	// In-flight requests get that time to finish.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Shutdown is the "graceful shutdown": it stops accepting new connections
	// and waits for the active ones to finish within the timeout.
	return server.Shutdown(shutdownCtx)
}
