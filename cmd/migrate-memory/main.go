// Command migrate-memory performs the one-shot move of assistant conversation
// history out of the sys_assistant_memory table and into the JSONL store that
// internal/memory now reads and writes.
//
// Run it once, with the server stopped, before any new turns are written:
//
//	make migrate-memory                  # export to JSONL, then clear the table
//	go run ./cmd/migrate-memory -dry-run  # export only, leave the table alone
//
// The command OVERWRITES any existing history.jsonl for the users it touches,
// so it is not meant to be run repeatedly against a live system. It refuses to
// clear the table unless every row was exported successfully.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"go.uber.org/zap"

	"scm/internal/config"
	"scm/internal/database"
	"scm/internal/memory"
)

// legacyTable is the retired short-term memory table.
const legacyTable = "sys_assistant_memory"

// legacyRow mirrors the old schema. It lives here rather than in
// internal/model because that model no longer exists — the application never
// reads this table again, only this command does.
type legacyRow struct {
	ID                  uint      `gorm:"column:id"`
	TenantID            uint      `gorm:"column:tenant_id"`
	UserID              uint      `gorm:"column:user_id"`
	AgentRole           string    `gorm:"column:agent_role"`
	AgentName           string    `gorm:"column:agent_name"`
	UserMessage         string    `gorm:"column:user_message"`
	UserAttachmentsJSON string    `gorm:"column:user_attachments_json"`
	AssistantReply      string    `gorm:"column:assistant_reply"`
	AssistantUsageJSON  string    `gorm:"column:assistant_usage_json"`
	ToolCalls           string    `gorm:"column:tool_calls"`
	Consolidated        bool      `gorm:"column:consolidated"`
	CreatedAt           time.Time `gorm:"column:created_at"`
}

func main() {
	dryRun := flag.Bool("dry-run", false, "export to JSONL but leave the table untouched")
	dirOverride := flag.String("dir", "", "override the configured memory data dir for this run")
	flag.Parse()

	logger, _ := zap.NewProduction()
	defer logger.Sync()
	zap.ReplaceGlobals(logger)

	cfg := config.Load()
	dataDir := cfg.Memory.DataDir
	if *dirOverride != "" {
		dataDir = *dirOverride
	}

	// database.Open also runs AutoMigrate; the legacy table is no longer in
	// the model list, so it is neither recreated nor dropped here.
	db, err := database.Open(cfg.DB.Driver, cfg.DB.DSN)
	if err != nil {
		zap.L().Fatal("db open", zap.Error(err))
	}

	if !db.Migrator().HasTable(legacyTable) {
		fmt.Printf("nothing to migrate: table %q does not exist\n", legacyTable)
		return
	}

	var rows []legacyRow
	if err := db.Table(legacyTable).
		Order("tenant_id ASC, user_id ASC, id ASC").
		Find(&rows).Error; err != nil {
		zap.L().Fatal("read legacy turns", zap.Error(err))
	}
	if len(rows) == 0 {
		fmt.Printf("nothing to migrate: table %q is empty\n", legacyTable)
		return
	}

	// Group by (tenant, user) while preserving id order so each user's log is
	// written chronologically.
	type userKey struct{ tenant, user uint }
	var order []userKey
	groups := map[userKey][]legacyRow{}
	for _, r := range rows {
		k := userKey{r.TenantID, r.UserID}
		if _, seen := groups[k]; !seen {
			order = append(order, k)
		}
		groups[k] = append(groups[k], r)
	}

	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		zap.L().Fatal("create memory data dir", zap.String("dir", dataDir), zap.Error(err))
	}
	store := memory.NewJSONLStore(dataDir, cfg.Memory.MaxFileBytes)
	ctx := context.Background()

	migrated, failed := 0, 0
	for _, k := range order {
		legacy := groups[k]

		// The old code marked whole batches consolidated at once, so the
		// consolidated rows form a prefix of the id-ordered set. That prefix
		// becomes the byte watermark in the new store.
		prefix := 0
		for _, r := range legacy {
			if !r.Consolidated {
				break
			}
			prefix++
		}

		turns := make([]memory.Turn, 0, len(legacy))
		for _, r := range legacy {
			turns = append(turns, toTurn(r))
		}

		if err := store.ImportBulk(ctx, k.tenant, k.user, turns, prefix); err != nil {
			failed += len(legacy)
			zap.L().Error("import failed",
				zap.Uint("tenant_id", k.tenant), zap.Uint("user_id", k.user),
				zap.Int("turns", len(legacy)), zap.Error(err))
			continue
		}
		migrated += len(turns)
		zap.L().Info("migrated user",
			zap.Uint("tenant_id", k.tenant), zap.Uint("user_id", k.user),
			zap.Int("turns", len(turns)), zap.Int("already_consolidated", prefix))
	}

	fmt.Printf("\nexported %d/%d turns for %d users into %s\n", migrated, len(rows), len(order), dataDir)

	if *dryRun {
		fmt.Println("dry run: table left untouched")
		return
	}
	if failed > 0 || migrated != len(rows) {
		zap.L().Fatal("refusing to clear the table: migration incomplete",
			zap.Int("migrated", migrated), zap.Int("total", len(rows)), zap.Int("failed", failed))
	}

	if err := db.Exec("DELETE FROM " + legacyTable).Error; err != nil {
		zap.L().Fatal("clear legacy table", zap.Error(err))
	}
	fmt.Printf("cleared %s\n", legacyTable)
	fmt.Printf("the table is now unused; drop it whenever you like:\n  DROP TABLE %s;\n", legacyTable)
}

func toTurn(r legacyRow) memory.Turn {
	return memory.Turn{
		AgentRole:   r.AgentRole,
		AgentName:   r.AgentName,
		UserMessage: r.UserMessage,
		Attachments: memory.RawJSON(r.UserAttachmentsJSON),
		Reply:       r.AssistantReply,
		Usage:       memory.RawJSON(r.AssistantUsageJSON),
		ToolCalls:   r.ToolCalls,
		CreatedAt:   r.CreatedAt,
	}
}
