package logdb_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/go-go-golems/geppetto/pkg/inference/tools/scopedjs"
)

func TestEvalRuntimeDoesNotExposePrivateLogTables(t *testing.T) {
	ctx := context.Background()
	db := openTestLogDB(t, ctx)
	defer func() { _ = db.Close() }()

	out, err := db.EvalTool().Eval(ctx, scopedjs.EvalInput{Code: `
const tables = {
  inputTables: inputDB.schema().tables,
  outputTables: outputDB.schema().tables
};
tables
`})
	if err != nil {
		t.Fatalf("eval returned host error: %v", err)
	}
	if out.Error != "" {
		t.Fatalf("eval returned error payload: %s", out.Error)
	}
	blob := strings.ToLower(string(mustMarshalForTest(t, out.Result)))
	for _, forbidden := range []string{"chat_log_sessions", "eval_tool_calls", "turn_block_membership", "repldb_meta"} {
		if strings.Contains(blob, forbidden) {
			t.Fatalf("private table %s leaked through eval_js result: %s", forbidden, blob)
		}
	}
}

func mustMarshalForTest(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	return b
}
