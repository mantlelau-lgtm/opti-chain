// Package authz is the authorisation module. It is the single place that
// answers "can this actor call this tool?" — for both API handlers and the
// agent assistant. It depends on nothing except the Actor type; the RBAC
// engine is injected as an interface.
package authz

import "scm/pkg/authx"

// PermChecker is the permission engine. The real implementation is
// service.RBACService.HasPerm, which handles both JWT role-based actors
// and AK/SK key-based actors (KeyAuth/KeyPerms).
type PermChecker interface {
	HasPerm(actor *authx.Actor, perm string) bool
}

// Service is the authorisation gate. Every tool call — whether from an API
// handler or an LLM agent — passes through Can.
type Service struct {
	rbac PermChecker
}

// New creates an authz Service backed by the RBAC engine.
func New(rbac PermChecker) *Service {
	return &Service{rbac: rbac}
}

// Can reports whether the actor is permitted to call the tool identified by
// the permission code (e.g. "material:manage", "po:create").
//
//	actor — the caller (JWT user or AK/SK key)
//	perm  — the permission code the tool requires
func (s *Service) Can(actor *authx.Actor, perm string) bool {
	if actor == nil || perm == "" {
		return false
	}
	return s.rbac.HasPerm(actor, perm)
}
