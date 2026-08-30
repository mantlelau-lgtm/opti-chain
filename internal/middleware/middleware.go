// Package middleware provides reusable HTTP middleware (CORS, recovery).
package middleware

import (
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// CORS returns a CORS middleware suitable for the dev SPA. The origin is
// configurable so production can tighten it.
func CORS(origin string) gin.HandlerFunc {
	cfg := cors.DefaultConfig()
	if origin != "" && origin != "*" {
		cfg.AllowOrigins = []string{origin}
	} else {
		cfg.AllowAllOrigins = true
	}
	cfg.AllowMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
	cfg.AllowHeaders = []string{"Origin", "Content-Type", "Authorization"}
	cfg.MaxAge = 12 * time.Hour
	return cors.New(cfg)
}

// Recovery converts a recovered panic into a 500 envelope instead of crashing.
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"code":    50000,
					"message": "internal error: " + toStr(r),
					"data":    nil,
				})
				c.Abort()
			}
		}()
		c.Next()
	}
}

func toStr(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return "unknown panic"
}
