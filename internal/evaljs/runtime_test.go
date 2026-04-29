package evaljs_test

import (
	"context"
	"testing"
	"time"

	"github.com/go-go-golems/geppetto/pkg/inference/tools/scopedjs"
	"github.com/go-go-golems/go-go-agent/internal/evaljs"
	"github.com/go-go-golems/go-go-agent/internal/helpdb"
	"github.com/go-go-golems/go-go-agent/internal/helpdocs"
)

func TestEvalJSCanQueryEmbeddedHelpAndWriteOutput(t *testing.T) {
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

	rt, err := evaljs.Build(ctx, evaljs.Scope{InputDB: input.DB, OutputDB: output.DB}, evaljs.Options{Timeout: time.Second})
	if err != nil {
		t.Fatalf("build eval runtime: %v", err)
	}
	defer rt.Close()

	out, err := rt.Handle.Executor.RunEval(ctx, scopedjs.EvalInput{Code: `
const rows = inputDB.query("SELECT slug, title FROM docs WHERE slug = ?", "eval-js-api");
outputDB.exec("INSERT INTO notes(key, value) VALUES (?, ?)", "seen", rows[0].slug);
return {slug: rows[0].slug, notes: outputDB.query("SELECT key, value FROM notes")};
`}, scopedjs.DefaultEvalOptions())
	if err != nil {
		t.Fatalf("RunEval returned error: %v", err)
	}
	if out.Error != "" {
		t.Fatalf("eval error: %s", out.Error)
	}
	m, ok := out.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T: %#v", out.Result, out.Result)
	}
	if got := m["slug"]; got != "eval-js-api" {
		t.Fatalf("expected slug eval-js-api, got %#v", got)
	}
}

func TestInputDBRejectsWrites(t *testing.T) {
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

	rt, err := evaljs.Build(ctx, evaljs.Scope{InputDB: input.DB, OutputDB: output.DB}, evaljs.Options{Timeout: time.Second})
	if err != nil {
		t.Fatalf("build eval runtime: %v", err)
	}
	defer rt.Close()

	out, err := rt.Handle.Executor.RunEval(ctx, scopedjs.EvalInput{Code: `return inputDB.exec("DELETE FROM sections");`}, scopedjs.DefaultEvalOptions())
	if err != nil {
		t.Fatalf("RunEval returned error: %v", err)
	}
	if out.Error == "" {
		t.Fatalf("expected read-only eval error")
	}
}
