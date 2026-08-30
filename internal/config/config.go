// Package config loads runtime configuration.
//
// The system targets two databases (SQLite for dev, MySQL for prod).
// The active dialect is chosen via the SCMDB_DRIVER env var so that the same
// schema / code runs on either without any DDL changes.
package config

import (
	"os"
	"time"
)

// Database configures the underlying data source.
type Database struct {
	Driver string // "sqlite" or "mysql"
	DSN    string // sqlite path, or mysql dsn
}

// Server holds HTTP server settings.
type Server struct {
	Addr       string
	CORSOrigin string
}

// Config is the top-level application configuration.
type Config struct {
	DB     Database
	Server Server
}

// Load reads configuration from environment variables, with sane defaults
// that let the app run out-of-the-box on SQLite.
func Load() *Config {
	drv := getEnv("SCMDB_DRIVER", "sqlite")
	var dsn string
	switch drv {
	case "mysql":
		dsn = getEnv("SCMDB_DSN", "root:root@tcp(127.0.0.1:3306)/scm?charset=utf8mb4&parseTime=true&loc=Local")
	default:
		drv = "sqlite"
		dsn = getEnv("SCMDB_DSN", "scm.db")
	}

	return &Config{
		DB: Database{Driver: drv, DSN: dsn},
		Server: Server{
			Addr:       getEnv("SCM_ADDR", ":8088"),
			CORSOrigin: getEnv("SCM_CORS_ORIGIN", "*"),
		},
	}
}

func getEnv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

// PingDuration returns a short timeout used for connectivity checks.
func PingDuration() time.Duration { return 5 * time.Second }
