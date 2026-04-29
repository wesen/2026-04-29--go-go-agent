package evaljs_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/go-go-golems/geppetto/pkg/inference/tools/scopedjs"
	"github.com/go-go-golems/go-go-agent/internal/evaljs"
)

type fakeEvalTool struct{}

func (fakeEvalTool) Eval(ctx context.Context, in evaljs.EvalInput) (evaljs.EvalOutput, error) {
	_ = ctx
	return evaljs.EvalOutput{Result: map[string]any{"code": in.Code}}, nil
}

func TestBuildRequiresReplAPIBackedEvalTool(t *testing.T) {
	_, err := evaljs.Build(context.Background(), evaljs.Scope{}, evaljs.Options{Timeout: time.Second})
	if err == nil || !strings.Contains(err.Error(), "replapi-backed EvalTool") {
		t.Fatalf("expected missing eval tool error, got %v", err)
	}
}

func TestBuildRegistersInjectedEvalToolContract(t *testing.T) {
	rt, err := evaljs.Build(context.Background(), evaljs.Scope{}, evaljs.Options{Timeout: time.Second}, evaljs.WithEvalTool(fakeEvalTool{}))
	if err != nil {
		t.Fatalf("build eval runtime: %v", err)
	}
	defer rt.Close()

	if rt.Tool == nil {
		t.Fatalf("expected injected eval tool")
	}
	out, err := rt.Tool.Eval(context.Background(), scopedjs.EvalInput{Code: `return 1;`})
	if err != nil {
		t.Fatalf("eval returned error: %v", err)
	}
	m, ok := out.Result.(map[string]any)
	if !ok || m["code"] != `return 1;` {
		t.Fatalf("unexpected result: %#v", out.Result)
	}
}

func TestNewEngineFactoryExposesOnlyInputAndOutputGlobalsInManifest(t *testing.T) {
	manifest := evaljs.Manifest()
	var globals []string
	for _, global := range manifest.Globals {
		globals = append(globals, global.Name)
	}
	joined := strings.Join(globals, ",")
	if !strings.Contains(joined, "inputDB") || !strings.Contains(joined, "outputDB") {
		t.Fatalf("expected inputDB and outputDB globals, got %v", globals)
	}
	if strings.Contains(joined, "log") {
		t.Fatalf("private log DB leaked into manifest globals: %v", globals)
	}
}
