// Package memory is the independent two-layer memory module for the
// assistant. Short-term memory (conversation turns) provides context
// continuity; long-term memory (knowledge graph + user profile) is built
// from short-term via LLM consolidation and fed back to improve future
// responses.
package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"scm/internal/model"
	"scm/pkg/authx"
	"scm/pkg/llmclient"
	"scm/internal/repo"
)

// ShortTermEntry is one conversation turn returned for context injection.
type ShortTermEntry struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	ToolCalls string `json:"tool_calls,omitempty"`
}

// LongTermContext is the structured knowledge extracted from past interactions.
type LongTermContext struct {
	Profile string   `json:"profile"`
	Facts   []string `json:"facts"`
}

// RetrieveResult packs the two layers together.
type RetrieveResult struct {
	ShortTerm []ShortTermEntry
	LongTerm  LongTermContext
}

// Config tunes the memory module.
type Config struct {
	WindowSize          int // recent turns to return (default 10)
	ConsolidateInterval int // unconsolidated turns before triggering extraction (default 5)
}

// Service is the memory module. It is independent of the assistant and can
// be used by any caller that holds an actor.
type Service struct {
	mem  *repository.AssistantMemoryRepo
	node *repository.MemoryNodeRepo
	edge *repository.MemoryEdgeRepo
	prof *repository.MemoryProfileRepo
	llm  *llmclient.Client
	cfg  Config
}

func NewService(
	mem *repository.AssistantMemoryRepo,
	node *repository.MemoryNodeRepo,
	edge *repository.MemoryEdgeRepo,
	prof *repository.MemoryProfileRepo,
	llm *llmclient.Client,
	cfg Config,
) *Service {
	if cfg.WindowSize <= 0 {
		cfg.WindowSize = 10
	}
	if cfg.ConsolidateInterval <= 0 {
		cfg.ConsolidateInterval = 5
	}
	return &Service{mem: mem, node: node, edge: edge, prof: prof, llm: llm, cfg: cfg}
}

// Store saves a conversation turn and triggers consolidation if the
// unconsolidated count exceeds the threshold.
func (s *Service) Store(ctx context.Context, actor *authx.Actor, agentRole, message, reply, toolCalls string) {
	entry := model.AssistantMemory{
		TenantID: actor.TenantID, UserID: actor.UserID, AgentRole: agentRole,
		UserMessage: message, AssistantReply: reply, ToolCalls: toolCalls,
	}
	if err := s.mem.Create(&entry); err != nil {
		return // best-effort; don't fail the chat for memory
	}

	// Trigger consolidation if enough unconsolidated turns have accumulated.
	// Run in a goroutine so it doesn't block the chat response.
	if n, _ := s.mem.UnconsolidatedCount(ctx, actor.TenantID, actor.UserID); n >= int64(s.cfg.ConsolidateInterval) {
		go s.consolidate(ctx, actor)
	}
}

// Retrieve returns the two-layer memory context for the user.
func (s *Service) Retrieve(ctx context.Context, actor *authx.Actor) *RetrieveResult {
	// Short-term: last N turns, reversed to chronological order.
	turns, _ := s.mem.ListRecent(ctx, actor.TenantID, actor.UserID, s.cfg.WindowSize)
	var short []ShortTermEntry
	for i := len(turns) - 1; i >= 0; i-- {
		t := turns[i]
		short = append(short, ShortTermEntry{Role: "user", Content: t.UserMessage})
		content := t.AssistantReply
		// If the reply was a tool-only round, summarise instead of replaying.
		if content == "" && t.ToolCalls != "" {
			content = "(调用了工具: " + t.ToolCalls + ")"
		}
		if content != "" {
			short = append(short, ShortTermEntry{Role: "assistant", Content: content, ToolCalls: t.ToolCalls})
		}
	}

	// Long-term: profile + top facts from the knowledge graph.
	var ltc LongTermContext
	if p, err := s.prof.Get(ctx, actor.TenantID, actor.UserID); err == nil && p != nil && p.ProfileJSON != "" {
		ltc.Profile = p.ProfileJSON
	}
	edges, _ := s.edge.TopEdges(ctx, actor.TenantID, actor.UserID, 10)
	if len(edges) > 0 {
		nodeMap := map[uint]string{}
		nodes, _ := s.node.ListByUser(ctx, actor.TenantID, actor.UserID)
		for _, n := range nodes {
			nodeMap[n.ID] = n.Label
		}
		for _, e := range edges {
			from := nodeMap[e.FromNodeID]
			to := nodeMap[e.ToNodeID]
			if from == "" || to == "" {
				continue
			}
			ltc.Facts = append(ltc.Facts, fmt.Sprintf("%s --%s--> %s (权重 %.1f)", from, e.RelationType, to, e.Weight))
		}
	}

	return &RetrieveResult{ShortTerm: short, LongTerm: ltc}
}

