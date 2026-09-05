package service

import (
	"fmt"

	"scm/pkg/authx"
	"scm/pkg/authz"
)

// Tool is a callable operation exposed by the tool service layer. API
// handlers and agent assistants both consume the same tool definitions.
type Tool struct {
	Name        string
	Description string
	Perm        string         // permission code required to call this tool
	Schema      map[string]any // JSON Schema describing the tool's parameters
	Exec        func(actor *authx.Actor, args map[string]any) (any, error)
}

// Registry is the tool service layer. All tools are registered at startup
// with their executors (closures capturing the service dependencies).
// ListByRoles filters by role permissions; Execute checks authz before
// running the tool.
type Registry struct {
	tools    map[string]*Tool
	order    []string
	rbac     authz.PermChecker
	authzSvc *authz.Service
}

// NewRegistry creates an empty tool registry.
func NewRegistry(rbac authz.PermChecker, authzSvc *authz.Service) *Registry {
	return &Registry{
		tools:    make(map[string]*Tool),
		rbac:     rbac,
		authzSvc: authzSvc,
	}
}

// Register adds a tool. Name must be unique; duplicate names panic.
func (r *Registry) Register(t *Tool) {
	if _, ok := r.tools[t.Name]; ok {
		panic(fmt.Sprintf("tool %q registered twice", t.Name))
	}
	r.tools[t.Name] = t
	r.order = append(r.order, t.Name)
}

// List returns every registered tool in registration order.
func (r *Registry) List() []*Tool {
	out := make([]*Tool, 0, len(r.order))
	for _, name := range r.order {
		out = append(out, r.tools[name])
	}
	return out
}

// ListByRoles returns tools the given roles are permitted to call. The
// filter is per-role: a tool is included if ANY of the roles holds the
// permission code the tool requires.
func (r *Registry) ListByRoles(roles []string) []*Tool {
	if len(roles) == 0 {
		return nil
	}
	roleActor := &authx.Actor{Roles: roles}
	var out []*Tool
	for _, name := range r.order {
		t := r.tools[name]
		if r.rbac.HasPerm(roleActor, t.Perm) {
			out = append(out, t)
		}
	}
	return out
}

// Execute invokes a tool by name, passing the actor identity and the
// arguments. Authz is checked before execution: the actor must hold the
// permission the tool requires.
func (r *Registry) Execute(name string, actor *authx.Actor, args map[string]any) (any, error) {
	t, ok := r.tools[name]
	if !ok {
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
	if !r.authzSvc.Can(actor, t.Perm) {
		return nil, fmt.Errorf("forbidden: %s requires %s", name, t.Perm)
	}
	return t.Exec(actor, args)
}
