package logdb

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-go-golems/geppetto/pkg/inference/tools/scopedjs"
	gojengine "github.com/go-go-golems/go-go-goja/engine"
	"github.com/go-go-golems/go-go-goja/pkg/replsession"
)

type EvalCorrelation struct {
	ToolCallID     string
	ChatSessionID  string
	TurnID         string
	EvalSessionID  string
	ReplCellID     int64
	CreatedAtMs    int64
	Code           string
	InputJSON      json.RawMessage
	EvalOutputJSON json.RawMessage
	ErrorText      string
}

type EvalTool struct {
	DB *DB
}

func (d *DB) EvalTool() *EvalTool {
	if d == nil {
		return nil
	}
	return &EvalTool{DB: d}
}

func (e *EvalTool) Eval(ctx context.Context, in scopedjs.EvalInput) (scopedjs.EvalOutput, error) {
	if e == nil || e.DB == nil || e.DB.ReplApp == nil {
		return scopedjs.EvalOutput{Error: "eval_js replapi backend is not configured"}, nil
	}
	started := time.Now().UTC()
	source := buildEvalCellSource(in)
	if err := e.prepareEvalGlobals(ctx, in); err != nil {
		return scopedjs.EvalOutput{Error: err.Error()}, nil
	}

	resp, evalErr := e.DB.ReplApp.Evaluate(ctx, e.DB.EvalSessionID, source)
	resultJSON, resultErr := resultJSONFromResponse(resp, evalErr)
	out := convertReplResponseToEvalOutput(resp, evalErr, resultJSON, resultErr, started)

	corr := EvalCorrelation{
		ToolCallID:     "",
		ChatSessionID:  e.DB.ChatSessionID,
		TurnID:         "",
		EvalSessionID:  e.DB.EvalSessionID,
		ReplCellID:     replCellID(resp),
		CreatedAtMs:    started.UnixMilli(),
		Code:           in.Code,
		InputJSON:      mustJSON(in.Input),
		EvalOutputJSON: mustJSON(out),
		ErrorText:      out.Error,
	}
	if err := e.DB.insertEvalToolCall(ctx, corr); err != nil && e.DB.Strict {
		return out, err
	}
	return out, nil
}

func buildEvalCellSource(in scopedjs.EvalInput) string {
	code := strings.TrimSpace(in.Code)
	if code == "" {
		return "undefined"
	}
	return code
}

func (e *EvalTool) prepareEvalGlobals(ctx context.Context, in scopedjs.EvalInput) error {
	input := in.Input
	if input == nil {
		input = map[string]any{}
	}
	return e.DB.ReplApp.WithRuntime(ctx, e.DB.EvalSessionID, func(rt *gojengine.Runtime) error {
		vm := rt.VM
		if err := vm.Set("input", input); err != nil {
			return err
		}
		global := vm.GlobalObject()
		if err := global.Set("window", global); err != nil {
			return err
		}
		if err := global.Set("global", global); err != nil {
			return err
		}
		return nil
	})
}

func resultJSONFromResponse(resp *replsession.EvaluateResponse, evalErr error) (string, error) {
	if evalErr != nil {
		return "", nil
	}
	if resp == nil || resp.Cell == nil {
		return "", fmt.Errorf("eval_js returned no repl cell")
	}
	if resp.Cell.Execution.ResultJSON == "" {
		return "", fmt.Errorf("replsession did not provide structured result JSON")
	}
	return resp.Cell.Execution.ResultJSON, nil
}

func convertReplResponseToEvalOutput(resp *replsession.EvaluateResponse, evalErr error, resultJSON string, resultErr error, started time.Time) scopedjs.EvalOutput {
	out := scopedjs.EvalOutput{DurationMs: time.Since(started).Milliseconds()}
	if resp != nil && resp.Cell != nil {
		out.DurationMs = resp.Cell.Execution.DurationMS
		out.Console = consoleFromRepl(resp.Cell.Execution.Console)
		if resp.Cell.Execution.Error != "" {
			out.Error = resp.Cell.Execution.Error
			return out
		}
	}
	if evalErr != nil {
		out.Error = evalErr.Error()
		return out
	}
	if resp == nil || resp.Cell == nil {
		out.Error = "eval_js returned no repl cell"
		return out
	}
	if resultErr != nil {
		out.Error = "eval_js result was not available: " + resultErr.Error()
		return out
	}
	var envelope struct {
		Result  any    `json:"result"`
		Error   string `json:"error,omitempty"`
		Kind    string `json:"kind,omitempty"`
		Preview string `json:"preview,omitempty"`
	}
	if decodeErr := json.Unmarshal([]byte(resultJSON), &envelope); decodeErr != nil {
		out.Error = "eval_js result was not valid JSON: " + decodeErr.Error()
		return out
	}
	if envelope.Error != "" {
		out.Error = envelope.Error
		return out
	}
	if envelope.Kind != "" && envelope.Result == nil {
		out.Result = map[string]any{"kind": envelope.Kind, "preview": envelope.Preview}
		return out
	}
	out.Result = envelope.Result
	return out
}

func consoleFromRepl(events []replsession.ConsoleEvent) []scopedjs.ConsoleLine {
	if len(events) == 0 {
		return nil
	}
	out := make([]scopedjs.ConsoleLine, 0, len(events))
	for _, event := range events {
		out = append(out, scopedjs.ConsoleLine{Level: event.Kind, Text: event.Message})
	}
	return out
}

func replCellID(resp *replsession.EvaluateResponse) int64 {
	if resp == nil || resp.Cell == nil {
		return 0
	}
	return int64(resp.Cell.ID)
}

func mustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`null`)
	}
	return b
}
