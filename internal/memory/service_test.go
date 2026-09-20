package memory

import (
	"context"
	"path/filepath"
	"testing"

	"scm/internal/database"
	repository "scm/internal/repo"
	"scm/pkg/authx"
)

// newTestService wires a Service against a throwaway SQLite database and a
// real JSONL store, with no LLM client. ConsolidateInterval is set high enough
// that knowledge extraction — and therefore any LLM call — is never triggered.
func newTestService(t *testing.T) (*Service, *authx.Actor) {
	t.Helper()

	db, err := database.Open("sqlite", filepath.Join(t.TempDir(), "mem.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	gdb := repository.NewGormDB(db.DB)
	svc := NewService(
		NewJSONLStore(t.TempDir(), 0),
		repository.NewMemoryNodeRepo(gdb),
		repository.NewMemoryEdgeRepo(gdb),
		repository.NewMemoryProfileRepo(gdb),
		nil,
		Config{WindowSize: 2, HistoryLimit: 5, ConsolidateInterval: 1000},
	)
	return svc, &authx.Actor{TenantID: 1, UserID: 42, Username: "tester"}
}

// storeTurn is the exact call shape the assistant handler uses after a chat
// round completes.
func storeTurn(t *testing.T, svc *Service, actor *authx.Actor, msg, reply string) {
	t.Helper()
	svc.Store(context.Background(), actor, "assistant", "采购助手", msg, "", reply, "", "")
}

func TestServiceStoreThenHistory(t *testing.T) {
	svc, actor := newTestService(t)
	ctx := context.Background()

	for i, msg := range []string{"第一轮", "第二轮", "第三轮"} {
		storeTurn(t, svc, actor, msg, "回复"+string(rune('1'+i)))
	}

	// limit 0 falls back to Config.HistoryLimit (5), so all three rounds come
	// back as six chat messages, oldest first.
	got := svc.History(ctx, actor, 0)
	if len(got) != 6 {
		t.Fatalf("got %d entries, want 6: %+v", len(got), got)
	}
	for i, want := range []string{"第一轮", "回复1", "第二轮", "回复2", "第三轮", "回复3"} {
		if got[i].Content != want {
			t.Errorf("entry %d = %q, want %q", i, got[i].Content, want)
		}
	}
	if got[0].Role != "user" || got[1].Role != "assistant" {
		t.Errorf("roles not alternating: %q/%q", got[0].Role, got[1].Role)
	}
	if got[1].AgentName != "采购助手" {
		t.Errorf("agent name lost: %q", got[1].AgentName)
	}

	// An explicit limit overrides the default and keeps only the newest round.
	latest := svc.History(ctx, actor, 1)
	if len(latest) != 2 {
		t.Fatalf("got %d entries for limit=1, want 2", len(latest))
	}
	if latest[0].Content != "第三轮" {
		t.Errorf("limit=1 returned %q, want the newest round", latest[0].Content)
	}
}

// TestServiceRetrieveUsesNarrowerWindow pins the split that motivated
// History(): the LLM prompt gets WindowSize rounds, the UI gets HistoryLimit.
func TestServiceRetrieveUsesNarrowerWindow(t *testing.T) {
	svc, actor := newTestService(t)
	ctx := context.Background()

	for _, msg := range []string{"第一轮", "第二轮", "第三轮"} {
		storeTurn(t, svc, actor, msg, "回复")
	}

	res := svc.Retrieve(ctx, actor)
	if res == nil {
		t.Fatal("Retrieve returned nil")
	}
	if len(res.ShortTerm) != 4 {
		t.Fatalf("got %d entries, want WindowSize(2) rounds = 4: %+v",
			len(res.ShortTerm), res.ShortTerm)
	}
	if res.ShortTerm[0].Content != "第二轮" {
		t.Errorf("window should hold the newest rounds, first = %q", res.ShortTerm[0].Content)
	}
	// The UI path must still see everything.
	if n := len(svc.History(ctx, actor, 0)); n != 6 {
		t.Errorf("History returned %d entries, want 6", n)
	}
}

func TestServiceStoreIgnoresAnonymousActor(t *testing.T) {
	svc, actor := newTestService(t)
	ctx := context.Background()

	// Both shapes reach Store from the handler when auth is off or the token
	// carries no user; neither may panic nor write a record.
	svc.Store(ctx, nil, "assistant", "a", "消息", "", "回复", "", "")
	svc.Store(ctx, &authx.Actor{TenantID: 1, UserID: 0}, "assistant", "a", "消息", "", "回复", "", "")

	if got := svc.History(ctx, actor, 0); len(got) != 0 {
		t.Fatalf("anonymous turns were persisted: %+v", got)
	}
}

func TestServiceHistoryEmptyForNewUser(t *testing.T) {
	svc, actor := newTestService(t)
	if got := svc.History(context.Background(), actor, 0); got != nil {
		t.Fatalf("want nil history for a user with no log, got %+v", got)
	}
}

func TestServiceAttachmentsAndUsageRoundTrip(t *testing.T) {
	svc, actor := newTestService(t)
	ctx := context.Background()

	svc.Store(ctx, actor, "assistant", "采购助手", "看看这个报价",
		`[{"id":"a1","name":"报价单.pdf","size":2048}]`,
		"已解析报价单",
		`{"total_tokens":128,"model":"qwen"}`,
		`["doc_parse"]`)

	got := svc.History(ctx, actor, 0)
	if len(got) != 2 {
		t.Fatalf("got %d entries, want 2", len(got))
	}
	if string(got[0].Attachments) != `[{"id":"a1","name":"报价单.pdf","size":2048}]` {
		t.Errorf("attachments mangled: %s", got[0].Attachments)
	}
	if string(got[1].Usage) != `{"total_tokens":128,"model":"qwen"}` {
		t.Errorf("usage mangled: %s", got[1].Usage)
	}
	if got[1].ToolCalls != `["doc_parse"]` {
		t.Errorf("tool_calls mangled: %s", got[1].ToolCalls)
	}
}

func TestServiceClearWipesHistoryAndGraph(t *testing.T) {
	svc, actor := newTestService(t)
	ctx := context.Background()

	storeTurn(t, svc, actor, "记住我喜欢冷轧板", "好的")
	if err := svc.prof.Upsert(ctx, actor.TenantID, actor.UserID, `{"偏好":"冷轧板"}`); err != nil {
		t.Fatalf("seed profile: %v", err)
	}

	if res := svc.Retrieve(ctx, actor); res.LongTerm.Profile == "" {
		t.Fatal("profile should be visible before Clear")
	}

	if err := svc.Clear(ctx, actor); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if got := svc.History(ctx, actor, 0); len(got) != 0 {
		t.Errorf("history survived Clear: %+v", got)
	}
	if res := svc.Retrieve(ctx, actor); res.LongTerm.Profile != "" {
		t.Errorf("profile survived Clear: %s", res.LongTerm.Profile)
	}
}
