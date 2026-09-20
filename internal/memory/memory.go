package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	repository "scm/internal/repo"
	"scm/pkg/authx"
	"scm/pkg/llmclient"
)

// Default values used when Config fields are zero.
const (
	defaultWindowSize          = 10
	defaultHistoryLimit        = 100
	defaultConsolidateInterval = 5
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
	WindowSize          int // recent turns injected into the LLM context (default 10)
	HistoryLimit        int // recent turns returned to the chat UI (default 100)
	ConsolidateInterval int // unconsolidated turns before triggering extraction (default 5)
	// Decay controls the long-term edge-weight decay loop (optional).
	// If Decay.Interval < 0 the loop is disabled; StartDecayLoop will
	// return immediately without starting any goroutine.
	Decay DecayConfig
}

// Service is the memory module. It is independent of the assistant and can
// be used by any caller that holds an actor.
//
// Conversation turns live in JSONL files (see JSONLStore); only the long-term
// knowledge graph is backed by the database.
type Service struct {
	store *JSONLStore
	node  *repository.MemoryNodeRepo
	edge  *repository.MemoryEdgeRepo
	prof  *repository.MemoryProfileRepo
	llm   *llmclient.Client
	cfg   Config
	// consolidating guards against overlapping extraction runs for the same
	// user; consolidate() is launched from a goroutine per Store().
	consolidating sync.Map // uint64(tenant<<32|user) -> struct{}
}

