package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"scm/internal/model"
	repository "scm/internal/repo"
	"scm/pkg/authx"
	"scm/pkg/llmclient"
)

// Default values used when Config fields are zero.
const (
	defaultWindowSize          = 10
	defaultConsolidateInterval = 5
	defaultShortTermKeepTurns  = 100
	defaultDecayInterval       = 24 * time.Hour
	defaultDecayAgeDays        = 7
	defaultDecayFactor         = 0.9
	defaultDecayFloor          = 0.05
)

// ShortTermEntry is one conversation turn returned for context injection.
type ShortTermEntry struct {
	Role        string          `json:"role"`
	Content     string          `json:"content"`
	ToolCalls   string          `json:"tool_calls,omitempty"`
	AgentName   string          `json:"agent_name,omitempty"`
	Timestamp   time.Time       `json:"timestamp,omitempty"`
	Attachments json.RawMessage `json:"attachments,omitempty"` // JSON array for user turns
	Usage       json.RawMessage `json:"usage,omitempty"`       // JSON AssistantUsage for assistant turns
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

// DecayConfig tunes the periodic edge-weight decay loop.
type DecayConfig struct {
	// Interval between decay runs. 0 defaults to 24h.
	Interval time.Duration
	// OlderThanDays is how stale an edge's LastUpdated must be before
	// we start multiplying its weight. 0 defaults to 7.
	OlderThanDays int
	// Factor is multiplied into stale weights each run; < 1 means decay.
	// 0 or out-of-range defaults to 0.9 (-10%).
	Factor float64
	// Floor is the weight below which a decayed edge is PURGED entirely
	// (natural forgetting). 0 is treated as 0.05 default.
	Floor float64
}

// Config tunes the memory module.
type Config struct {
	WindowSize          int // recent turns to return (default 10)
	ConsolidateInterval int // unconsolidated turns before triggering extraction (default 5)
	// ShortTermKeepTurns is how many recent turns to keep per user when
	// trimming old rows. 0 defaults to 100. This is the "short-term memory
	// compression" policy: Store() calls TrimOldTurnsKeepRecent after
	// every insert so the table never grows unbounded.
	ShortTermKeepTurns int
	// Decay controls the long-term edge-weight decay loop (optional).
	// If Decay.Interval < 0 the loop is disabled; StartDecayLoop will
	// return immediately without starting any goroutine.
	Decay DecayConfig
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
		cfg.WindowSize = defaultWindowSize
	}
	if cfg.ConsolidateInterval <= 0 {
		cfg.ConsolidateInterval = defaultConsolidateInterval
	}
	if cfg.ShortTermKeepTurns <= 0 {
		cfg.ShortTermKeepTurns = defaultShortTermKeepTurns
	}
	if cfg.Decay.Interval == 0 {
		cfg.Decay.Interval = defaultDecayInterval
	}
	if cfg.Decay.OlderThanDays <= 0 {
		cfg.Decay.OlderThanDays = defaultDecayAgeDays
	}
	if cfg.Decay.Factor <= 0 || cfg.Decay.Factor >= 1 {
		cfg.Decay.Factor = defaultDecayFactor
	}
	if cfg.Decay.Floor <= 0 {
		cfg.Decay.Floor = defaultDecayFloor
	}
	return &Service{mem: mem, node: node, edge: edge, prof: prof, llm: llm, cfg: cfg}
}

