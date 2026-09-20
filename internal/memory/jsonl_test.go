package memory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func newTestStore(t *testing.T, maxFileBytes int64) *JSONLStore {
	t.Helper()
	return NewJSONLStore(t.TempDir(), maxFileBytes)
}

func turn(msg string) Turn {
	return Turn{
		AgentRole:   "assistant",
		AgentName:   "测试助手",
		UserMessage: msg,
		Reply:       "回复: " + msg,
		CreatedAt:   time.Date(2026, 9, 19, 10, 0, 0, 0, time.UTC),
	}
}

func appendN(t *testing.T, s *JSONLStore, tenant, user uint, n int, pad int) {
	t.Helper()
	ctx := context.Background()
	for i := 0; i < n; i++ {
		rec := turn(fmt.Sprintf("消息-%03d", i))
		if pad > 0 {
			rec.UserMessage += strings.Repeat("x", pad)
		}
		if err := s.Append(ctx, tenant, user, rec); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}
}

// messages extracts the user message prefix so assertions stay readable.
func messages(turns []Turn) []string {
	out := make([]string, len(turns))
	for i, t := range turns {
		out[i] = t.UserMessage
	}
	return out
}

func hasPrefix(t *testing.T, got []string, wantIdx []int) {
	t.Helper()
	if len(got) != len(wantIdx) {
		t.Fatalf("got %d turns, want %d: %v", len(got), len(wantIdx), got)
	}
	for i, want := range wantIdx {
		prefix := fmt.Sprintf("消息-%03d", want)
		if !strings.HasPrefix(got[i], prefix) {
			t.Errorf("turn %d = %q, want prefix %q", i, got[i], prefix)
		}
	}
}

func TestTailOnEmptyStore(t *testing.T) {
	s := newTestStore(t, 0)
	got, err := s.Tail(context.Background(), 1, 1, 10)
	if err != nil {
		t.Fatalf("unexpected error for missing file: %v", err)
	}
	if got != nil {
		t.Fatalf("want nil turns, got %v", got)
	}
}

func TestAppendThenTailChronological(t *testing.T) {
	s := newTestStore(t, 0)
	appendN(t, s, 1, 7, 5, 0)

	got, err := s.Tail(context.Background(), 1, 7, 3)
	if err != nil {
		t.Fatal(err)
	}
	// Tail must return the LAST three, oldest first — the caller feeds this
	// straight into a chat transcript.
	hasPrefix(t, messages(got), []int{2, 3, 4})

	if got[0].Reply != "回复: 消息-002" {
		t.Errorf("reply not round-tripped: %q", got[0].Reply)
	}
	if got[0].TenantID != 1 || got[0].UserID != 7 {
		t.Errorf("tenant/user not stamped: %d/%d", got[0].TenantID, got[0].UserID)
	}
}

func TestTailLimitExceedsHistory(t *testing.T) {
	s := newTestStore(t, 0)
	appendN(t, s, 2, 2, 4, 0)

	got, err := s.Tail(context.Background(), 2, 2, 100)
	if err != nil {
		t.Fatal(err)
	}
	hasPrefix(t, messages(got), []int{0, 1, 2, 3})
}

// TestTailAcrossChunkBoundary is the important one: Tail reads the file
// backwards in 64 KiB chunks, so a record straddling a chunk boundary is split
// across two reads. Getting the carry-over wrong silently drops or corrupts
// turns once a user's log grows past one chunk.
func TestTailAcrossChunkBoundary(t *testing.T) {
	s := newTestStore(t, 0)
	// ~3 KiB per record × 60 ≈ 180 KiB, i.e. several chunks.
	appendN(t, s, 3, 3, 60, 3000)

	path := filepath.Join(s.userDir(3, 3), historyFileName)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() <= tailChunkSize {
		t.Fatalf("fixture too small to span a chunk: %d bytes", info.Size())
	}

	got, err := s.Tail(context.Background(), 3, 3, 60)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 60 {
		t.Fatalf("got %d turns, want all 60", len(got))
	}
	for i := range got {
		want := fmt.Sprintf("消息-%03d", i)
		if !strings.HasPrefix(got[i].UserMessage, want) {
			t.Fatalf("turn %d out of order or corrupted: %.40q", i, got[i].UserMessage)
		}
		// A mis-joined chunk boundary shows up as a truncated message body.
		if wantLen := len(want) + 3000; len(got[i].UserMessage) != wantLen {
			t.Fatalf("turn %d length = %d, want %d (chunk boundary mishandled)",
				i, len(got[i].UserMessage), wantLen)
		}
	}

	// A window that starts mid-file must also be exact.
	tail5, err := s.Tail(context.Background(), 3, 3, 5)
	if err != nil {
		t.Fatal(err)
	}
	hasPrefix(t, messages(tail5), []int{55, 56, 57, 58, 59})
}

