package service

import (
	"testing"

	"scm/internal/model"
)

// TestAssistantAgents ensures every fixed role has exactly one agent with a
// system prompt, and that the tool registry is complete and well-formed
// (every tool has a name, permission and description so permission enforcement
// and the LLM's tool selection cannot silently break).
func TestAssistantAgentsAndTools(t *testing.T) {
	tools := registerAssistantTools(AssistantDeps{})
	if len(tools) == 0 {
		t.Fatal("tool registry is empty")
	}
	seenTool := map[string]bool{}
	for _, tool := range tools {
		if tool.Name == "" || tool.Perm == "" || tool.Description == "" {
			t.Fatalf("tool missing metadata: %+v", tool)
		}
		if tool.Exec == nil {
			t.Fatalf("tool %s has nil Exec", tool.Name)
		}
		if seenTool[tool.Name] {
			t.Fatalf("duplicate tool %s", tool.Name)
		}
		seenTool[tool.Name] = true
	}

	roles := []string{
		model.RoleAdmin, model.RoleProcSpec, model.RoleProcMgr,
		model.RolePlanSpec, model.RolePlanSup, model.RoleQC, model.RoleWhMgr,
	}
	if len(agentMetas) != len(roles) {
		t.Fatalf("expected %d agents, got %d", len(roles), len(agentMetas))
	}
	seenAgent := map[string]bool{}
	for _, a := range agentMetas {
		if a.Name == "" || a.System == "" || a.Description == "" {
			t.Fatalf("agent %s missing prompt/description", a.Role)
		}
		if seenAgent[a.Role] {
			t.Fatalf("duplicate agent %s", a.Role)
		}
		seenAgent[a.Role] = true
	}
	for _, r := range roles {
		if !seenAgent[r] {
			t.Fatalf("missing agent for role %s", r)
		}
	}
}
