package memory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
)

const (
	historyFileName = "history.jsonl"
	stateFileName   = "state.json"
	// tailChunkSize is how much of the log is read per step when scanning
	// backwards for the most recent turns.
	tailChunkSize = 64 * 1024
	// defaultMaxFileBytes caps a single history.jsonl before it is archived.
	defaultMaxFileBytes = 32 << 20 // 32 MB
)

// Turn is one persisted assistant conversation round: the user's message and
// the assistant's reply, written as a single JSONL record.
type Turn struct {
	TenantID    uint            `json:"tenant_id"`
	UserID      uint            `json:"user_id"`
	AgentRole   string          `json:"agent_role,omitempty"`
	AgentName   string          `json:"agent_name,omitempty"`
	UserMessage string          `json:"user_message"`
	Attachments json.RawMessage `json:"attachments,omitempty"`
	Reply       string          `json:"assistant_reply,omitempty"`
	Usage       json.RawMessage `json:"usage,omitempty"`
	ToolCalls   string          `json:"tool_calls,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
}

// storeState is the sidecar written next to history.jsonl. The log itself is
// strictly append-only, so "which turns have already been folded into the
// long-term graph" is a single byte offset rather than a per-record flag.
type storeState struct {
	ConsolidatedBytes int64     `json:"consolidated_bytes"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// JSONLStore persists conversation turns as newline-delimited JSON, one
// directory per (tenant, user):
//
//	<dataDir>/t<tenant>/u<user>/history.jsonl   append-only turn log
//	<dataDir>/t<tenant>/u<user>/state.json      consolidation watermark
//
// It replaces the sys_assistant_memory table: the log is inspectable with
// plain tools (cat/jq), survives independently of the database, and keeps
// the full conversation instead of hard-deleting everything past a turn cap.
type JSONLStore struct {
	dir      string
	maxBytes int64
	locks    sync.Map // uint64(tenant<<32|user) -> *sync.Mutex
}

// NewJSONLStore returns a store rooted at dir. maxFileBytes <= 0 selects the
// 32 MB default; rotation archives the log rather than deleting turns.
func NewJSONLStore(dir string, maxFileBytes int64) *JSONLStore {
	if dir == "" {
		dir = filepath.Join("data", "assistant-memory")
	}
	if maxFileBytes <= 0 {
		maxFileBytes = defaultMaxFileBytes
	}
	return &JSONLStore{dir: dir, maxBytes: maxFileBytes}
}

// Dir reports the storage root. Used by the migration command and diagnostics.
func (s *JSONLStore) Dir() string { return s.dir }

// userDir builds the per-user path. Both components are unsigned integers, so
// no path traversal is reachable from caller input.
func (s *JSONLStore) userDir(tenantID, userID uint) string {
	return filepath.Join(s.dir, fmt.Sprintf("t%d", tenantID), fmt.Sprintf("u%d", userID))
}

// lockFor serialises access to one user's files. Store() runs on the request
// goroutine while consolidate() runs on its own, so appends and watermark
// updates must not interleave.
func (s *JSONLStore) lockFor(tenantID, userID uint) *sync.Mutex {
	key := uint64(tenantID)<<32 | uint64(userID)
	v, _ := s.locks.LoadOrStore(key, &sync.Mutex{})
	return v.(*sync.Mutex)
}

// Append writes one turn to the end of the user's log.
func (s *JSONLStore) Append(ctx context.Context, tenantID, userID uint, rec Turn) error {
	mu := s.lockFor(tenantID, userID)
	mu.Lock()
	defer mu.Unlock()

	dir := s.userDir(tenantID, userID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create memory dir: %w", err)
	}
	rec.TenantID, rec.UserID = tenantID, userID
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = time.Now()
	}
	line, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("encode turn: %w", err)
	}

	path := filepath.Join(dir, historyFileName)
	// Rotate before writing: the incoming turn must land in a file that is
	// under the size cap, and never in one that is about to be archived.
	s.maybeRotate(dir, path)

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open history: %w", err)
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("append history: %w", err)
	}
	return nil
}

