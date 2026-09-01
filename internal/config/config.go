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

// Auth configures JWT authentication.
type Auth struct {
	Enabled   bool          // SCM_AUTH=off disables auth (dev only)
	JWTSecret string        // signing key; the default is dev-only
	TokenTTL  time.Duration // access-token lifetime
}

// LLM configures the local LLM gateway (OpenAI-compatible) that powers the
// in-app assistant's semantic analysis.
type LLM struct {
	URL   string // base URL, e.g. http://127.0.0.1:8080
	Model string // model id as known to the gateway
	Key   string // optional bearer key for the gateway
}

// Config is the top-level application configuration.
type Config struct {
	DB     Database
	Server Server
	Auth   Auth
	LLM    LLM
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

	ttlHours := getEnv("SCM_JWT_TTL", "24")
	ttl, err := time.ParseDuration(ttlHours + "h")
	if err != nil || ttl <= 0 {
		ttl = 24 * time.Hour
	}

	return &Config{
		DB: Database{Driver: drv, DSN: dsn},
		Server: Server{
			Addr:       getEnv("SCM_ADDR", ":8088"),
			CORSOrigin: getEnv("SCM_CORS_ORIGIN", "*"),
		},
		Auth: Auth{
			Enabled:   getEnv("SCM_AUTH", "on") != "off",
			JWTSecret: getEnv("SCM_JWT_SECRET", "scm-dev-secret-change-me"),
			TokenTTL:  ttl,
		},
		LLM: LLM{
			URL:   getEnv("SCM_LLM_URL", "http://127.0.0.1:8080"),
			Model: getEnv("SCM_LLM_MODEL", "qwen3-coder-flash"),
			Key:   getEnv("SCM_LLM_KEY", ""),
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