// Store saves a conversation turn and triggers consolidation if the
// unconsolidated count exceeds the threshold. It also trims old short-term
// turns per (ShortTermKeepTurns) so memory never grows unbounded.
func (s *Service) Store(ctx context.Context, actor *authx.Actor,
	agentRole, agentName, message, userAttachmentsJSON, reply, assistantUsageJSON, toolCalls string) {
	if actor == nil || actor.UserID == 0 {
		return
	}
	entry := model.AssistantMemory{
		TenantID: actor.TenantID, UserID: actor.UserID,
		AgentRole: agentRole, AgentName: agentName,
		UserMessage: message, UserAttachmentsJSON: userAttachmentsJSON,
		AssistantReply: reply, AssistantUsageJSON: assistantUsageJSON,
		ToolCalls: toolCalls,
	}
	if err := s.mem.Create(&entry); err != nil {
		return // best-effort; don't fail the chat for memory
	}

	// Trim old turns synchronously — best-effort, non-blocking even if slow.
	if deleted, err := s.mem.TrimOldTurnsKeepRecent(ctx, actor.TenantID, actor.UserID, s.cfg.ShortTermKeepTurns); err == nil && deleted > 0 {
		zap.L().Info("[memory trim] old short-term turns removed",
			zap.Uint("tenant_id", actor.TenantID),
			zap.Uint("user_id", actor.UserID),
			zap.Int64("deleted", deleted),
			zap.Int("keep", s.cfg.ShortTermKeepTurns),
		)
	}

	// Trigger consolidation if enough unconsolidated turns have accumulated.
	// Run in a goroutine so it doesn't block the chat response.
	if n, _ := s.mem.UnconsolidatedCount(ctx, actor.TenantID, actor.UserID); n >= int64(s.cfg.ConsolidateInterval) {
		go s.consolidate(ctx, actor)
	}
}

// RunDecayOnce applies one pass of the edge decay algorithm. It's safe to
// call on-demand (e.g. from tests or an admin endpoint) and is the inner
// body of StartDecayLoop.
func (s *Service) RunDecayOnce(ctx context.Context) (int64, int64, error) {
	decayed, purged, err := s.edge.ApplyDecay(ctx,
		s.cfg.Decay.OlderThanDays, s.cfg.Decay.Factor, s.cfg.Decay.Floor)
	if err != nil {
		zap.L().Error("[memory decay] ApplyDecay failed", zap.Error(err))
	} else if decayed > 0 || purged > 0 {
		zap.L().Info("[memory decay] run complete",
			zap.Int("older_than_days", s.cfg.Decay.OlderThanDays),
			zap.Float64("factor", s.cfg.Decay.Factor),
			zap.Float64("floor", s.cfg.Decay.Floor),
			zap.Int64("edges_decayed", decayed),
			zap.Int64("edges_purged", purged),
		)
	}
	return decayed, purged, err
}

// StartDecayLoop launches a long-lived goroutine that runs RunDecayOnce at
// the configured interval. It returns immediately; pass a cancellable
// context if you need to shut it down cleanly. If Decay.Interval < 0 this
// is a no-op (allows tests to disable the loop).
func (s *Service) StartDecayLoop(parentCtx context.Context) {
	if s.cfg.Decay.Interval < 0 {
		return
	}
	go func() {
		// Run once at startup to catch up after a restart gap, then tick.
		ctx, cancel := context.WithCancel(parentCtx)
		defer cancel()
		_, _, _ = s.RunDecayOnce(ctx)
		t := time.NewTicker(s.cfg.Decay.Interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				_, _, _ = s.RunDecayOnce(ctx)
			}
		}
	}()
}

// Retrieve returns the two-layer memory context for the user.
func (s *Service) Retrieve(ctx context.Context, actor *authx.Actor) *RetrieveResult {
	// Short-term: last N turns, reversed to chronological order.
	turns, _ := s.mem.ListRecent(ctx, actor.TenantID, actor.UserID, s.cfg.WindowSize)
	var short []ShortTermEntry
	for i := len(turns) - 1; i >= 0; i-- {
		t := turns[i]
		ts := t.CreatedAt
		userEntry := ShortTermEntry{Role: "user", Content: t.UserMessage, Timestamp: ts}
		if t.UserAttachmentsJSON != "" {
			userEntry.Attachments = json.RawMessage(t.UserAttachmentsJSON)
		}
		short = append(short, userEntry)
		content := t.AssistantReply
		// If the reply was a tool-only round, summarise instead of replaying.
		if content == "" && t.ToolCalls != "" {
			content = "(调用了工具: " + t.ToolCalls + ")"
		}
		if content != "" {
			ass := ShortTermEntry{Role: "assistant", Content: content, ToolCalls: t.ToolCalls, Timestamp: ts}
			if t.AgentName != "" {
				ass.AgentName = t.AgentName
			}
			if t.AssistantUsageJSON != "" {
				ass.Usage = json.RawMessage(t.AssistantUsageJSON)
			}
			short = append(short, ass)
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
		_ = now                                                                       // suppress unused
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
