package logdb_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/go-go-golems/geppetto/pkg/inference/tools/scopedjs"
	"github.com/go-go-golems/go-go-agent/internal/evaljs"
	"github.com/go-go-golems/go-go-agent/internal/helpdb"
	"github.com/go-go-golems/go-go-agent/internal/helpdocs"
	"github.com/go-go-golems/go-go-agent/internal/logdb"
)

func TestEvalToolExecutesThroughReplAPIAndPersistsCorrelation(t *testing.T) {
	ctx := context.Background()
	db := openTestLogDB(t, ctx)
	defer func() { _ = db.Close() }()

	out, err := db.EvalTool().Eval(ctx, scopedjs.EvalInput{Code: `
const rows = inputDB.query("SELECT slug, title FROM docs WHERE slug = ?", "eval-js-api");
console.log("rows", rows.length);
outputDB.exec("INSERT INTO notes(key, value) VALUES (?, ?)", "seen", rows[0].slug);
const result = {slug: rows[0].slug, notes: outputDB.query("SELECT key, value FROM notes")};
result
`})
	if err != nil {
		t.Fatalf("eval returned host error: %v", err)
	}
	if out.Error != "" {
		t.Fatalf("eval returned error payload: %s", out.Error)
	}
	m, ok := out.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T: %#v", out.Result, out.Result)
	}
	if got := m["slug"]; got != "eval-js-api" {
		t.Fatalf("expected slug eval-js-api, got %#v", got)
	}
	if len(out.Console) == 0 {
		t.Fatalf("expected console output")
	}

	var evalCount int
	if err := db.ReplStore.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM evaluations WHERE session_id = ?`, db.EvalSessionID).Scan(&evalCount); err != nil {
		t.Fatalf("query evaluations: %v", err)
	}
	if evalCount != 1 {
		t.Fatalf("expected one repl evaluation, got %d", evalCount)
	}

	var corrCount int
	if err := db.ReplStore.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM eval_tool_calls WHERE chat_session_id = ? AND eval_session_id = ?`, db.ChatSessionID, db.EvalSessionID).Scan(&corrCount); err != nil {
		t.Fatalf("query eval_tool_calls: %v", err)
	}
	if corrCount != 1 {
		t.Fatalf("expected one correlation row, got %d", corrCount)
	}
}

func TestEvalToolPersistsTopLevelDeclarationsAcrossCalls(t *testing.T) {
	ctx := context.Background()
	db := openTestLogDB(t, ctx)
	defer func() { _ = db.Close() }()

	first, err := db.EvalTool().Eval(ctx, scopedjs.EvalInput{Code: `
const base = 41;
function plusOne(x) { return x + 1; }
plusOne(base)
`})
	if err != nil {
		t.Fatalf("first eval returned host error: %v", err)
	}
	if first.Error != "" {
		t.Fatalf("first eval returned error payload: %s", first.Error)
	}
	if got := first.Result; got != float64(42) {
		t.Fatalf("expected first result 42, got %T %#v", got, got)
	}

	second, err := db.EvalTool().Eval(ctx, scopedjs.EvalInput{Code: `plusOne(base + 1)`})
	if err != nil {
		t.Fatalf("second eval returned host error: %v", err)
	}
	if second.Error != "" {
		t.Fatalf("second eval returned error payload: %s", second.Error)
	}
	if got := second.Result; got != float64(43) {
		t.Fatalf("expected second result 43, got %T %#v", got, got)
	}
}

func TestEvalToolSetsPerCallInputAndGlobalAliases(t *testing.T) {
	ctx := context.Background()
	db := openTestLogDB(t, ctx)
	defer func() { _ = db.Close() }()

	out, err := db.EvalTool().Eval(ctx, scopedjs.EvalInput{
		Code:  `window.answer = input.value * 2; global.answer === globalThis.answer && globalThis.answer`,
		Input: map[string]any{"value": 21},
	})
	if err != nil {
		t.Fatalf("eval returned host error: %v", err)
	}
	if out.Error != "" {
		t.Fatalf("eval returned error payload: %s", out.Error)
	}
	if got := out.Result; got != float64(42) {
		t.Fatalf("expected result 42, got %T %#v", got, got)
	}

	out, err = db.EvalTool().Eval(ctx, scopedjs.EvalInput{
		Code:  `input.value * 2`,
		Input: map[string]any{"value": 5},
	})
	if err != nil {
		t.Fatalf("second eval returned host error: %v", err)
	}
	if out.Error != "" {
		t.Fatalf("second eval returned error payload: %s", out.Error)
	}
	if got := out.Result; got != float64(10) {
		t.Fatalf("expected per-call input result 10, got %T %#v", got, got)
	}
}

