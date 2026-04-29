package main

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	glazedsettings "github.com/go-go-golems/glazed/pkg/settings"
	"github.com/go-go-golems/glazed/pkg/types"
	_ "github.com/mattn/go-sqlite3"
)

type inspectSettings struct {
	LogDBPath string `glazed:"log-db"`
	Limit     int    `glazed:"limit"`
	SessionID string `glazed:"session-id"`
	TurnID    string `glazed:"turn-id"`
	Source    bool   `glazed:"source"`
}

type row map[string]any

type InspectQueryCommand struct {
	*cmds.CommandDescription
	kind string
}

var _ cmds.GlazeCommand = &InspectQueryCommand{}

func NewInspectCommands() ([]cmds.Command, error) {
	defs := []struct {
		name  string
		short string
		kind  string
		flags []*fields.Definition
	}{
		{"sessions", "List chat log sessions", "sessions", nil},
		{"eval-calls", "List eval_js tool call correlation rows", "eval-calls", []*fields.Definition{limitField()}},
		{"repl-evals", "List replsession evaluation cells", "repl-evals", []*fields.Definition{limitField(), fields.New("source", fields.TypeBool, fields.WithDefault(false), fields.WithHelp("Print full raw source instead of a preview"))}},
		{"bindings", "List persistent JavaScript bindings", "bindings", []*fields.Definition{fields.New("session-id", fields.TypeString, fields.WithDefault(""), fields.WithHelp("Filter by repl session id"))}},
		{"turns", "List persisted chat turns", "turns", []*fields.Definition{limitField()}},
		{"blocks", "List unique persisted chat blocks", "blocks", []*fields.Definition{limitField()}},
		{"turn-blocks", "List turn/block membership rows", "turn-blocks", []*fields.Definition{limitField(), fields.New("turn-id", fields.TypeString, fields.WithDefault(""), fields.WithHelp("Filter by turn id"))}},
		{"schema", "List SQLite tables and row counts", "schema", nil},
	}
	out := make([]cmds.Command, 0, len(defs))
	for _, def := range defs {
		cmd, err := newInspectQueryCommand(def.name, def.short, def.kind, def.flags...)
		if err != nil {
			return nil, err
		}
		out = append(out, cmd)
	}
	return out, nil
}

func limitField() *fields.Definition {
	return fields.New("limit", fields.TypeInteger, fields.WithDefault(20), fields.WithHelp("Maximum rows to print"))
}

func newInspectQueryCommand(name, short, kind string, extraFlags ...*fields.Definition) (*InspectQueryCommand, error) {
	glazedSection, err := glazedsettings.NewGlazedSchema()
	if err != nil {
		return nil, err
	}
	flags := []*fields.Definition{
		fields.New("log-db", fields.TypeString, fields.WithDefault(""), fields.WithHelp("Path to private chat log SQLite DB"), fields.WithRequired(true)),
	}
	flags = append(flags, extraFlags...)
	desc := cmds.NewCommandDescription(
		name,
		cmds.WithParents("inspect"),
		cmds.WithShort(short),
		cmds.WithLong(fmt.Sprintf(`%s.

Examples:
  chat inspect %s --log-db /tmp/chat.sqlite
  chat inspect %s --log-db /tmp/chat.sqlite --output json`, short, name, name)),
		cmds.WithFlags(flags...),
		cmds.WithSections(glazedSection),
	)
	return &InspectQueryCommand{CommandDescription: desc, kind: kind}, nil
}

func (c *InspectQueryCommand) RunIntoGlazeProcessor(ctx context.Context, vals *values.Values, gp middlewares.Processor) error {
	s := &inspectSettings{Limit: 20}
	if err := vals.DecodeSectionInto(schema.DefaultSlug, s); err != nil {
		return err
	}
	return withInspectDB(ctx, *s, func(db *sql.DB) error {
		rows, headers, err := c.inspectRows(ctx, db, *s)
		if err != nil {
			return err
		}
		for _, r := range rows {
			if err := gp.AddRow(ctx, rowToGlazedRow(headers, r)); err != nil {
				return err
			}
		}
		return nil
	})
}

