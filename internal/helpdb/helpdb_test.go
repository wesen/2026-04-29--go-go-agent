package helpdb_test

import (
	"context"
	"testing"

	"github.com/go-go-golems/go-go-agent/internal/helpdb"
	"github.com/go-go-golems/go-go-agent/internal/helpdocs"
)

func TestPrepareInputDBLoadsEmbeddedHelp(t *testing.T) {
	ctx := context.Background()
	prepared, err := helpdb.PrepareInputDB(ctx, helpdb.InputDBConfig{
		HelpFS:  helpdocs.FS,
		HelpDir: helpdocs.Dir,
	})
	if err != nil {
		t.Fatalf("PrepareInputDB failed: %v", err)
	}
	defer func() { _ = prepared.Close() }()

	rows, err := prepared.DB.QueryContext(ctx, `SELECT slug, title FROM docs ORDER BY slug`)
	if err != nil {
		t.Fatalf("query docs view failed: %v", err)
	}
	defer func() { _ = rows.Close() }()

	slugs := map[string]bool{}
	for rows.Next() {
		var slug, title string
		if err := rows.Scan(&slug, &title); err != nil {
			t.Fatalf("scan row: %v", err)
		}
		if title == "" {
			t.Fatalf("empty title for slug %q", slug)
		}
		slugs[slug] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows error: %v", err)
	}
	for _, want := range []string{"eval-js-api", "database-globals-api", "chat-repl-user-guide"} {
		if !slugs[want] {
			t.Fatalf("missing embedded help slug %q in %#v", want, slugs)
		}
	}
}

func TestPrepareOutputDBCreatesNotes(t *testing.T) {
	ctx := context.Background()
	prepared, err := helpdb.PrepareOutputDB(ctx, "")
	if err != nil {
		t.Fatalf("PrepareOutputDB failed: %v", err)
	}
	defer func() { _ = prepared.Close() }()

	if _, err := prepared.DB.ExecContext(ctx, `INSERT INTO notes(key, value) VALUES (?, ?)`, "k", "v"); err != nil {
		t.Fatalf("insert note: %v", err)
	}
	var count int
	if err := prepared.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM notes`).Scan(&count); err != nil {
		t.Fatalf("count notes: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 note, got %d", count)
	}
}
