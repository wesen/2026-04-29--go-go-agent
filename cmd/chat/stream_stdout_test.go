package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/go-go-golems/geppetto/pkg/events"
)

func TestStdoutStreamSinkPrintsDeltasAndToolSummaries(t *testing.T) {
	var out bytes.Buffer
	sink := newStdoutStreamSink(&out, nil, stdoutStreamOptions{})

	if err := sink.PublishEvent(events.NewPartialCompletionEvent(events.EventMetadata{}, "Hello", "Hello")); err != nil {
		t.Fatalf("publish delta 1: %v", err)
	}
	if err := sink.PublishEvent(events.NewPartialCompletionEvent(events.EventMetadata{}, " world", "Hello world")); err != nil {
		t.Fatalf("publish delta 2: %v", err)
	}
	if err := sink.PublishEvent(events.NewToolCallEvent(events.EventMetadata{}, events.ToolCall{ID: "call-1", Name: "eval_js", Input: `{"code":"return 1"}`})); err != nil {
		t.Fatalf("publish tool call: %v", err)
	}
	if err := sink.PublishEvent(events.NewToolCallExecutionResultEvent(events.EventMetadata{}, events.ToolResult{ID: "call-1", Name: "eval_js", Result: `{"result":1}`})); err != nil {
		t.Fatalf("publish tool result: %v", err)
	}

	got := out.String()
	for _, want := range []string{
		"assistant: Hello world",
		"[tool eval_js call call-1]",
		"[tool eval_js done call-1]",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, got)
		}
	}
	if strings.Contains(got, "args:") || strings.Contains(got, "result:") {
		t.Fatalf("default streaming output should not include verbose args/results, got:\n%s", got)
	}
}

func TestStdoutStreamSinkCanPrintToolDetails(t *testing.T) {
	var out bytes.Buffer
	sink := newStdoutStreamSink(&out, nil, stdoutStreamOptions{ShowToolArgs: true, ShowToolResults: true, MaxPreviewChars: 20})

	_ = sink.PublishEvent(events.NewToolCallEvent(events.EventMetadata{}, events.ToolCall{ID: "call-1", Name: "eval_js", Input: `{"code":"const rows = inputDB.query('select * from docs'); return rows;"}`}))
	_ = sink.PublishEvent(events.NewToolCallExecutionResultEvent(events.EventMetadata{}, events.ToolResult{ID: "call-1", Name: "eval_js", Result: `{"result":[1,2,3,4,5,6,7,8,9]}`}))

	got := out.String()
	if !strings.Contains(got, "args:") {
		t.Fatalf("expected args preview, got:\n%s", got)
	}
	if !strings.Contains(got, "result:") {
		t.Fatalf("expected result preview, got:\n%s", got)
	}
	if !strings.Contains(got, "…") {
		t.Fatalf("expected truncated preview, got:\n%s", got)
	}
}

func TestStdoutStreamSinkWritesErrorsToErrOut(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	sink := newStdoutStreamSink(&out, &errOut, stdoutStreamOptions{})

	_ = sink.PublishEvent(events.NewPartialCompletionEvent(events.EventMetadata{}, "before", "before"))
	_ = sink.PublishEvent(events.NewErrorEvent(events.EventMetadata{}, errTest("boom")))

	if !strings.Contains(out.String(), "assistant: before") {
		t.Fatalf("expected partial output, got %q", out.String())
	}
	if !strings.Contains(errOut.String(), "[error] boom") {
		t.Fatalf("expected error output, got %q", errOut.String())
	}
}

type errTest string

func (e errTest) Error() string { return string(e) }