func NewService(
	store *JSONLStore,
	node *repository.MemoryNodeRepo,
	edge *repository.MemoryEdgeRepo,
	prof *repository.MemoryProfileRepo,
	llm *llmclient.Client,
	cfg Config,
) *Service {
	if cfg.WindowSize <= 0 {
		cfg.WindowSize = defaultWindowSize
	}
	if cfg.HistoryLimit <= 0 {
		cfg.HistoryLimit = defaultHistoryLimit
	}
	if cfg.ConsolidateInterval <= 0 {
		cfg.ConsolidateInterval = defaultConsolidateInterval
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
	return &Service{store: store, node: node, edge: edge, prof: prof, llm: llm, cfg: cfg}
}

// Store appends a conversation turn to the user's JSONL log and triggers
// consolidation once enough unconsolidated turns have accumulated. It never
// returns an error: memory is best-effort and must not break the chat.
func (s *Service) Store(ctx context.Context, actor *authx.Actor,
	agentRole, agentName, message, userAttachmentsJSON, reply, assistantUsageJSON, toolCalls string) {
	if actor == nil || actor.UserID == 0 {
		return
	}
	rec := Turn{
		AgentRole:   agentRole,
		AgentName:   agentName,
		UserMessage: message,
		Attachments: RawJSON(userAttachmentsJSON),
		Reply:       reply,
		Usage:       RawJSON(assistantUsageJSON),
		ToolCalls:   toolCalls,
	}
	if err := s.store.Append(ctx, actor.TenantID, actor.UserID, rec); err != nil {
		zap.L().Warn("[memory] append turn failed",
			zap.Uint("tenant_id", actor.TenantID),
			zap.Uint("user_id", actor.UserID),
			zap.Error(err),
		)
		return
	}

	// Trigger consolidation if enough unconsolidated turns have accumulated.
	// Run in a goroutine so it doesn't block the chat response.
	if pending, _, err := s.store.Unconsolidated(ctx, actor.TenantID, actor.UserID); err == nil &&
		len(pending) >= s.cfg.ConsolidateInterval {
		go s.consolidate(actor)
	}
}

// RawJSON embeds an already-serialised payload as nested JSON so the log stays
// readable by jq. Anything that is not valid JSON is stored as a JSON string
// rather than corrupting the record.
func RawJSON(s string) json.RawMessage {
	if s == "" {
		return nil
	}
	if json.Valid([]byte(s)) {
		return json.RawMessage(s)
	}
	b, err := json.Marshal(s)
	if err != nil {
		return nil
	}
	return b
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

// Retrieve returns the two-layer memory context for the user. The short-term
// layer is sized by WindowSize, which is deliberately smaller than what
// History() serves: this output is injected into the LLM prompt, so it trades
// recall for token cost.
func (s *Service) Retrieve(ctx context.Context, actor *authx.Actor) *RetrieveResult {
	// Short-term: last WindowSize turns, already in chronological order.
	turns, _ := s.store.Tail(ctx, actor.TenantID, actor.UserID, s.cfg.WindowSize)
	short := toShortTerm(turns)

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

// History returns turns for chat-UI replay. It is deliberately decoupled from
// WindowSize: the UI wants as much of the transcript as it can render, while
// the LLM prompt only gets a narrow window. limit <= 0 falls back to
// Config.HistoryLimit.
func (s *Service) History(ctx context.Context, actor *authx.Actor, limit int) []ShortTermEntry {
	if actor == nil || actor.UserID == 0 {
		return nil
	}
	if limit <= 0 {
		limit = s.cfg.HistoryLimit
	}
	turns, err := s.store.Tail(ctx, actor.TenantID, actor.UserID, limit)
	if err != nil {
		zap.L().Warn("[memory] read history failed",
			zap.Uint("tenant_id", actor.TenantID),
			zap.Uint("user_id", actor.UserID),
			zap.Error(err),
		)
		return nil
	}
	return toShortTerm(turns)
}

// toShortTerm expands each stored round into the user/assistant message pair
// the chat protocol expects. turns must already be in chronological order.
func toShortTerm(turns []Turn) []ShortTermEntry {
	var short []ShortTermEntry
	for _, t := range turns {
		userEntry := ShortTermEntry{Role: "user", Content: t.UserMessage, Timestamp: t.CreatedAt}
		if len(t.Attachments) > 0 {
			userEntry.Attachments = t.Attachments
		}
		short = append(short, userEntry)

		content := t.Reply
		// If the reply was a tool-only round, summarise instead of replaying.
		if content == "" && t.ToolCalls != "" {
			content = "(调用了工具: " + t.ToolCalls + ")"
		}
		if content == "" {
			continue
		}
		ass := ShortTermEntry{Role: "assistant", Content: content, ToolCalls: t.ToolCalls, Timestamp: t.CreatedAt}
		if t.AgentName != "" {
			ass.AgentName = t.AgentName
		}
		if len(t.Usage) > 0 {
			ass.Usage = t.Usage
		}
		short = append(short, ass)
	}
	return short
}

// Clear removes all memory (JSONL history + long-term graph) for the user.
func (s *Service) Clear(ctx context.Context, actor *authx.Actor) error {
	t, u := actor.TenantID, actor.UserID
	if err := s.store.Delete(ctx, t, u); err != nil {
		zap.L().Warn("[memory] delete history failed",
			zap.Uint("tenant_id", t), zap.Uint("user_id", u), zap.Error(err))
	}
	_ = s.edge.DeleteByUser(ctx, t, u)
	_ = s.node.DeleteByUser(ctx, t, u)
	_ = s.prof.DeleteByUser(ctx, t, u)
	return nil
}

// consolidate runs the LLM extraction over unconsolidated turns and updates
// the knowledge graph. It is called asynchronously from Store.
func (s *Service) consolidate(actor *authx.Actor) {
	// One extraction per user at a time: Store() can fire several goroutines
	// before the first LLM call returns, and re-extracting the same turns
	// would inflate every edge weight.
	key := uint64(actor.TenantID)<<32 | uint64(actor.UserID)
	if _, busy := s.consolidating.LoadOrStore(key, struct{}{}); busy {
		return
	}
	defer s.consolidating.Delete(key)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	turns, watermark, err := s.store.Unconsolidated(ctx, actor.TenantID, actor.UserID)
	if err != nil || len(turns) == 0 {
		return
	}

	// Build the conversation transcript for the LLM.
	var b strings.Builder
	for _, t := range turns {
		b.WriteString("用户: " + t.UserMessage + "\n")
		b.WriteString("助手: " + t.Reply + "\n")
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
	}

	// Update profile if provided.
	if extract.Profile != "" {
		_ = s.prof.Upsert(ctx, actor.TenantID, actor.UserID, extract.Profile)
	}

	// Advance the watermark past exactly the turns we just extracted. Turns
	// appended while the LLM was in flight stay pending for the next pass.
	if err := s.store.MarkConsolidated(ctx, actor.TenantID, actor.UserID, watermark); err != nil {
		zap.L().Warn("[memory] advance consolidation watermark failed",
			zap.Uint("tenant_id", actor.TenantID),
			zap.Uint("user_id", actor.UserID),
			zap.Error(err),
		)
	}
}
