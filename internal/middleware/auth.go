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

// RequirePerm enforces the DB-catalogued permission for the matched route.
// Routes without a binding only require authentication.
func RequirePerm(enabled bool, check func(*authx.Actor, string, string) error) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !enabled {
			c.Next()
			return
		}
		a := authx.GetActor(c)
		if a == nil {
			unauthorized(c)
			return
		}
		if err := check(a, c.Request.Method, c.Request.URL.Path); err != nil {
			response.HTTPFail(c, http.StatusForbidden, response.ErrForbidden, err.Error())
			c.Abort()
			return
		}
		c.Next()
	}
}
