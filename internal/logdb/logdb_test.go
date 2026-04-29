package logdb_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/go-go-golems/go-go-agent/internal/evaljs"
	"github.com/go-go-golems/go-go-agent/internal/helpdb"
	"github.com/go-go-golems/go-go-agent/internal/helpdocs"
	"github.com/go-go-golems/go-go-agent/internal/logdb"
)

func TestOpenCreatesPrivateLogSchemasAndEvalSession(t *testing.T) {
	ctx := context.Background()
	input, err := helpdb.PrepareInputDB(ctx, helpdb.InputDBConfig{HelpFS: helpdocs.FS, HelpDir: helpdocs.Dir})
	if err != nil {
		t.Fatalf("prepare input: %v", err)
	}
	defer input.Close()
	output, err := helpdb.PrepareOutputDB(ctx, "")
	if err != nil {
		t.Fatalf("prepare output: %v", err)
	}
	defer output.Close()

	factory, err := evaljs.NewEngineFactory(evaljs.Scope{InputDB: input.DB, OutputDB: output.DB})
	if err != nil {
		t.Fatalf("build factory: %v", err)
	}

	path := filepath.Join(t.TempDir(), "chat-log.sqlite")
	db, err := logdb.Open(ctx, logdb.Config{Path: path, Profile: "test-profile", ChatSessionID: "chat-test"}, factory)
	if err != nil {
		t.Fatalf("open log db: %v", err)
	}
	defer db.Close()

	for _, table := range []string{
		"turns",
		"blocks",
		"turn_block_membership",
		"repldb_meta",
		"sessions",
		"evaluations",
		"console_events",
		"bindings",
		"binding_versions",
		"binding_docs",
		"chat_log_sessions",
		"eval_tool_calls",
	} {
		if !tableExists(t, db, table) {
			t.Fatalf("expected table %s", table)
		}
	}

	var evalSessionID, profile string
	if err := db.ReplStore.DB().QueryRowContext(ctx, `SELECT eval_session_id, profile FROM chat_log_sessions WHERE chat_session_id = ?`, "chat-test").Scan(&evalSessionID, &profile); err != nil {
		t.Fatalf("query chat_log_sessions: %v", err)
	}
	if evalSessionID != "chat-test:eval_js" {
		t.Fatalf("unexpected eval session id: %s", evalSessionID)
	}
	if profile != "test-profile" {
		t.Fatalf("unexpected profile: %s", profile)
	}

	var sessionCount int
	if err := db.ReplStore.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM sessions WHERE session_id = ?`, evalSessionID).Scan(&sessionCount); err != nil {
		t.Fatalf("query sessions: %v", err)
	}
	if sessionCount != 1 {
		t.Fatalf("expected repl session row, got %d", sessionCount)
	}
}

func tableExists(t *testing.T, db *logdb.DB, name string) bool {
	t.Helper()
	var count int
	if err := db.ReplStore.DB().QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, name).Scan(&count); err != nil {
		t.Fatalf("query sqlite_master: %v", err)
	}
	return count == 1
}
