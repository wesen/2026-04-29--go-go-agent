package main

import (
	"bytes"
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestRootHelpSeparatesRunFlags(t *testing.T) {
	cmd := newRootCommand(context.Background())
	cmd.AddCommand(newRunCommand(context.Background()))
	cmd.AddCommand(newInspectCommand())
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
	cmd := newRootCommand(context.Background())
	cmd.AddCommand(newRunCommand(context.Background()))
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

	cmd := newInspectSchemaCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--log-db", path})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute inspect schema: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "chat_log_sessions") || !strings.Contains(got, "1") {
		t.Fatalf("expected schema output with table count, got:\n%s", got)
	}
}
