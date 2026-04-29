package main

import (
	"bytes"
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-go-golems/glazed/pkg/cmds/logging"
	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/cobra"
)

func TestRootHelpSeparatesRunFlags(t *testing.T) {
	cmd := newTestRootCommand(t)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute help: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "run") || !strings.Contains(got, "inspect") {
		t.Fatalf("expected root help to list run and inspect commands, got:\n%s", got)
	}
	if strings.Contains(got, "--stream") {
		t.Fatalf("root help should not include run-only --stream flag, got:\n%s", got)
	}
}

func TestRunHelpContainsRunFlags(t *testing.T) {
	cmd := newTestRootCommand(t)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"run", "--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute run help: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "--stream") || !strings.Contains(got, "--log-db") {
		t.Fatalf("run help should include run flags, got:\n%s", got)
	}
}

func newTestRootCommand(t *testing.T) *cobra.Command {
	t.Helper()
	cmd := newRootCommand(context.Background())
	if err := registerGlazedCommands(context.Background(), cmd); err != nil {
		t.Fatalf("register commands: %v", err)
	}
	if err := logging.AddLoggingSectionToRootCommand(cmd, "chat"); err != nil {
		t.Fatalf("add logging section: %v", err)
	}
	return cmd
}

func TestInspectSchemaCommandPrintsTables(t *testing.T) {
	path := filepath.Join(t.TempDir(), "inspect.sqlite")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE chat_log_sessions(chat_session_id TEXT); INSERT INTO chat_log_sessions(chat_session_id) VALUES ('chat-1');`); err != nil {
		t.Fatalf("seed db: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}

	cmd, err := newInspectQueryCommand("schema", "List SQLite tables and row counts", "schema")
	if err != nil {
		t.Fatalf("new inspect command: %v", err)
	}
	if err := withInspectDB(context.Background(), inspectSettings{LogDBPath: path}, func(db *sql.DB) error {
		rows, _, err := cmd.inspectRows(context.Background(), db, inspectSettings{LogDBPath: path})
		if err != nil {
			return err
		}
		if len(rows) != 1 || rows[0]["table"] != "chat_log_sessions" || rows[0]["rows"] != int64(1) {
			t.Fatalf("expected schema row count, got %#v", rows)
		}
		return nil
	}); err != nil {
		t.Fatalf("inspect schema: %v", err)
	}
}
