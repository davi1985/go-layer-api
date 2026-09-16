// Package config holds ALL the application configuration.
//
// Instead of hardcoded values scattered around the code (host, password,
// port), we read everything from environment variables, with fallbacks
// (defaults) for local development.
//
// Frontend analogy: it is your `.env` file turned into a typed object, with
// the `.env.example` baked into the code.
package config

import (
	"os" // official package for reading environment variables and args
)

// Config is the "configuration object": data only, no behavior.
// Fields starting with an UPPERCASE letter are EXPORTED (public) — other
// packages can read cfg.ServerPort, for example.
type Config struct {
	ServerPort string // port the HTTP server listens on (e.g. "3000")
	DBSource   string // Postgres connection string (URL with driver, user, password, database)
}

// Load reads the environment variables and returns a filled *Config.
// It is a kind of "factory": whoever calls it gets a ready-made config.
func Load() *Config {
	return &Config{
		// If SERVER_PORT is not set, use "3000".
		ServerPort: getEnv("SERVER_PORT", "3000"),
		// If DATABASE_URL is not set, use the local dev URL (docker-compose defaults).
		DBSource: getEnv("DATABASE_URL", "postgres://root:root@localhost:5432/postgres?sslmode=disable"),
	}
}

// getEnv is a helper: if the environment variable exists (non-empty), use
// its value; otherwise use the fallback. This lets the app run locally
// without configuring anything.
func getEnv(key, fallback string) string {
	// A Go idiom: `if` with an init statement. `value` exists only inside the if.
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
