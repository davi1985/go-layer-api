// Package db deals EXCLUSIVELY with the database connection.
//
// Frontend analogy: it is the file that creates your "API client/fetch" —
// here the client is a *sql.DB pointing at Postgres.
package db

import (
	"context"      // to put a timeout on the ping
	"database/sql" // Go's official package that abstracts relational databases
	"fmt"          // to format errors while keeping the original cause with %w
	"log"          // Go's standard logger (prints to the terminal)
	"time"         // durations (pool settings and the ping timeout)

	_ "github.com/lib/pq" // the Postgres driver. The "_" registers the driver without us using it directly here
)

// Connect opens the connection, configures the pool, and confirms the
// database responds.
//
// IMPORTANT: sql.Open does NOT actually connect; it only creates the handle
// and registers the driver. What validates the connection is the
// PingContext call at the end of this function.
//
// It returns two values: the ready *sql.DB, or an error. Whoever calls it
// (main) is responsible for closing the connection with defer conn.Close().
func Connect(ctx context.Context, dsn string) (*sql.DB, error) {
	// sql.Open takes the "driver name" ("postgres") and the connection
	// string. The first variable `conn` is the handle; err only comes if the
	// driver name is invalid.
	conn, err := sql.Open("postgres", dsn)
	if err != nil {
		// %w "wraps" the original error inside a new message. This PRESERVES
		// the chain of causes, allowing errors.Is/errors.As downstream.
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// =====================================================================
	// POOL CONFIGURATION (important for concurrency)
	// =====================================================================
	// The http.Server handles many requests in goroutines at the same time.
	// Without an organized pool, the app could open hundreds of connections
	// on Postgres and bring it down.
	conn.SetMaxOpenConns(10)                 // at most 10 connections OPEN at once
	conn.SetMaxIdleConns(5)                  // keeps up to 5 idle connections "in reserve"
	conn.SetConnMaxLifetime(5 * time.Minute) // recycles connections after 5 min (avoids stale/dead connections)

	// Creates a context limited to 5s just for the ping.
	// If the database does not answer in 5s, the ping fails instead of
	// hanging forever.
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	// cancel() releases the context resources as soon as the function ends.
	defer cancel()

	// PingContext makes a REAL request to the database. Only here do we know
	// if Postgres is alive and the password/database are correct.
	if err := conn.PingContext(ctx); err != nil {
		conn.Close() // ping failed: close the handle so no resource leaks
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Startup confirmation: the REAL pool state via conn.Stats() (open and
	// idle connections at this moment). The DSN itself is deliberately NOT
	// logged — it contains the password.
	log.Printf("database connected (open_conns=%d idle_conns=%d)", conn.Stats().OpenConnections, conn.Stats().Idle)

	return conn, nil // all good: returns the healthy pool
}
