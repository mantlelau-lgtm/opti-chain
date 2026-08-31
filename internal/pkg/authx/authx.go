// Package authx carries the authenticated identity across layers without
// coupling middleware to service.
package authx

import "github.com/gin-gonic/gin"

// Actor is the authenticated principal resolved from the bearer token.
// TenantID scopes every data access; Roles drive permission checks.
type Actor struct {
	UserID   uint
	Username string
	Name     string
	TenantID uint
	Roles    []string
}

// ContextKey stores the Actor in gin.Context.
const ContextKey = "actor"

// SetActor stores the actor for downstream handlers.
func SetActor(c *gin.Context, a *Actor) { c.Set(ContextKey, a) }

// GetActor returns the actor, or nil when auth is disabled / absent.
func GetActor(c *gin.Context) *Actor {
	if a, ok := c.Get(ContextKey); ok {
		if actor, ok := a.(*Actor); ok {
			return actor
		}
	}
	return nil
}
