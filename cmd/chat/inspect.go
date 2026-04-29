package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/cobra"
)

type inspectSettings struct {
	LogDBPath string
	Limit     int
	JSON      bool
	SessionID string
	TurnID    string
	Source    bool
}

type row map[string]any

func newInspectCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inspect",
		Short: "Inspect private chat log databases",
		Long: `Inspect reads a chat --log-db SQLite database after a run.

All inspect commands are read-only and support --json for machine-readable output.`,
	}
	cmd.AddCommand(newInspectSessionsCommand())
	cmd.AddCommand(newInspectEvalCallsCommand())
	cmd.AddCommand(newInspectReplEvalsCommand())
	cmd.AddCommand(newInspectBindingsCommand())
	cmd.AddCommand(newInspectTurnsCommand())
	cmd.AddCommand(newInspectBlocksCommand())
	cmd.AddCommand(newInspectTurnBlocksCommand())
	cmd.AddCommand(newInspectSchemaCommand())
	return cmd
}

func newInspectSessionsCommand() *cobra.Command {
	var s inspectSettings
	cmd := &cobra.Command{
		Use:   "sessions",
		Short: "List chat log sessions",
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = args
			return withInspectDB(cmd.Context(), s, func(db *sql.DB) error {
				rows, err := queryRows(cmd.Context(), db, `SELECT chat_session_id, eval_session_id, conv_id, profile, started_at_ms, strict, log_schema_version FROM chat_log_sessions ORDER BY started_at_ms DESC`)
				if err != nil {
					return err
				}
				return printInspect(cmd.OutOrStdout(), s.JSON, []string{"chat_session_id", "eval_session_id", "conv_id", "profile", "started", "strict", "schema"}, rows)
			})
		},
	}
	addInspectCommonFlags(cmd, &s, false)
	return cmd
}

func newInspectEvalCallsCommand() *cobra.Command {
	var s inspectSettings
	cmd := &cobra.Command{
		Use:   "eval-calls",
		Short: "List eval_js tool call correlation rows",
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = args
			return withInspectDB(cmd.Context(), s, func(db *sql.DB) error {
				rows, err := queryRows(cmd.Context(), db, `SELECT eval_tool_call_id, repl_cell_id, created_at_ms, error_text, code, eval_output_json FROM eval_tool_calls ORDER BY created_at_ms DESC LIMIT ?`, s.Limit)
				if err != nil {
					return err
				}
				for _, r := range rows {
					r["code"] = preview(fmt.Sprint(r["code"]), 120)
					r["eval_output_json"] = preview(fmt.Sprint(r["eval_output_json"]), 120)
				}
				return printInspect(cmd.OutOrStdout(), s.JSON, []string{"eval_tool_call_id", "repl_cell_id", "created", "error_text", "code", "eval_output_json"}, rows)
			})
		},
	}
	addInspectCommonFlags(cmd, &s, true)
	return cmd
}

func newInspectReplEvalsCommand() *cobra.Command {
	var s inspectSettings
	cmd := &cobra.Command{
		Use:   "repl-evals",
		Short: "List replsession evaluation cells",
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = args
			return withInspectDB(cmd.Context(), s, func(db *sql.DB) error {
				rows, err := queryRows(cmd.Context(), db, `SELECT evaluation_id, session_id, cell_id, created_at, ok, error_text, raw_source, result_json FROM evaluations ORDER BY created_at DESC LIMIT ?`, s.Limit)
				if err != nil {
					return err
				}
				for _, r := range rows {
					if !s.Source {
						r["raw_source"] = preview(fmt.Sprint(r["raw_source"]), 120)
					}
					r["result_json"] = preview(fmt.Sprint(r["result_json"]), 120)
				}
				return printInspect(cmd.OutOrStdout(), s.JSON, []string{"evaluation_id", "session_id", "cell_id", "created_at", "ok", "error_text", "raw_source", "result_json"}, rows)
			})
		},
	}
	addInspectCommonFlags(cmd, &s, true)
	cmd.Flags().BoolVar(&s.Source, "source", false, "Print full raw source instead of a preview")
	return cmd
}

func newInspectBindingsCommand() *cobra.Command {
	var s inspectSettings
	cmd := &cobra.Command{
		Use:   "bindings",
		Short: "List persistent JavaScript bindings",
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = args
			return withInspectDB(cmd.Context(), s, func(db *sql.DB) error {
				rows, err := queryRows(cmd.Context(), db, `SELECT b.session_id, b.name, b.latest_cell_id, COALESCE(bv.runtime_type, '') AS runtime_type, COALESCE(bv.display_value, '') AS display_value FROM bindings b LEFT JOIN binding_versions bv ON bv.binding_id = b.binding_id AND bv.cell_id = b.latest_cell_id WHERE (? = '' OR b.session_id = ?) ORDER BY b.session_id, b.name`, s.SessionID, s.SessionID)
				if err != nil {
					return err
				}
				for _, r := range rows {
					r["display_value"] = preview(fmt.Sprint(r["display_value"]), 120)
				}
				return printInspect(cmd.OutOrStdout(), s.JSON, []string{"session_id", "name", "latest_cell_id", "runtime_type", "display_value"}, rows)
			})
		},
	}
	addInspectCommonFlags(cmd, &s, false)
	cmd.Flags().StringVar(&s.SessionID, "session-id", "", "Filter by repl session id")
	return cmd
}

