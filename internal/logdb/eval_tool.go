package logdb

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/go-go-golems/geppetto/pkg/inference/tools/scopedjs"
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
	source, err := buildEvalCellSource(in)
	if err != nil {
		return scopedjs.EvalOutput{Error: err.Error()}, nil
	}

	resp, evalErr := e.DB.ReplApp.Evaluate(ctx, e.DB.EvalSessionID, source)
	out := convertReplResponseToEvalOutput(resp, evalErr, started)

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

func buildEvalCellSource(in scopedjs.EvalInput) (string, error) {
	input := in.Input
	if input == nil {
		input = map[string]any{}
	}
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("marshal eval input: %w", err)
	}
	return fmt.Sprintf(`
const __chat_eval_input = %s;
const __chat_eval_result = await (async function(input) {
%s
})(__chat_eval_input);
JSON.stringify({ result: __chat_eval_result });
`, inputJSON, in.Code), nil
}

func convertReplResponseToEvalOutput(resp *replsession.EvaluateResponse, err error, started time.Time) scopedjs.EvalOutput {
	out := scopedjs.EvalOutput{DurationMs: time.Since(started).Milliseconds()}
	if resp != nil && resp.Cell != nil {
		out.DurationMs = resp.Cell.Execution.DurationMS
		out.Console = consoleFromRepl(resp.Cell.Execution.Console)
		if resp.Cell.Execution.Error != "" {
			out.Error = resp.Cell.Execution.Error
			return out
		}
	}
	if err != nil {
		out.Error = err.Error()
		return out
	}
	if resp == nil || resp.Cell == nil {
		out.Error = "eval_js returned no repl cell"
		return out
	}
	resultText := resp.Cell.Execution.Result
	var envelope struct {
		Result any `json:"result"`
	}
	if decodeErr := json.Unmarshal([]byte(resultText), &envelope); decodeErr != nil {
		if unquoted, unquoteErr := strconv.Unquote(resultText); unquoteErr == nil {
			if decodeErr = json.Unmarshal([]byte(unquoted), &envelope); decodeErr == nil {
				out.Result = envelope.Result
				return out
			}
		}
		out.Error = "eval_js result was not valid JSON: " + decodeErr.Error()
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