// maybeRotate archives history.jsonl once it has grown past maxBytes. It
// refuses to rotate while unconsolidated turns remain, so the watermark is
// never left pointing into a file that has just been moved away. Called before
// an append, which means an over-cap file is archived on the next write after
// consolidation catches up.
func (s *JSONLStore) maybeRotate(dir, path string) {
	info, err := os.Stat(path)
	if err != nil || info.Size() < s.maxBytes {
		return
	}
	if st := s.readState(dir); st.ConsolidatedBytes < info.Size() {
		return
	}
	archived := filepath.Join(dir, "history-"+time.Now().Format("20060102-150405")+".jsonl")
	if err := os.Rename(path, archived); err != nil {
		zap.L().Warn("[memory] history rotation failed", zap.String("path", path), zap.Error(err))
		return
	}
	if err := s.writeState(dir, storeState{}); err != nil {
		zap.L().Warn("[memory] watermark reset failed after rotation", zap.Error(err))
	}
	zap.L().Info("[memory] history rotated",
		zap.String("archived", archived), zap.Int64("bytes", info.Size()))
}

// Tail returns the last limit turns in chronological order. It scans the file
// backwards in chunks, so the cost tracks the requested window rather than the
// total size of the log.
func (s *JSONLStore) Tail(ctx context.Context, tenantID, userID uint, limit int) ([]Turn, error) {
	if limit <= 0 {
		return nil, nil
	}
	mu := s.lockFor(tenantID, userID)
	mu.Lock()
	defer mu.Unlock()

	path := filepath.Join(s.userDir(tenantID, userID), historyFileName)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}

	var out []Turn
	offset := info.Size()
	buf := make([]byte, tailChunkSize)
	var pending []byte // leading fragment of a line whose tail was in a later chunk
	for offset > 0 && len(out) < limit {
		read := int64(tailChunkSize)
		if read > offset {
			read = offset
		}
		offset -= read
		if _, err := f.ReadAt(buf[:read], offset); err != nil && err != io.EOF {
			return nil, err
		}
		chunk := make([]byte, 0, read+int64(len(pending)))
		chunk = append(chunk, buf[:read]...)
		chunk = append(chunk, pending...)

		lines := bytes.Split(chunk, []byte("\n"))
		pending = lines[0]
		rest := lines[1:]
		if offset == 0 {
			// Reached the head of the file: what looked like a fragment is
			// actually the first complete record.
			rest = append([][]byte{pending}, rest...)
			pending = nil
		}
		for i := len(rest) - 1; i >= 0 && len(out) < limit; i-- {
			if t, ok := parseTurn(rest[i]); ok {
				out = append(out, t)
			}
		}
	}

	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

// Unconsolidated returns every whole turn written after the watermark, together
// with the offset to commit once they have been processed. A torn trailing
// line (a crash mid-append) is excluded and left pending rather than dropped,
// so it is retried on the next pass.
func (s *JSONLStore) Unconsolidated(ctx context.Context, tenantID, userID uint) (turns []Turn, watermark int64, err error) {
	mu := s.lockFor(tenantID, userID)
	mu.Lock()
	defer mu.Unlock()

	dir := s.userDir(tenantID, userID)
	path := filepath.Join(dir, historyFileName)
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, 0, nil
		}
		return nil, 0, err
	}
	start := s.readState(dir).ConsolidatedBytes
	if start < 0 || start > info.Size() {
		start = 0 // stale watermark (rotation, manual truncation) — rescan
	}
	if start >= info.Size() {
		return nil, info.Size(), nil
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()
	buf := make([]byte, info.Size()-start)
	if _, err := io.ReadFull(io.NewSectionReader(f, start, int64(len(buf))), buf); err != nil {
		return nil, 0, err
	}
	end := bytes.LastIndexByte(buf, '\n')
	if end < 0 {
		return nil, start, nil
	}
	complete := buf[:end+1]
	for _, line := range bytes.Split(complete, []byte("\n")) {
		if t, ok := parseTurn(line); ok {
			turns = append(turns, t)
		}
	}
	return turns, start + int64(len(complete)), nil
}