// Clear removes all memory (short-term + long-term) for the user.
func (s *Service) Clear(ctx context.Context, actor *authx.Actor) error {
	t, u := actor.TenantID, actor.UserID
	_ = s.mem.DeleteByUser(ctx, t, u)
	_ = s.edge.DeleteByUser(ctx, t, u)
	_ = s.node.DeleteByUser(ctx, t, u)
	_ = s.prof.DeleteByUser(ctx, t, u)
	return nil
}

// consolidate runs the LLM extraction over unconsolidated turns and updates
// the knowledge graph. It is called asynchronously from Store.
func (s *Service) consolidate(ctx context.Context, actor *authx.Actor) {
	turns, err := s.mem.ListUnconsolidated(ctx, actor.TenantID, actor.UserID)
	if err != nil || len(turns) == 0 {
		return
	}

	// Build the conversation transcript for the LLM.
	var b strings.Builder
	for _, t := range turns {
		b.WriteString("用户: " + t.UserMessage + "\n")
		b.WriteString("助手: " + t.AssistantReply + "\n")
		if t.ToolCalls != "" {
			b.WriteString("工具调用: " + t.ToolCalls + "\n")
		}
		b.WriteString("\n")
	}

	prompt := fmt.Sprintf(`从以下对话中提取结构化知识，只返回 JSON，不要任何其他文字:
{
  "nodes": [{"type":"MATERIAL|SUPPLIER|PO|BOM|CATEGORY|USER","entity_id":0,"label":"名称"}],
  "edges": [{"from":"源标签","to":"目标标签","relation":"PREFERS|FREQUENTLY_BUYS|SUPPLIES|LAST_ORDERED","weight":1}],
  "profile": "一句话描述用户的行为习惯或偏好"
}

对话:
%s`, b.String())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := s.llm.Chat(ctx, []llmclient.Message{
		{Role: "system", Content: "你是一个知识提取器。只返回 JSON，不要任何其他文字。"},
		{Role: "user", Content: prompt},
	}, nil)
	if err != nil {
		return
	}

	content := resp.Choices[0].Message.Content
	// Strip markdown fences if present.
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var extract struct {
		Nodes []struct {
			Type     string `json:"type"`
			EntityID uint   `json:"entity_id"`
			Label    string `json:"label"`
		} `json:"nodes"`
		Edges []struct {
			From     string  `json:"from"`
			To       string  `json:"to"`
			Relation string  `json:"relation"`
			Weight   float64 `json:"weight"`
		} `json:"edges"`
		Profile string `json:"profile"`
	}
	if err := json.Unmarshal([]byte(content), &extract); err != nil {
		return
	}

	// Upsert nodes.
	nodeByLabel := map[string]uint{}
	for _, n := range extract.Nodes {
		if n.Label == "" || n.Type == "" {
			continue
		}
		nd, err := s.node.FindOrCreate(ctx, actor.TenantID, actor.UserID, n.Type, n.EntityID, n.Label)
		if err != nil {
			continue
		}
		nodeByLabel[n.Label] = nd.ID
	}

	// Upsert edges.
	now := time.Now()
	for _, e := range extract.Edges {
		fromID, ok1 := nodeByLabel[e.From]
		toID, ok2 := nodeByLabel[e.To]
		if !ok1 || !ok2 || e.Relation == "" {
			continue
		}
		if e.Weight <= 0 {
			e.Weight = 1
		}
		_ = s.edge.Upsert(ctx, actor.TenantID, actor.UserID, fromID, toID, e.Relation, e.Weight)
		// Also set the reverse direction for symmetric relations.
		_ = s.edge.Upsert(ctx, actor.TenantID, actor.UserID, toID, fromID, e.Relation, e.Weight*0.5)
		// Touch the edge's last_updated.
		s.edge.Upsert(ctx, actor.TenantID, actor.UserID, fromID, toID, e.Relation, 0) // weight=0 to just update timestamp
		_ = now                                                                  // suppress unused
	}

	// Update profile if provided.
	if extract.Profile != "" {
		_ = s.prof.Upsert(ctx, actor.TenantID, actor.UserID, extract.Profile)
	}

	// Mark turns as consolidated.
	ids := make([]uint, len(turns))
	for i, t := range turns {
		ids[i] = t.ID
	}
	_ = s.mem.MarkConsolidated(ids)
}
