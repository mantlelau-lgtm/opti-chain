package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"scm/internal/pkg/authx"
	"scm/internal/pkg/response"
)

// Auth enforces a valid bearer token on protected routes. The parsed Actor is
// stored in the context for downstream handlers. When enabled is false (dev
// escape hatch) requests pass through anonymously.
func Auth(enabled bool, parse func(token string) (*authx.Actor, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !enabled {
			c.Next()
			return
		}
		header := c.GetHeader("Authorization")
		token, ok := strings.CutPrefix(header, "Bearer ")
		if !ok || token == "" {
			unauthorized(c)
			return
		}
		actor, err := parse(token)
		if err != nil {
			unauthorized(c)
			return
		}
		authx.SetActor(c, actor)
		c.Next()
	}
}

func unauthorized(c *gin.Context) {
	response.HTTPFail(c, http.StatusUnauthorized, response.ErrUnauthorized, "authentication required")
	c.Abort()
}