// MarkConsolidated advances the watermark to an offset previously returned by
// Unconsolidated, so those turns are not extracted into the graph twice.
func (s *JSONLStore) MarkConsolidated(ctx context.Context, tenantID, userID uint, offset int64) error {
	mu := s.lockFor(tenantID, userID)
	mu.Lock()
	defer mu.Unlock()

	dir := s.userDir(tenantID, userID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	st := s.readState(dir)
	if offset <= st.ConsolidatedBytes {
		return nil
	}
	st.ConsolidatedBytes = offset
	return s.writeState(dir, st)
}

// Delete removes a user's history log and watermark (the "clear memory" path).
// The long-term graph lives in the database and is cleared separately.
func (s *JSONLStore) Delete(ctx context.Context, tenantID, userID uint) error {
	mu := s.lockFor(tenantID, userID)
	mu.Lock()
	defer mu.Unlock()
	return os.RemoveAll(s.userDir(tenantID, userID))
}

// ImportBulk writes a batch of pre-existing turns, replacing any log already
// present, and sets the watermark so the first consolidatedPrefix records are
// treated as already folded into the long-term graph. It exists for the
// one-shot migration off sys_assistant_memory.
func (s *JSONLStore) ImportBulk(ctx context.Context, tenantID, userID uint, turns []Turn, consolidatedPrefix int) error {
	if len(turns) == 0 {
		return nil
	}
	mu := s.lockFor(tenantID, userID)
	mu.Lock()
	defer mu.Unlock()

	dir := s.userDir(tenantID, userID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create memory dir: %w", err)
	}
	if consolidatedPrefix < 0 {
		consolidatedPrefix = 0
	}
	if consolidatedPrefix > len(turns) {
		consolidatedPrefix = len(turns)
	}

	var body bytes.Buffer
	var prefixBytes int64
	for i := range turns {
		turns[i].TenantID, turns[i].UserID = tenantID, userID
		line, err := json.Marshal(turns[i])
		if err != nil {
			return fmt.Errorf("encode turn %d: %w", i, err)
		}
		body.Write(line)
		body.WriteByte('\n')
		if i < consolidatedPrefix {
			prefixBytes += int64(len(line)) + 1
		}
	}

	path := filepath.Join(dir, historyFileName)
	if err := os.WriteFile(path, body.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write history: %w", err)
	}
	return s.writeState(dir, storeState{ConsolidatedBytes: prefixBytes})
}

func (s *JSONLStore) readState(dir string) storeState {
	b, err := os.ReadFile(filepath.Join(dir, stateFileName))
	if err != nil {
		return storeState{}
	}
	var st storeState
	if err := json.Unmarshal(b, &st); err != nil {
		return storeState{}
	}
	return st
}

// writeState persists via temp-file + rename so a crash can never leave a
// half-written watermark behind.
func (s *JSONLStore) writeState(dir string, st storeState) error {
	st.UpdatedAt = time.Now()
	b, err := json.Marshal(st)
	if err != nil {
		return err
	}
	tmp := filepath.Join(dir, stateFileName+".tmp")
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, stateFileName))
}

func parseTurn(line []byte) (Turn, bool) {
	line = bytes.TrimSpace(line)
	if len(line) == 0 {
		return Turn{}, false
	}
	var t Turn
	if err := json.Unmarshal(line, &t); err != nil {
		zap.L().Warn("[memory] skipping unreadable history record",
			zap.Int("bytes", len(line)), zap.Error(err))
		return Turn{}, false
	}
	return t, true
}
