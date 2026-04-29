package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/go-go-golems/geppetto/pkg/events"
)

type stdoutStreamOptions struct {
	ShowToolArgs    bool
	ShowToolResults bool
	MaxPreviewChars int
}

type stdoutStreamSink struct {
	mu sync.Mutex

	out    io.Writer
	errOut io.Writer
	opts   stdoutStreamOptions

	assistantStarted bool
	thinkingStarted  bool
	lastWasDelta     bool
}

func newStdoutStreamSink(out io.Writer, errOut io.Writer, opts stdoutStreamOptions) *stdoutStreamSink {
	if out == nil {
		out = io.Discard
	}
	if errOut == nil {
		errOut = out
	}
	if opts.MaxPreviewChars <= 0 {
		opts.MaxPreviewChars = 500
	}
	return &stdoutStreamSink{out: out, errOut: errOut, opts: opts}
}

func (s *stdoutStreamSink) PublishEvent(event events.Event) error {
	if s == nil || event == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	switch e := event.(type) {
	case *events.EventPartialCompletion:
		return s.writeDelta(e.Delta)
	case *events.EventThinkingPartial:
		return s.writeThinkingDelta(e.Delta)
	case *events.EventToolCall:
		return s.writeToolCall(e.ToolCall)
	case *events.EventToolCallExecute:
		return s.writeToolExecute(e.ToolCall)
	case *events.EventToolCallExecutionResult:
		return s.writeToolExecutionResult(e.ToolResult)
	case *events.EventToolResult:
		return s.writeToolResult(e.ToolResult)
	case *events.EventError:
		return s.writeError(e.ErrorString)
	default:
		return nil
	}
}

func (s *stdoutStreamSink) writeDelta(delta string) error {
	if delta == "" {
		return nil
	}
	if !s.assistantStarted {
		if _, err := fmt.Fprint(s.out, "\nassistant: "); err != nil {
			return err
		}
		s.assistantStarted = true
	}
	_, err := fmt.Fprint(s.out, delta)
	s.lastWasDelta = true
	return err
}

func (s *stdoutStreamSink) writeThinkingDelta(delta string) error {
	if delta == "" {
		return nil
	}
	if !s.thinkingStarted {
		if err := s.ensureLineBreak(); err != nil {
			return err
		}
		if _, err := fmt.Fprint(s.out, "\nthinking: "); err != nil {
			return err
		}
		s.thinkingStarted = true
	}
	_, err := fmt.Fprint(s.out, delta)
	s.lastWasDelta = true
	return err
}

func (s *stdoutStreamSink) writeToolCall(tc events.ToolCall) error {
	if err := s.ensureLineBreak(); err != nil {
		return err
	}
	name := defaultString(tc.Name, "tool")
	id := defaultString(tc.ID, "unknown")
	if _, err := fmt.Fprintf(s.out, "\n[tool %s call %s]\n", name, id); err != nil {
		return err
	}
	if s.opts.ShowToolArgs && strings.TrimSpace(tc.Input) != "" {
		if _, err := fmt.Fprint(s.out, formatToolInput(name, tc.Input, s.opts.MaxPreviewChars)); err != nil {
			return err
		}
	}
	s.lastWasDelta = false
	return nil
}

func (s *stdoutStreamSink) writeToolExecute(tc events.ToolCall) error {
	if err := s.ensureLineBreak(); err != nil {
		return err
	}
	name := defaultString(tc.Name, "tool")
	id := defaultString(tc.ID, "unknown")
	_, err := fmt.Fprintf(s.out, "[tool %s running %s]\n", name, id)
	s.lastWasDelta = false
	return err
}

func (s *stdoutStreamSink) writeToolExecutionResult(tr events.ToolResult) error {
	if err := s.ensureLineBreak(); err != nil {
		return err
	}
	name := defaultString(tr.Name, "tool")
	id := defaultString(tr.ID, "unknown")
	if _, err := fmt.Fprintf(s.out, "[tool %s done %s]\n", name, id); err != nil {
		return err
	}
	if s.opts.ShowToolResults && strings.TrimSpace(tr.Result) != "" {
		if _, err := fmt.Fprint(s.out, formatToolResult(tr.Result, s.opts.MaxPreviewChars)); err != nil {
			return err
		}
	}
	s.lastWasDelta = false
	return nil
}

func (s *stdoutStreamSink) writeToolResult(tr events.ToolResult) error {
	if err := s.ensureLineBreak(); err != nil {
		return err
	}
	name := defaultString(tr.Name, "tool")
	id := defaultString(tr.ID, "unknown")
	if _, err := fmt.Fprintf(s.out, "[tool %s result %s]\n", name, id); err != nil {
		return err
	}
	if s.opts.ShowToolResults && strings.TrimSpace(tr.Result) != "" {
		if _, err := fmt.Fprint(s.out, formatToolResult(tr.Result, s.opts.MaxPreviewChars)); err != nil {
			return err
		}
	}
	s.lastWasDelta = false
	return nil
}

func (s *stdoutStreamSink) writeError(message string) error {
	if err := s.ensureLineBreak(); err != nil {
		return err
	}
	_, err := fmt.Fprintf(s.errOut, "\n[error] %s\n", message)
	s.lastWasDelta = false
	return err
}

func (s *stdoutStreamSink) ensureLineBreak() error {
	if s.lastWasDelta {
		_, err := fmt.Fprintln(s.out)
		s.lastWasDelta = false
		return err
	}
	return nil
}

func formatToolInput(toolName string, input string, limit int) string {
	if strings.EqualFold(toolName, "eval_js") {
		var payload struct {
			Code  string         `json:"code"`
			Input map[string]any `json:"input"`
		}
		if err := json.Unmarshal([]byte(input), &payload); err == nil && strings.TrimSpace(payload.Code) != "" {
			var b strings.Builder
			b.WriteString("code:\n")
			b.WriteString(truncateMultiline(payload.Code, limit))
			b.WriteString("\n")
			if len(payload.Input) > 0 {
				if inputJSON, err := json.MarshalIndent(payload.Input, "", "  "); err == nil {
					b.WriteString("input:\n")
					b.WriteString(truncateMultiline(string(inputJSON), limit))
					b.WriteString("\n")
				}
			}
			return b.String()
		}
	}
	return fmt.Sprintf("args: %s\n", truncateOneLine(input, limit))
}

func formatToolResult(result string, limit int) string {
	var formatted any
	if err := json.Unmarshal([]byte(result), &formatted); err == nil {
		if b, err := json.MarshalIndent(formatted, "", "  "); err == nil {
			return "result:\n" + truncateMultiline(string(b), limit) + "\n"
		}
	}
	return fmt.Sprintf("result: %s\n", truncateOneLine(result, limit))
}

func defaultString(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func truncateOneLine(value string, limit int) string {
	value = strings.Join(strings.Fields(value), " ")
	if limit <= 0 || len(value) <= limit {
		return value
	}
	if limit <= 1 {
		return "…"
	}
	return value[:limit-1] + "…"
}

func truncateMultiline(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || len(value) <= limit {
		return value
	}
	if limit <= 1 {
		return "…"
	}
	return value[:limit-1] + "…"
}