func (c *InspectQueryCommand) inspectRows(ctx context.Context, db *sql.DB, s inspectSettings) ([]row, []string, error) {
	switch c.kind {
	case "sessions":
		rows, err := queryRows(ctx, db, `SELECT chat_session_id, eval_session_id, conv_id, profile, started_at_ms, strict, log_schema_version FROM chat_log_sessions ORDER BY started_at_ms DESC`)
		return rows, []string{"chat_session_id", "eval_session_id", "conv_id", "profile", "started_at_ms", "strict", "log_schema_version"}, err
	case "eval-calls":
		rows, err := queryRows(ctx, db, `SELECT eval_tool_call_id, repl_cell_id, created_at_ms, error_text, code, eval_output_json FROM eval_tool_calls ORDER BY created_at_ms DESC LIMIT ?`, s.Limit)
		for _, r := range rows {
			r["code"] = preview(fmt.Sprint(r["code"]), 120)
			r["eval_output_json"] = preview(fmt.Sprint(r["eval_output_json"]), 120)
		}
		return rows, []string{"eval_tool_call_id", "repl_cell_id", "created_at_ms", "error_text", "code", "eval_output_json"}, err
	case "repl-evals":
		rows, err := queryRows(ctx, db, `SELECT evaluation_id, session_id, cell_id, created_at, ok, error_text, raw_source, result_json FROM evaluations ORDER BY created_at DESC LIMIT ?`, s.Limit)
		for _, r := range rows {
			if !s.Source {
				r["raw_source"] = preview(fmt.Sprint(r["raw_source"]), 120)
			}
			r["result_json"] = preview(fmt.Sprint(r["result_json"]), 120)
		}
		return rows, []string{"evaluation_id", "session_id", "cell_id", "created_at", "ok", "error_text", "raw_source", "result_json"}, err
	case "bindings":
		rows, err := queryRows(ctx, db, `SELECT b.session_id, b.name, b.latest_cell_id, COALESCE(bv.runtime_type, '') AS runtime_type, COALESCE(bv.display_value, '') AS display_value FROM bindings b LEFT JOIN binding_versions bv ON bv.binding_id = b.binding_id AND bv.cell_id = b.latest_cell_id WHERE (? = '' OR b.session_id = ?) ORDER BY b.session_id, b.name`, s.SessionID, s.SessionID)
		for _, r := range rows {
			r["display_value"] = preview(fmt.Sprint(r["display_value"]), 120)
		}
		return rows, []string{"session_id", "name", "latest_cell_id", "runtime_type", "display_value"}, err
	case "turns":
		rows, err := queryRows(ctx, db, `SELECT conv_id, session_id, turn_id, turn_created_at_ms, runtime_key, inference_id, updated_at_ms FROM turns ORDER BY updated_at_ms DESC LIMIT ?`, s.Limit)
		return rows, []string{"conv_id", "session_id", "turn_id", "turn_created_at_ms", "runtime_key", "inference_id", "updated_at_ms"}, err
	case "blocks":
		rows, err := queryRows(ctx, db, `SELECT block_id, kind, role, first_seen_at_ms, payload_json FROM blocks ORDER BY first_seen_at_ms DESC LIMIT ?`, s.Limit)
		for _, r := range rows {
			r["payload_json"] = preview(fmt.Sprint(r["payload_json"]), 160)
		}
		return rows, []string{"block_id", "kind", "role", "first_seen_at_ms", "payload_json"}, err
	case "turn-blocks":
		rows, err := queryRows(ctx, db, `SELECT conv_id, session_id, turn_id, phase, snapshot_created_at_ms, ordinal, block_id, content_hash FROM turn_block_membership WHERE (? = '' OR turn_id = ?) ORDER BY snapshot_created_at_ms DESC, ordinal ASC LIMIT ?`, s.TurnID, s.TurnID, s.Limit)
		return rows, []string{"conv_id", "session_id", "turn_id", "phase", "snapshot_created_at_ms", "ordinal", "block_id", "content_hash"}, err
	case "schema":
		tables, err := tableNames(ctx, db)
		if err != nil {
			return nil, nil, err
		}
		out := make([]row, 0, len(tables))
		for _, table := range tables {
			count, err := countTable(ctx, db, table)
			if err != nil {
				return nil, nil, err
			}
			out = append(out, row{"table": table, "rows": count})
		}
		return out, []string{"table", "rows"}, nil
	default:
		return nil, nil, fmt.Errorf("unknown inspect kind %q", c.kind)
	}
}

func rowToGlazedRow(headers []string, r row) types.Row {
	pairs := make([]types.MapRowPair, 0, len(headers))
	for _, h := range headers {
		pairs = append(pairs, types.MRP(h, r[h]))
	}
	return types.NewRow(pairs...)
}

func withInspectDB(ctx context.Context, s inspectSettings, fn func(*sql.DB) error) error {
	path := strings.TrimSpace(s.LogDBPath)
	if path == "" {
		return fmt.Errorf("--log-db is required")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?mode=ro&_busy_timeout=5000", abs))
	if err != nil {
		return err
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return err
	}
	return fn(db)
}

func queryRows(ctx context.Context, db *sql.DB, query string, args ...any) ([]row, error) {
	rs, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	cols, err := rs.Columns()
	if err != nil {
		return nil, err
	}
	out := []row{}
	for rs.Next() {
		values := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rs.Scan(ptrs...); err != nil {
			return nil, err
		}
		r := row{}
		for i, col := range cols {
			r[col] = normalizeDBValue(values[i])
		}
		out = append(out, r)
	}
	return out, rs.Err()
}

func tableNames(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := queryRows(ctx, db, `SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, fmt.Sprint(r["name"]))
	}
	return out, nil
}

func countTable(ctx context.Context, db *sql.DB, table string) (int64, error) {
	if !safeIdent(table) {
		return 0, fmt.Errorf("unsafe table name %q", table)
	}
	var count int64
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM "`+table+`"`).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func normalizeDBValue(v any) any {
	switch x := v.(type) {
	case []byte:
		return string(x)
	case int64:
		if x > 1_000_000_000_000 {
			return fmt.Sprintf("%s (%d)", time.UnixMilli(x).Format(time.RFC3339), x)
		}
		return x
	default:
		return x
	}
}

func preview(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	if max <= 0 || len(s) <= max {
		return s
	}
	if max <= 1 {
		return "…"
	}
	return s[:max-1] + "…"
}

func safeIdent(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !(r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}
