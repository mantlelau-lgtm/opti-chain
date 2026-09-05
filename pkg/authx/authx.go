// Package authx carries the authenticated identity across layers without
// coupling middleware to service.
package authx

import "github.com/gin-gonic/gin"

// Actor is the authenticated principal resolved from the bearer token or an
// AK/SK signature. TenantID scopes every data access; Roles drive JWT
// permission checks, while KeyAuth/KeyPerms drive AK/SK permission checks.
type Actor struct {
	UserID   uint
	Username string
	Name     string
	TenantID uint
	Roles    []string

	// KeyAuth marks an AK/SK actor; KeyPerms is the key's granted permission
	// set. An empty slice means "all permissions".
	KeyAuth  bool
	KeyPerms []string
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