func TestEvalToolReturnsMetadataForFunctionFinalExpression(t *testing.T) {
	ctx := context.Background()
	db := openTestLogDB(t, ctx)
	defer func() { _ = db.Close() }()

	out, err := db.EvalTool().Eval(ctx, scopedjs.EvalInput{Code: `function helper() { return 1; } helper`})
	if err != nil {
		t.Fatalf("eval returned host error: %v", err)
	}
	if out.Error != "" {
		t.Fatalf("eval returned error payload: %s", out.Error)
	}
	m, ok := out.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected metadata map result, got %T %#v", out.Result, out.Result)
	}
	if got := m["kind"]; got != "function" {
		t.Fatalf("expected kind=function, got %#v", got)
	}
	if got := m["preview"]; got == "" {
		t.Fatalf("expected preview, got %#v", got)
	}
}

func TestEvalToolReturnsTopLevelReturnErrorsAsPayload(t *testing.T) {
	ctx := context.Background()
	db := openTestLogDB(t, ctx)
	defer func() { _ = db.Close() }()

	out, err := db.EvalTool().Eval(ctx, scopedjs.EvalInput{Code: `return 42;`})
	if err != nil {
		t.Fatalf("eval returned host error: %v", err)
	}
	if out.Error == "" {
		t.Fatalf("expected top-level return error payload")
	}
}

func TestEvalToolReturnsReadOnlyErrorsAsPayload(t *testing.T) {
	ctx := context.Background()
	db := openTestLogDB(t, ctx)
	defer func() { _ = db.Close() }()

	out, err := db.EvalTool().Eval(ctx, scopedjs.EvalInput{Code: `inputDB.exec("DELETE FROM sections")`})
	if err != nil {
		t.Fatalf("eval returned host error: %v", err)
	}
	if out.Error == "" {
		t.Fatalf("expected read-only error payload")
	}

	var corrCount int
	if err := db.ReplStore.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM eval_tool_calls WHERE error_text <> ''`).Scan(&corrCount); err != nil {
		t.Fatalf("query error correlations: %v", err)
	}
	if corrCount != 1 {
		t.Fatalf("expected one error correlation row, got %d", corrCount)
	}
}

func TestEvalToolReturnsSerializationErrorsAsPayload(t *testing.T) {
	ctx := context.Background()
	db := openTestLogDB(t, ctx)
	defer func() { _ = db.Close() }()

	out, err := db.EvalTool().Eval(ctx, scopedjs.EvalInput{Code: `const x = {}; x.self = x; x`})
	if err != nil {
		t.Fatalf("eval returned host error: %v", err)
	}
	if out.Error == "" {
		t.Fatalf("expected serialization error payload")
	}

	var corrCount int
	if err := db.ReplStore.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM eval_tool_calls WHERE error_text <> ''`).Scan(&corrCount); err != nil {
		t.Fatalf("query error correlations: %v", err)
	}
	if corrCount != 1 {
		t.Fatalf("expected one serialization error correlation row, got %d", corrCount)
	}
}

func openTestLogDB(t *testing.T, ctx context.Context) *logdb.DB {
	t.Helper()
	input, err := helpdb.PrepareInputDB(ctx, helpdb.InputDBConfig{HelpFS: helpdocs.FS, HelpDir: helpdocs.Dir})
	if err != nil {
		t.Fatalf("prepare input: %v", err)
	}
	t.Cleanup(func() { _ = input.Close() })
	output, err := helpdb.PrepareOutputDB(ctx, "")
	if err != nil {
		t.Fatalf("prepare output: %v", err)
	}
	t.Cleanup(func() { _ = output.Close() })
	factory, err := evaljs.NewEngineFactory(evaljs.Scope{InputDB: input.DB, OutputDB: output.DB})
	if err != nil {
		t.Fatalf("build factory: %v", err)
	}
	db, err := logdb.Open(ctx, logdb.Config{Path: filepath.Join(t.TempDir(), "chat-log.sqlite"), ChatSessionID: "chat-test"}, factory)
	if err != nil {
		t.Fatalf("open log db: %v", err)
	}
	return db
}
