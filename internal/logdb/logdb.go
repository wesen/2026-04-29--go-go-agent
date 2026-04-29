package logdb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-go-golems/go-go-goja/engine"
	"github.com/go-go-golems/go-go-goja/pkg/replapi"
	"github.com/go-go-golems/go-go-goja/pkg/repldb"
	"github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	"github.com/rs/zerolog/log"
)

const schemaVersion = 1

type Config struct {
	Path          string
	Strict        bool
	Profile       string
	ChatSessionID string
}

type DB struct {
	Path          string
	TurnStore     chatstore.TurnStore
	ReplStore     *repldb.Store
	ReplApp       *replapi.App
	ChatSessionID string
	EvalSessionID string
	ConvID        string
	Strict        bool
}

func Open(ctx context.Context, cfg Config, factory *engine.Factory) (*DB, error) {
	if factory == nil {
		return nil, fmt.Errorf("open log db: eval runtime factory is nil")
	}
	path, err := resolvePath(cfg.Path)
	if err != nil {
		return nil, err
	}

	turnStore, err := chatstore.NewSQLiteTurnStore(path)
	if err != nil {
		return nil, fmt.Errorf("open turn store: %w", err)
	}
	cleanup := func() { _ = turnStore.Close() }

	replStore, err := repldb.Open(ctx, path)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("open repl store: %w", err)
	}
	cleanup = func() {
		_ = turnStore.Close()
		_ = replStore.Close()
	}

	if err := migrateAppTables(ctx, replStore.DB()); err != nil {
		cleanup()
		return nil, err
	}

	replApp, err := replapi.New(factory, log.Logger, replapi.WithProfile(replapi.ProfilePersistent), replapi.WithStore(replStore))
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("create repl app: %w", err)
	}

	chatSessionID := strings.TrimSpace(cfg.ChatSessionID)
	if chatSessionID == "" {
		chatSessionID = newID("chat")
	}
	evalSessionID := chatSessionID + ":eval_js"
	evalSession, err := replApp.CreateSessionWithOptions(ctx, replapi.SessionOverrides{ID: evalSessionID})
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("create eval session: %w", err)
	}

	db := &DB{
		Path:          path,
		TurnStore:     turnStore,
		ReplStore:     replStore,
		ReplApp:       replApp,
		ChatSessionID: chatSessionID,
		EvalSessionID: evalSession.ID,
		ConvID:        newID("conv"),
		Strict:        cfg.Strict,
	}
	if err := db.recordChatLogSession(ctx, cfg); err != nil {
		cleanup()
		return nil, err
	}
	return db, nil
}

func (d *DB) Close() error {
	if d == nil {
		return nil
	}
	var errs []error
	if d.TurnStore != nil {
		if err := d.TurnStore.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if d.ReplStore != nil {
		if err := d.ReplStore.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("close log db: %v", errs)
	}
	return nil
}

func migrateAppTables(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("migrate log db: sql db is nil")
	}
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS chat_log_sessions (
  chat_session_id TEXT PRIMARY KEY,
  eval_session_id TEXT NOT NULL,
  conv_id TEXT NOT NULL,
  profile TEXT NOT NULL DEFAULT '',
  log_db_path TEXT NOT NULL DEFAULT '',
  started_at_ms INTEGER NOT NULL,
  ended_at_ms INTEGER,
  strict INTEGER NOT NULL DEFAULT 0,
  log_schema_version INTEGER NOT NULL DEFAULT 1
);`,
		`CREATE TABLE IF NOT EXISTS eval_tool_calls (
  eval_tool_call_id INTEGER PRIMARY KEY AUTOINCREMENT,
  tool_call_id TEXT NOT NULL DEFAULT '',
  chat_session_id TEXT NOT NULL,
  turn_id TEXT NOT NULL DEFAULT '',
  eval_session_id TEXT NOT NULL,
  repl_cell_id INTEGER,
  created_at_ms INTEGER NOT NULL,
  code TEXT NOT NULL,
  input_json TEXT NOT NULL DEFAULT '{}',
  eval_output_json TEXT NOT NULL DEFAULT '{}',
  error_text TEXT NOT NULL DEFAULT '',
  FOREIGN KEY(chat_session_id) REFERENCES chat_log_sessions(chat_session_id)
);`,
		`CREATE INDEX IF NOT EXISTS eval_tool_calls_by_session_created ON eval_tool_calls(chat_session_id, created_at_ms DESC);`,
		`CREATE INDEX IF NOT EXISTS eval_tool_calls_by_eval_cell ON eval_tool_calls(eval_session_id, repl_cell_id);`,
		`CREATE INDEX IF NOT EXISTS eval_tool_calls_by_turn ON eval_tool_calls(turn_id);`,
	}
	for _, stmt := range stmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("migrate log db: %w", err)
		}
	}
	return nil
}

func (d *DB) recordChatLogSession(ctx context.Context, cfg Config) error {
	_, err := d.ReplStore.DB().ExecContext(ctx, `INSERT OR REPLACE INTO chat_log_sessions(
chat_session_id, eval_session_id, conv_id, profile, log_db_path, started_at_ms, strict, log_schema_version
) VALUES(?, ?, ?, ?, ?, ?, ?, ?)`, d.ChatSessionID, d.EvalSessionID, d.ConvID, cfg.Profile, d.Path, time.Now().UnixMilli(), boolInt(d.Strict), schemaVersion)
	if err != nil {
		return fmt.Errorf("record chat log session: %w", err)
	}
	return nil
}

func (d *DB) insertEvalToolCall(ctx context.Context, row EvalCorrelation) error {
	_, err := d.ReplStore.DB().ExecContext(ctx, `INSERT INTO eval_tool_calls(
tool_call_id, chat_session_id, turn_id, eval_session_id, repl_cell_id, created_at_ms, code, input_json, eval_output_json, error_text
) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, row.ToolCallID, row.ChatSessionID, row.TurnID, row.EvalSessionID, nullableCellID(row.ReplCellID), row.CreatedAtMs, row.Code, string(jsonOrDefault(row.InputJSON, `{}`)), string(jsonOrDefault(row.EvalOutputJSON, `{}`)), row.ErrorText)
	if err != nil {
		return fmt.Errorf("insert eval tool call: %w", err)
	}
	return nil
}

func resolvePath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return filepath.Join(os.TempDir(), newID("chat-log")+".sqlite"), nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("create log db directory: %w", err)
	}
	return path, nil
}

func newID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func nullableCellID(id int64) any {
	if id <= 0 {
		return nil
	}
	return id
}

func jsonOrDefault(v json.RawMessage, fallback string) json.RawMessage {
	if len(v) == 0 || !json.Valid(v) {
		return json.RawMessage(fallback)
	}
	return v
}