func TestConsolidationWatermark(t *testing.T) {
	s := newTestStore(t, 0)
	ctx := context.Background()
	appendN(t, s, 4, 4, 3, 0)

	pending, watermark, err := s.Unconsolidated(ctx, 4, 4)
	if err != nil {
		t.Fatal(err)
	}
	hasPrefix(t, messages(pending), []int{0, 1, 2})
	if watermark <= 0 {
		t.Fatalf("watermark should advance past the records, got %d", watermark)
	}

	if err := s.MarkConsolidated(ctx, 4, 4, watermark); err != nil {
		t.Fatal(err)
	}
	pending, _, err = s.Unconsolidated(ctx, 4, 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 {
		t.Fatalf("want 0 unconsolidated after mark, got %d", len(pending))
	}

	// Turns written after the mark must be picked up again, and only those.
	appendN(t, s, 4, 4, 2, 0)
	pending, _, err = s.Unconsolidated(ctx, 4, 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 2 {
		t.Fatalf("want the 2 new turns, got %d", len(pending))
	}

	// Tail is unaffected by the watermark: history replay always sees all 5.
	all, err := s.Tail(ctx, 4, 4, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 5 {
		t.Fatalf("watermark must not hide turns from Tail, got %d", len(all))
	}
}

func TestMarkConsolidatedNeverGoesBackwards(t *testing.T) {
	s := newTestStore(t, 0)
	ctx := context.Background()
	appendN(t, s, 5, 5, 2, 0)

	_, watermark, err := s.Unconsolidated(ctx, 5, 5)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.MarkConsolidated(ctx, 5, 5, watermark); err != nil {
		t.Fatal(err)
	}
	// A stale or duplicated call must not rewind and re-extract everything.
	if err := s.MarkConsolidated(ctx, 5, 5, 1); err != nil {
		t.Fatal(err)
	}
	pending, _, err := s.Unconsolidated(ctx, 5, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 {
		t.Fatalf("watermark rewound: %d turns became pending again", len(pending))
	}
}

// TestTornTrailingLineIsNotConsumed simulates a crash mid-append: the last
// line has no terminating newline. It must be skipped rather than treated as
// a record, and the watermark must stop before it so it is retried later.
func TestTornTrailingLineIsNotConsumed(t *testing.T) {
	s := newTestStore(t, 0)
	ctx := context.Background()
	appendN(t, s, 6, 6, 2, 0)

	path := filepath.Join(s.userDir(6, 6), historyFileName)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`{"user_message":"截断的记录"`); err != nil {
		t.Fatal(err)
	}
	f.Close()

	pending, watermark, err := s.Unconsolidated(ctx, 6, 6)
	if err != nil {
		t.Fatal(err)
	}
	hasPrefix(t, messages(pending), []int{0, 1})

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if watermark >= info.Size() {
		t.Fatalf("watermark %d consumed the torn line (size %d)", watermark, info.Size())
	}

	got, err := s.Tail(ctx, 6, 6, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("Tail should skip the torn line, got %d turns", len(got))
	}

	// Completing the line makes it readable on the next pass.
	f, err = os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("}\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()

	pending, _, err = s.Unconsolidated(ctx, 6, 6)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 3 {
		t.Fatalf("repaired line should now be pending, got %d", len(pending))
	}
}

func TestCorruptRecordIsSkipped(t *testing.T) {
	s := newTestStore(t, 0)
	ctx := context.Background()
	appendN(t, s, 8, 8, 1, 0)

	path := filepath.Join(s.userDir(8, 8), historyFileName)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprintln(f, "这不是 JSON")
	fmt.Fprintln(f, "")
	f.Close()

	appendN(t, s, 8, 8, 1, 0)

	got, err := s.Tail(ctx, 8, 8, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want the 2 valid records, got %d", len(got))
	}
}

func TestRotationArchivesConsolidatedLog(t *testing.T) {
	// A cap far below one record guarantees the file is over-cap as soon as
	// the first turn is written and consolidated.
	s := newTestStore(t, 1)
	ctx := context.Background()

	appendN(t, s, 9, 9, 1, 0)
	_, watermark, err := s.Unconsolidated(ctx, 9, 9)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.MarkConsolidated(ctx, 9, 9, watermark); err != nil {
		t.Fatal(err)
	}

	// The next append rotates first, so this turn starts a fresh log.
	appendN(t, s, 9, 9, 1, 0)

	dir := s.userDir(9, 9)
	archives, err := filepath.Glob(filepath.Join(dir, "history-*.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if len(archives) != 1 {
		t.Fatalf("want 1 archived log, got %d: %v", len(archives), archives)
	}

	// After rotation the watermark resets, so the new turn is pending again.
	pending, _, err := s.Unconsolidated(ctx, 9, 9)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 {
		t.Fatalf("want 1 pending turn after rotation, got %d", len(pending))
	}
	// Tail only reads the active log; the archived turn is out of the window.
	got, err := s.Tail(ctx, 9, 9, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("want only the post-rotation turn, got %d", len(got))
	}
}

// TestRotationBlockedByPendingTurns guards the invariant that makes the byte
// watermark safe: an over-cap file with unconsolidated turns must NOT be
// archived, or those turns would be lost to the extractor.
func TestRotationBlockedByPendingTurns(t *testing.T) {
	s := newTestStore(t, 1)
	ctx := context.Background()

	appendN(t, s, 10, 10, 3, 0) // never consolidated

	archives, err := filepath.Glob(filepath.Join(s.userDir(10, 10), "history-*.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if len(archives) != 0 {
		t.Fatalf("rotated with pending turns, archives: %v", archives)
	}
	pending, _, err := s.Unconsolidated(ctx, 10, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 3 {
		t.Fatalf("want all 3 turns still pending, got %d", len(pending))
	}
}

func TestDeleteRemovesUserDir(t *testing.T) {
	s := newTestStore(t, 0)
	ctx := context.Background()
	appendN(t, s, 11, 11, 2, 0)
	// Also create state.json so Delete has to remove more than just the log.
	if err := s.MarkConsolidated(ctx, 11, 11, 1); err != nil {
		t.Fatal(err)
	}

	if err := s.Delete(ctx, 11, 11); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(s.userDir(11, 11)); !os.IsNotExist(err) {
		t.Fatalf("user dir still present after Delete: %v", err)
	}
	got, err := s.Tail(ctx, 11, 11, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("want empty history after Delete, got %d", len(got))
	}
}

func TestUsersAreIsolated(t *testing.T) {
	s := newTestStore(t, 0)
	ctx := context.Background()
	appendN(t, s, 1, 1, 3, 0)
	appendN(t, s, 1, 2, 1, 0)
	appendN(t, s, 2, 1, 2, 0)

	for _, tc := range []struct {
		tenant, user uint
		want         int
	}{{1, 1, 3}, {1, 2, 1}, {2, 1, 2}} {
		got, err := s.Tail(ctx, tc.tenant, tc.user, 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != tc.want {
			t.Errorf("t%d/u%d: got %d turns, want %d", tc.tenant, tc.user, len(got), tc.want)
		}
	}
}

func TestImportBulkSetsWatermarkAtPrefix(t *testing.T) {
	s := newTestStore(t, 0)
	ctx := context.Background()

	turns := []Turn{turn("消息-000"), turn("消息-001"), turn("消息-002"), turn("消息-003")}
	if err := s.ImportBulk(ctx, 12, 12, turns, 3); err != nil {
		t.Fatal(err)
	}

	// Only the turn past the consolidated prefix should be re-extracted.
	pending, _, err := s.Unconsolidated(ctx, 12, 12)
	if err != nil {
		t.Fatal(err)
	}
	hasPrefix(t, messages(pending), []int{3})

	// The full transcript is still replayable.
	all, err := s.Tail(ctx, 12, 12, 10)
	if err != nil {
		t.Fatal(err)
	}
	hasPrefix(t, messages(all), []int{0, 1, 2, 3})
}

func TestImportBulkReplacesExistingLog(t *testing.T) {
	s := newTestStore(t, 0)
	ctx := context.Background()
	appendN(t, s, 13, 13, 5, 0)

	if err := s.ImportBulk(ctx, 13, 13, []Turn{turn("消息-000")}, 0); err != nil {
		t.Fatal(err)
	}
	all, err := s.Tail(ctx, 13, 13, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Fatalf("ImportBulk should replace the log, got %d turns", len(all))
	}
}

func TestNestedJSONFieldsSurviveRoundTrip(t *testing.T) {
	s := newTestStore(t, 0)
	ctx := context.Background()

	rec := turn("带附件的消息")
	rec.Attachments = RawJSON(`[{"id":"a1","name":"报表.xlsx","size":2048}]`)
	rec.Usage = RawJSON(`{"total_tokens":123,"model":"qwen"}`)
	rec.ToolCalls = `["material_list"]`
	if err := s.Append(ctx, 14, 14, rec); err != nil {
		t.Fatal(err)
	}

	got, err := s.Tail(ctx, 14, 14, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d turns, want 1", len(got))
	}
	if string(got[0].Attachments) != `[{"id":"a1","name":"报表.xlsx","size":2048}]` {
		t.Errorf("attachments mangled: %s", got[0].Attachments)
	}
	if string(got[0].Usage) != `{"total_tokens":123,"model":"qwen"}` {
		t.Errorf("usage mangled: %s", got[0].Usage)
	}
	if got[0].ToolCalls != `["material_list"]` {
		t.Errorf("tool_calls mangled: %s", got[0].ToolCalls)
	}

	// The log must stay jq-friendly: attachments nested as JSON, not as an
	// escaped string.
	raw, err := os.ReadFile(filepath.Join(s.userDir(14, 14), historyFileName))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), `\"name\":`) {
		t.Errorf("attachments were double-encoded into a string:\n%s", raw)
	}
}

func TestRawJSON(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want string
	}{
		{"", ""},
		{`{"a":1}`, `{"a":1}`},
		{`[1,2]`, `[1,2]`},
		// Not valid JSON → stored as a JSON string so the record stays parseable.
		{`不是 JSON`, `"不是 JSON"`},
	} {
		got := RawJSON(tc.in)
		if string(got) != tc.want {
			t.Errorf("RawJSON(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestToShortTermExpandsRounds(t *testing.T) {
	round := turn("查一下库存")
	round.ToolCalls = `["stock_list"]`
	entries := toShortTerm([]Turn{round})

	if len(entries) != 2 {
		t.Fatalf("want a user+assistant pair, got %d entries", len(entries))
	}
	if entries[0].Role != "user" || entries[0].Content != "查一下库存" {
		t.Errorf("user entry wrong: %+v", entries[0])
	}
	if entries[1].Role != "assistant" || entries[1].AgentName != "测试助手" {
		t.Errorf("assistant entry wrong: %+v", entries[1])
	}
	if entries[1].Timestamp != round.CreatedAt {
		t.Errorf("timestamp lost: %v", entries[1].Timestamp)
	}
}

// TestToShortTermToolOnlyRound covers the case where the assistant produced no
// prose, only tool calls: the UI still needs a bubble, so the turn is
// summarised instead of dropped.
func TestToShortTermToolOnlyRound(t *testing.T) {
	round := Turn{UserMessage: "下单", ToolCalls: "po_create", CreatedAt: time.Now()}
	entries := toShortTerm([]Turn{round})

	if len(entries) != 2 {
		t.Fatalf("want 2 entries, got %d", len(entries))
	}
	if !strings.Contains(entries[1].Content, "po_create") {
		t.Errorf("tool-only round not summarised: %q", entries[1].Content)
	}
}

func TestToShortTermSkipsEmptyAssistant(t *testing.T) {
	entries := toShortTerm([]Turn{{UserMessage: "你好", CreatedAt: time.Now()}})
	if len(entries) != 1 {
		t.Fatalf("want only the user entry, got %d", len(entries))
	}
	if entries[0].Role != "user" {
		t.Errorf("got role %q, want user", entries[0].Role)
	}
}

// TestConcurrentAppendsDoNotInterleave hammers one user's log from many
// goroutines. Store() runs on the request goroutine while consolidate() runs
// on its own, so without the per-user mutex two writers could interleave
// inside a record and corrupt the log. Run under -race.
func TestConcurrentAppendsDoNotInterleave(t *testing.T) {
	s := newTestStore(t, 0)
	ctx := context.Background()

	const writers, perWriter = 16, 25
	var wg sync.WaitGroup
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < perWriter; i++ {
				rec := turn(fmt.Sprintf("w%02d-i%02d", w, i))
				rec.UserMessage += strings.Repeat("y", 500) // force multi-KB records
				if err := s.Append(ctx, 20, 20, rec); err != nil {
					t.Errorf("concurrent append: %v", err)
					return
				}
			}
		}(w)
	}
	wg.Wait()

	// Every record must be intact and parseable — a torn write shows up here
	// as a short read or a missing turn.
	all, err := s.Tail(ctx, 20, 20, writers*perWriter)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != writers*perWriter {
		t.Fatalf("got %d turns, want %d (records lost or merged)", len(all), writers*perWriter)
	}
	seen := map[string]bool{}
	for _, rec := range all {
		if wantLen := len("w00-i00") + 500; len(rec.UserMessage) != wantLen {
			t.Fatalf("record %q has length %d, want %d (interleaved write)",
				rec.UserMessage[:12], len(rec.UserMessage), wantLen)
		}
		key := rec.UserMessage[:7]
		if seen[key] {
			t.Fatalf("duplicate record %q", key)
		}
		seen[key] = true
	}

	// The watermark path must also see exactly the same set, once.
	pending, _, err := s.Unconsolidated(ctx, 20, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != writers*perWriter {
		t.Fatalf("Unconsolidated saw %d turns, want %d", len(pending), writers*perWriter)
	}
}