func newInspectTurnsCommand() *cobra.Command {
	var s inspectSettings
	cmd := &cobra.Command{
		Use:   "turns",
		Short: "List persisted chat turns",
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = args
			return withInspectDB(cmd.Context(), s, func(db *sql.DB) error {
				rows, err := queryRows(cmd.Context(), db, `SELECT conv_id, session_id, turn_id, turn_created_at_ms, runtime_key, inference_id, updated_at_ms FROM turns ORDER BY updated_at_ms DESC LIMIT ?`, s.Limit)
				if err != nil {
					return err
				}
				return printInspect(cmd.OutOrStdout(), s.JSON, []string{"conv_id", "session_id", "turn_id", "created", "runtime_key", "inference_id", "updated"}, rows)
			})
		},
	}
	addInspectCommonFlags(cmd, &s, true)
	return cmd
}

func newInspectBlocksCommand() *cobra.Command {
	var s inspectSettings
	cmd := &cobra.Command{
		Use:   "blocks",
		Short: "List unique persisted chat blocks",
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = args
			return withInspectDB(cmd.Context(), s, func(db *sql.DB) error {
				rows, err := queryRows(cmd.Context(), db, `SELECT block_id, kind, role, first_seen_at_ms, payload_json FROM blocks ORDER BY first_seen_at_ms DESC LIMIT ?`, s.Limit)
				if err != nil {
					return err
				}
				for _, r := range rows {
					r["payload_json"] = preview(fmt.Sprint(r["payload_json"]), 160)
				}
				return printInspect(cmd.OutOrStdout(), s.JSON, []string{"block_id", "kind", "role", "first_seen", "payload_json"}, rows)
			})
		},
	}
	addInspectCommonFlags(cmd, &s, true)
	return cmd
}

func newInspectTurnBlocksCommand() *cobra.Command {
	var s inspectSettings
	cmd := &cobra.Command{
		Use:   "turn-blocks",
		Short: "List turn/block membership rows",
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = args
			return withInspectDB(cmd.Context(), s, func(db *sql.DB) error {
				rows, err := queryRows(cmd.Context(), db, `SELECT conv_id, session_id, turn_id, phase, snapshot_created_at_ms, ordinal, block_id, content_hash FROM turn_block_membership WHERE (? = '' OR turn_id = ?) ORDER BY snapshot_created_at_ms DESC, ordinal ASC LIMIT ?`, s.TurnID, s.TurnID, s.Limit)
				if err != nil {
					return err
				}
				return printInspect(cmd.OutOrStdout(), s.JSON, []string{"conv_id", "session_id", "turn_id", "phase", "snapshot", "ordinal", "block_id", "content_hash"}, rows)
			})
		},
	}
	addInspectCommonFlags(cmd, &s, true)
	cmd.Flags().StringVar(&s.TurnID, "turn-id", "", "Filter by turn id")
	return cmd
}

func newInspectSchemaCommand() *cobra.Command {
	var s inspectSettings
	cmd := &cobra.Command{
		Use:   "schema",
		Short: "List SQLite tables and row counts",
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = args
			return withInspectDB(cmd.Context(), s, func(db *sql.DB) error {
				tables, err := tableNames(cmd.Context(), db)
				if err != nil {
					return err
				}
				out := make([]row, 0, len(tables))
				for _, table := range tables {
					count, err := countTable(cmd.Context(), db, table)
					if err != nil {
						return err
					}
					out = append(out, row{"table": table, "rows": count})
				}
				return printInspect(cmd.OutOrStdout(), s.JSON, []string{"table", "rows"}, out)
			})
		},
	}
	addInspectCommonFlags(cmd, &s, false)
	return cmd
}

func addInspectCommonFlags(cmd *cobra.Command, s *inspectSettings, withLimit bool) {
	cmd.Flags().StringVar(&s.LogDBPath, "log-db", "", "Path to private chat log SQLite DB")
	cmd.Flags().BoolVar(&s.JSON, "json", false, "Print JSON output")
	if withLimit {
		cmd.Flags().IntVar(&s.Limit, "limit", 20, "Maximum rows to print")
	} else {
		s.Limit = 0
	}
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

func printInspect(out io.Writer, asJSON bool, headers []string, rows []row) error {
	if asJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(rows)
	}
	fmt.Fprintln(out, strings.Join(headers, "\t"))
	for _, r := range rows {
		vals := make([]string, len(headers))
		for i, h := range headers {
			vals[i] = fmt.Sprint(r[columnName(h)])
		}
		fmt.Fprintln(out, strings.Join(vals, "\t"))
	}
	return nil
}

func columnName(header string) string {
	switch header {
	case "created":
		return "created_at_ms"
	case "started":
		return "started_at_ms"
	case "updated":
		return "updated_at_ms"
	case "schema":
		return "log_schema_version"
	case "first_seen":
		return "first_seen_at_ms"
	case "snapshot":
		return "snapshot_created_at_ms"
	default:
		return header
	}
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
