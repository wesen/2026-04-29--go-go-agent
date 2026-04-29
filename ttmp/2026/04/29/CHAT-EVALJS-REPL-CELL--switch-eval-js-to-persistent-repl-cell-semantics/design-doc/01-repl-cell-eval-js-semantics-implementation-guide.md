---
Title: REPL-cell eval_js semantics implementation guide
Ticket: CHAT-EVALJS-REPL-CELL
Status: active
Topics:
    - chat
    - geppetto
    - goja
    - llm-tools
    - sqlite
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: internal/logdb/eval_tool.go
      Note: Current eval_js adapter that wraps model code in an async function body and therefore loses top-level declarations between calls.
    - Path: internal/evaljs/runtime.go
      Note: Tool registration, eval_js tool description, runtime globals, and starter snippets that must change from return-based snippets to REPL-cell examples.
    - Path: /home/manuel/code/wesen/corporate-headquarters/go-go-goja/pkg/replsession/evaluate.go
      Note: replsession evaluation pipeline, instrumented execution, binding capture, raw execution, and current preview-only result reporting.
    - Path: /home/manuel/code/wesen/corporate-headquarters/go-go-goja/pkg/replsession/rewrite.go
      Note: Source rewriting that captures top-level declarations and final expression values for REPL-like cells.
    - Path: /home/manuel/code/wesen/corporate-headquarters/go-go-goja/pkg/replsession/types.go
      Note: Report types where an exact structured last-value field or envelope can be added if needed.
    - Path: /home/manuel/code/wesen/corporate-headquarters/go-go-goja/pkg/replapi/app.go
      Note: Public app facade used by go-go-agent to evaluate cells and access live runtimes.
    - Path: /home/manuel/code/wesen/corporate-headquarters/go-go-goja/pkg/replapi/config.go
      Note: Persistent profile and session option defaults that determine instrumented REPL behavior.
    - Path: /home/manuel/code/wesen/corporate-headquarters/geppetto/pkg/inference/tools/scopedjs/schema.go
      Note: Current eval_js input/output schema shared with the Geppetto scoped JavaScript tool wrapper.
ExternalSources: []
Summary: Detailed intern guide for switching eval_js from function-body calls to persistent replsession cell semantics.
LastUpdated: 2026-04-29T11:41:51.987573588-04:00
WhatFor: "Guide an intern through the design and implementation of REPL-cell eval_js semantics, including necessary go-go-goja updates."
WhenToUse: "Use before changing eval_js persistence semantics, replsession last-value reporting, or tool instructions for JavaScript snippets."
---

# REPL-cell `eval_js` semantics implementation guide

## Executive Summary

We want the chat agent's `eval_js` tool to behave like a real persistent JavaScript REPL cell, not like a one-shot function call. Today, each tool call is wrapped in an async function body so the model can write snippets with `return ...`. That makes short scripts convenient, but it also means declarations like `const helper = ...` or `function helper(...) { ... }` are local to one tool call and disappear before the next call.

The target behavior for this ticket is **Option B / REPL-cell mode**:

- The submitted JavaScript is evaluated as a real `replsession` cell.
- Top-level declarations persist across tool calls.
- The tool result is the value of the final expression, like a notebook or REPL.
- Top-level `return` is no longer the normal way to produce output.
- `globalThis` is the canonical global object.
- `window` and `global` may be provided as aliases of `globalThis` for convenience, while making clear that they do **not** imply a DOM or full Node.js environment.
- If `go-go-goja` only exposes display previews for the last value, update `go-go-goja` so `replsession` can expose an exact structured JSON envelope for the final expression without wrapping user code in another function.

The most important implementation rule is:

> Do not put user code inside another JavaScript function if we want replsession to persist top-level declarations.

Instead, set per-call host globals such as `input` before evaluation, then pass the user source directly to `replapi.Evaluate`.

## Problem Statement

### The observed bug

A model/user can call `eval_js` once with code like this:

```js
function normalizeSlug(s) {
  return String(s).toLowerCase().replace(/\s+/g, "-");
}

return normalizeSlug("Chat REPL User Guide");
```

That call works today because our adapter wraps the submitted code in an async function:

```js
const __chat_eval_input = {...};
const __chat_eval_result = await (async function(input) {
  function normalizeSlug(s) {
    return String(s).toLowerCase().replace(/\s+/g, "-");
  }

  return normalizeSlug("Chat REPL User Guide");
})(__chat_eval_input);

globalThis.__chat_eval_last_json = JSON.stringify({ result: __chat_eval_result });
globalThis.__chat_eval_last_json;
```

But in the next tool call:

```js
return normalizeSlug("Second Entry");
```

`normalizeSlug` is not defined. It was a local function inside the previous async function call. It was never a top-level `replsession` binding.

### Why this is surprising

The tool description currently says that calls execute in a persistent `replapi/replsession` session. That is true for the underlying runtime and evaluation history, but it is misleading for JavaScript declarations because the wrapper changes the user's lexical scope.

An LLM reasonably expects this sequence to work:

```js
function add(a, b) {
  return a + b;
}

add(1, 2)
```

then later:

```js
add(10, 20)
```

That is exactly how a REPL or notebook works. It is also how `go-go-goja/pkg/replsession` is designed to work when it receives the source as a real top-level cell.

### Root cause

There are two separate persistence layers:

1. **Runtime/session persistence**: the same goja runtime and replsession store are reused across calls.
2. **Binding persistence**: top-level declarations are detected, captured, and mirrored back to the global object by `replsession`.

The first layer is active today. The second layer is bypassed for user declarations because `internal/logdb/eval_tool.go` places the user's code inside an async function body.

## Mental model for a new intern

Think of the system as four nested boxes:

```text
+---------------------------------------------------------------+
| chat REPL / LLM agent                                          |
|                                                               |
|  asks model, receives tool calls, streams output               |
|                                                               |
|  +---------------------------------------------------------+   |
|  | eval_js tool adapter                                    |   |
|  |                                                         |   |
|  | converts Geppetto scopedjs input/output into replapi    |   |
|  | calls and logs correlations in the private DB           |   |
|  |                                                         |   |
|  |  +--------------------------------------------------+   |   |
|  |  | go-go-goja replapi / replsession                 |   |   |
|  |  |                                                  |   |   |
|  |  | owns persistent sessions, cells, bindings,       |   |   |
|  |  | history, restore, and binding capture            |   |   |
|  |  |                                                  |   |   |
|  |  |  +-------------------------------------------+   |   |   |
|  |  |  | goja JavaScript runtime                    |   |   |   |
|  |  |  |                                           |   |   |   |
|  |  |  | actual JS globals, functions, objects, DB |   |   |   |
|  |  |  | facades, globalThis/window/global aliases  |   |   |   |
|  |  |  +-------------------------------------------+   |   |   |
|  |  +--------------------------------------------------+   |   |
|  +---------------------------------------------------------+   |
+---------------------------------------------------------------+
```

The intern's job is mostly in the middle two boxes:

- In `go-go-agent`, stop converting the model's JavaScript into a function body.
- In `go-go-goja`, expose the exact final expression value if the current report only contains a display preview.

## Current architecture

### Chat command and tool registration

The chat binary wires the agent runtime in `cmd/chat/main.go`. The relevant path is:

```text
cmd/chat/main.go
  -> creates private log DB
  -> creates replapi-backed EvalTool
  -> evaljs.Build(..., evaljs.WithEvalTool(logDB.EvalTool()))
  -> runner.Start(...)
```

`internal/evaljs/runtime.go` defines the user-facing `eval_js` tool metadata. It creates a Geppetto tool from a Go function:

```go
def, err := geptools.NewToolFromFunc(
    r.Spec.Tool.Name,
    scopedjs.BuildDescription(...),
    func(ctx context.Context, in EvalInput) (EvalOutput, error) {
        return r.Tool.Eval(ctx, in)
    },
)
```

The metadata currently teaches the model function-body examples:

```js
const rows = inputDB.query("SELECT slug, title, short FROM docs ORDER BY title LIMIT 10"); return rows;
```

Those examples must change because top-level `return` will be invalid in REPL-cell mode.

### Current eval tool adapter

`internal/logdb/eval_tool.go` is the core adapter:

```go
func (e *EvalTool) Eval(ctx context.Context, in scopedjs.EvalInput) (scopedjs.EvalOutput, error) {
    source, err := buildEvalCellSource(in)
    resp, evalErr := e.DB.ReplApp.Evaluate(ctx, e.DB.EvalSessionID, source)
    resultJSON, resultErr := e.readLastResultJSON(ctx, evalErr)
    out := convertReplResponseToEvalOutput(resp, evalErr, resultJSON, resultErr, started)
    ... persist correlation row ...
    return out, nil
}
```

The current `buildEvalCellSource` is the source of the scoping bug:

```go
func buildEvalCellSource(in scopedjs.EvalInput) (string, error) {
    inputJSON, err := json.Marshal(input)
    return fmt.Sprintf(`
const __chat_eval_input = %s;
const __chat_eval_result = await (async function(input) {
%s
})(__chat_eval_input);
globalThis.__chat_eval_last_json = JSON.stringify({ result: __chat_eval_result });
globalThis.__chat_eval_last_json;
`, inputJSON, in.Code), nil
}
```

This has three behaviors:

- It provides `input` as a function parameter.
- It lets code use `return`.
- It hides all declarations inside a local function scope.

REPL-cell mode must remove this wrapper.

### go-go-goja replsession behavior

The relevant files in `go-go-goja` are:

- `/home/manuel/code/wesen/corporate-headquarters/go-go-goja/pkg/replsession/evaluate.go`
- `/home/manuel/code/wesen/corporate-headquarters/go-go-goja/pkg/replsession/rewrite.go`
- `/home/manuel/code/wesen/corporate-headquarters/go-go-goja/pkg/replsession/types.go`
- `/home/manuel/code/wesen/corporate-headquarters/go-go-goja/pkg/replapi/app.go`
- `/home/manuel/code/wesen/corporate-headquarters/go-go-goja/pkg/replapi/config.go`

In persistent/interactive mode, `replsession` uses instrumented execution. The high-level path is:

```text
replapi.App.Evaluate(sessionID, source)
  -> ensure live session / restore if needed
  -> replsession.Service.Evaluate(sessionID, source)
  -> analyze source with jsparse
  -> build rewrite report
  -> execute transformed source
  -> persist captured bindings and evaluation history
  -> return EvaluateResponse
```

`rewrite.go` is the key file. It builds an async IIFE around the whole cell, but this is different from our current adapter wrapper because `replsession` first analyzes the original top-level source and deliberately captures top-level names.

Simplified pseudocode for `buildRewrite`:

```go
func buildRewrite(source string, analysis *AnalysisResult, cellID int) RewriteReport {
    declaredNames := declaredNamesFromResult(analysis)
    helperLast := fmt.Sprintf("__ggg_repl_last_%d__", cellID)
    helperBindings := fmt.Sprintf("__ggg_repl_bindings_%d__", cellID)

    if last statement is expression {
        replace final expression with:
            __ggg_repl_last_N__ = (originalFinalExpression);
    }

    return async IIFE source:
        (async function () {
          let __ggg_repl_last_N__;
          <possibly modified user body>
          return {
            "__ggg_repl_bindings_N__": {
              "name1": name1,
              "name2": name2,
            },
            "__ggg_repl_last_N__": __ggg_repl_last_N__
          };
        })()
}
```

Then `evaluate.go` calls `persistWrappedReturn`:

```go
func (s *sessionState) persistWrappedReturn(...) ([]string, string, bool, error) {
    obj := value.ToObject(vm)
    bindingsObj := obj.Get(bindingsKey).ToObject(vm)
    for _, name := range bindingsObj.Keys() {
        vm.Set(name, bindingsObj.Get(name))
    }
    return persistedNames, gojaValuePreview(lastValue, vm), helperError, nil
}
```

That `vm.Set(name, value)` step is why real top-level cells persist functions and constants across later cells.

## Desired user-facing semantics

### Before

The current model-facing style is:

```js
const rows = inputDB.query("SELECT slug, title FROM docs LIMIT 5");
return rows;
```

### After

The new model-facing style should be:

```js
const rows = inputDB.query("SELECT slug, title FROM docs LIMIT 5");
rows
```

Or:

```js
function titles(rows) {
  return rows.map(r => r.title);
}

const rows = inputDB.query("SELECT title FROM docs LIMIT 5");
titles(rows)
```

Then later:

```js
titles(inputDB.query("SELECT title FROM docs LIMIT 3"))
```

The second call should work because `titles` was a top-level function declaration in the previous cell.

### Rules for the model/tool description

The tool description should say:

- Execute JavaScript as a persistent REPL cell.
- Top-level `const`, `let`, `var`, `function`, and `class` declarations persist across later calls.
- The tool result is the value of the final expression.
- Do not use top-level `return`; use a final expression instead.
- `input` is a per-call object supplied by the host.
- `inputDB` is the read-only embedded help database.
- `outputDB` is the writable scratch database.
- `globalThis` is the canonical global object.
- `window` and `global` are aliases of `globalThis` for convenience only; there is no browser DOM or full Node.js runtime unless separately provided.

Good example:

```js
const matches = inputDB.query(
  "SELECT slug, title FROM docs WHERE content LIKE ? LIMIT 5",
  `%${input.term}%`
);

matches.map(row => ({ slug: row.slug, title: row.title }))
```

Bad example:

```js
return inputDB.query("SELECT slug, title FROM docs LIMIT 5");
```

Why it is bad: top-level `return` is invalid in a real REPL cell.

## Proposed Solution

### One-sentence solution

Evaluate `eval_js` code as the user's real `replsession` cell, set per-call globals through the host runtime before evaluation, and extend `go-go-goja` so the exact final expression value is available as JSON without wrapping the user source.

### High-level flow after the change

```mermaid
sequenceDiagram
    participant LLM as LLM / Geppetto runner
    participant Tool as go-go-agent EvalTool
    participant App as go-go-goja replapi.App
    participant Sess as replsession.Service
    participant VM as goja Runtime
    participant DB as private SQLite log DB

    LLM->>Tool: eval_js({ code, input })
    Tool->>App: WithRuntime(sessionID, set input/window/global aliases)
    App->>VM: globalThis.input = input; window/global aliases
    Tool->>App: Evaluate(sessionID, code)
    App->>Sess: Evaluate persistent cell
    Sess->>Sess: analyze top-level declarations
    Sess->>Sess: rewrite final expression capture
    Sess->>VM: run transformed cell
    VM-->>Sess: bindings + final value
    Sess->>VM: vm.Set(name, captured binding)
    Sess-->>App: EvaluateResponse with exact JSON envelope
    App-->>Tool: response
    Tool->>DB: insert eval_tool_calls correlation
    Tool-->>LLM: EvalOutput{ result, console, error, durationMs }
```

### Important implementation principle

Do this:

```go
// Host setup before evaluating the cell.
e.DB.ReplApp.WithRuntime(ctx, sessionID, func(rt *engine.Runtime) error {
    rt.VM.Set("input", in.Input)
    ensureGlobalAliases(rt.VM)
    return nil
})

// Evaluate the user code directly as a replsession cell.
resp, err := e.DB.ReplApp.Evaluate(ctx, sessionID, in.Code)
```

Do **not** do this:

```go
// This breaks top-level declaration persistence.
source := fmt.Sprintf(`
await (async function(input) {
%s
})(input)
`, in.Code)
```

## Design Decisions

### Decision 1: `eval_js` becomes REPL-cell mode, not function-body mode

**Decision:** Treat the submitted `code` as a complete JavaScript REPL cell.

**Rationale:** The underlying `go-go-goja` `replsession` is already designed for persistent top-level declarations, binding reports, function source mapping, durable session history, and last-expression capture. We should use those features instead of fighting them with a wrapper.

**Consequence:** Models must stop using top-level `return` in examples.

### Decision 2: final expression is the result

**Decision:** The value of the final expression is the `eval_js` result.

**Rationale:** This matches REPLs, notebooks, browser devtools consoles, and `replsession`'s existing `CaptureLastExpression` behavior.

Example:

```js
const x = 20;
const y = 22;
x + y
```

Expected tool result:

```json
42
```

### Decision 3: `globalThis` is canonical

**Decision:** Document `globalThis` as the canonical global object.

**Rationale:** `globalThis` is the standard JavaScript global reference across browsers, Node.js, workers, and embedded runtimes.

### Decision 4: provide `window` and `global` aliases if cheap and safe

**Decision:** Provide compatibility aliases:

```js
globalThis.window = globalThis;
globalThis.global = globalThis;
```

**Rationale:** LLMs often generate browser-style `window.foo = ...` or Node-style `global.foo = ...`. Aliases reduce friction.

**Caveat:** The tool docs must say these are aliases only. They do not mean `document`, DOM APIs, Node's `process`, or filesystem APIs exist.

### Decision 5: set `input` as a host global, not with source prelude declarations

**Decision:** Set `input` through `WithRuntime` before evaluating the user's source.

**Rationale:** If we prepend source like `const input = ...;`, then a later cell would collide with the same `const input`. If we prepend `globalThis.input = ...;`, the history is noisier and source reports include host-generated code. Using `WithRuntime` keeps the user's cell source clean.

Suggested helper:

```go
func (e *EvalTool) prepareEvalGlobals(ctx context.Context, input map[string]any) error {
    if input == nil {
        input = map[string]any{}
    }
    return e.DB.ReplApp.WithRuntime(ctx, e.DB.EvalSessionID, func(rt *engine.Runtime) error {
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
```

### Decision 6: add exact structured last-value support to go-go-goja if needed

**Decision:** Do not rely on `ExecutionReport.Result` for tool output.

**Rationale:** `ExecutionReport.Result` is a display preview. We already observed that it can truncate with Unicode ellipses, which broke JSON decoding in earlier work. Tool results need exact JSON, not display strings.

Current workaround:

```js
globalThis.__chat_eval_last_json = JSON.stringify({ result: __chat_eval_result });
```

That workaround requires wrapping the user's code. REPL-cell mode should instead add a first-class exact-result path inside `go-go-goja`.

## Proposed go-go-goja API update

There are multiple acceptable ways to expose the exact last value. The recommended implementation is to add a JSON envelope to `replsession.ExecutionReport` or `RuntimeReport`.

### Proposed type addition

In `/home/manuel/code/wesen/corporate-headquarters/go-go-goja/pkg/replsession/types.go`:

```go
type ExecutionReport struct {
    Status      string         `json:"status"`
    Result      string         `json:"result"`
    ResultJSON  string         `json:"resultJson,omitempty"` // new: JSON envelope
    Error       string         `json:"error,omitempty"`
    DurationMS  int64          `json:"durationMs"`
    Awaited     bool           `json:"awaited"`
    Console     []ConsoleEvent `json:"console"`
    HadSideFX   bool           `json:"hadSideEffects"`
    HelperError bool           `json:"helperError"`
}
```

Use an envelope, not just raw JSON for the value:

```json
{
  "result": 42
}
```

Why an envelope? Because JavaScript `undefined` and functions are awkward for JSON. With an envelope we can later add metadata:

```json
{
  "result": null,
  "kind": "undefined"
}
```

or:

```json
{
  "error": "final expression is not JSON-serializable"
}
```

### Proposed internal return type

In `evaluate.go`, `executionOutcome` currently has:

```go
type executionOutcome struct {
    Awaited        bool
    LastValue      string
    PersistedNames []string
    HelperError    bool
}
```

Add:

```go
type executionOutcome struct {
    Awaited        bool
    LastValue      string
    LastValueJSON  string
    PersistedNames []string
    HelperError    bool
}
```

Then populate `ExecutionReport.ResultJSON` from `outcome.LastValueJSON`.

### Where to compute exact JSON

Best location: inside the `replsession` wrapper return path, because that is where the final expression helper value exists without needing another user-code wrapper.

`rewrite.go` currently returns an object like:

```js
return {
  "__ggg_repl_bindings_1__": {
    "x": x,
    "f": f
  },
  "__ggg_repl_last_1__": __ggg_repl_last_1__
};
```

Extend it to include a JSON envelope helper:

```js
return {
  "__ggg_repl_bindings_1__": {
    "x": x,
    "f": f
  },
  "__ggg_repl_last_1__": __ggg_repl_last_1__,
  "__ggg_repl_last_json_1__": JSON.stringify({ result: __ggg_repl_last_1__ })
};
```

But be careful: `JSON.stringify` can throw on cycles or BigInt. Use a small helper function or try/catch generated by the rewrite:

```js
let __ggg_repl_last_json_1__;
try {
  __ggg_repl_last_json_1__ = JSON.stringify({ result: __ggg_repl_last_1__ });
} catch (e) {
  __ggg_repl_last_json_1__ = JSON.stringify({
    error: "result is not JSON-serializable: " + String(e && e.message || e)
  });
}
```

Then include it in the returned helper object.

Pseudocode in `rewrite.go`:

```go
helperLastJSON := fmt.Sprintf("__ggg_repl_last_json_%d__", cellID)

builder.WriteString("  let ")
builder.WriteString(helperLast)
builder.WriteString(";\n")
builder.WriteString("  let ")
builder.WriteString(helperLastJSON)
builder.WriteString(";\n")

builder.WriteString(body)

builder.WriteString("  try {\n")
builder.WriteString("    ")
builder.WriteString(helperLastJSON)
builder.WriteString(" = JSON.stringify({ result: ")
builder.WriteString(helperLast)
builder.WriteString(" });\n")
builder.WriteString("  } catch (e) {\n")
builder.WriteString("    ")
builder.WriteString(helperLastJSON)
builder.WriteString(" = JSON.stringify({ error: 'result is not JSON-serializable: ' + String(e && e.message || e) });\n")
builder.WriteString("  }\n")
```

Then `persistWrappedReturn` reads the extra field:

```go
lastJSONValue := obj.Get(lastJSONKey)
lastJSON := ""
if lastJSONValue != nil && !goja.IsUndefined(lastJSONValue) && !goja.IsNull(lastJSONValue) {
    lastJSON = lastJSONValue.String()
}
```

### Alternative exact JSON implementation

Instead of generating `JSON.stringify` in `rewrite.go`, `persistWrappedReturn` could call JavaScript's `JSON.stringify` through goja after receiving `lastValue`:

```go
jsonObject := vm.Get("JSON").ToObject(vm)
stringify, _ := goja.AssertFunction(jsonObject.Get("stringify"))
envelope := vm.NewObject()
envelope.Set("result", lastValue)
jsonValue, err := stringify(jsonObject, envelope)
```

This keeps the generated source smaller. It also centralizes serialization in Go. The caveat is that it still executes JS serialization logic and can throw on cycles/BigInt, so it needs error handling.

Either implementation is acceptable. The important point is that `go-go-agent` should not have to wrap user code just to obtain JSON.

## Proposed go-go-agent implementation

### Step 1: replace `buildEvalCellSource`

Current function-body implementation:

```go
func buildEvalCellSource(in scopedjs.EvalInput) (string, error) {
    ...
    return fmt.Sprintf(`
const __chat_eval_input = %s;
const __chat_eval_result = await (async function(input) {
%s
})(__chat_eval_input);
globalThis.__chat_eval_last_json = JSON.stringify({ result: __chat_eval_result });
globalThis.__chat_eval_last_json;
`, inputJSON, in.Code), nil
}
```

New REPL-cell implementation:

```go
func buildEvalCellSource(in scopedjs.EvalInput) (string, error) {
    code := strings.TrimSpace(in.Code)
    if code == "" {
        return "undefined", nil
    }
    return code, nil
}
```

You may not even need this helper anymore.

### Step 2: set per-call `input` before evaluation

Add a helper in `internal/logdb/eval_tool.go`:

```go
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
```

Then call it before `Evaluate`:

```go
if err := e.prepareEvalGlobals(ctx, in); err != nil {
    return scopedjs.EvalOutput{Error: err.Error()}, nil
}

resp, evalErr := e.DB.ReplApp.Evaluate(ctx, e.DB.EvalSessionID, in.Code)
```

### Step 3: replace `readLastResultJSON`

Current code reads a global set by our wrapper:

```go
v := rt.VM.Get("__chat_eval_last_json")
```

New code should prefer `resp.Cell.Execution.ResultJSON` once added to `go-go-goja`:

```go
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
```

If `ResultJSON` is an envelope with an error field, decode and turn it into a tool error:

```go
type resultEnvelope struct {
    Result any    `json:"result"`
    Error  string `json:"error,omitempty"`
}

var env resultEnvelope
if err := json.Unmarshal([]byte(resultJSON), &env); err != nil {
    out.Error = "eval_js result was not valid JSON: " + err.Error()
    return out
}
if env.Error != "" {
    out.Error = env.Error
    return out
}
out.Result = env.Result
```

### Step 4: update tool docs and snippets

In `internal/evaljs/runtime.go`, update notes:

Current notes include:

```go
"Return a JSON-serializable value from the script.",
```

Replace with:

```go
"Write code as a persistent REPL cell; top-level declarations persist across calls.",
"The result is the final expression value; do not use top-level return.",
"Use globalThis for explicit global state; window and global are aliases of globalThis.",
"The final expression must be JSON-serializable for tool output.",
```

Current snippets:

```js
const rows = inputDB.query("SELECT slug, title, short FROM docs ORDER BY title LIMIT 10"); return rows;
```

Replace with:

```js
const rows = inputDB.query("SELECT slug, title, short FROM docs ORDER BY title LIMIT 10");
rows
```

A snippet showing persistent helper functions would be useful:

```js
function summarizeDoc(row) {
  return `${row.slug}: ${row.title}`;
}

const rows = inputDB.query("SELECT slug, title FROM docs ORDER BY title LIMIT 3");
rows.map(summarizeDoc)
```

### Step 5: tests in go-go-agent

Add tests in `internal/logdb/eval_tool_test.go`.

#### Test persistent function declaration

```go
func TestEvalToolPersistsFunctionDeclarationsAcrossCalls(t *testing.T) {
    db := newTestLogDB(t)
    tool := db.EvalTool()

    first, err := tool.Eval(ctx, scopedjs.EvalInput{
        Code: `
function plusOne(x) { return x + 1; }
plusOne(41)
`,
    })
    require.NoError(t, err)
    require.Empty(t, first.Error)
    require.Equal(t, float64(42), first.Result)

    second, err := tool.Eval(ctx, scopedjs.EvalInput{
        Code: `plusOne(9)`,
    })
    require.NoError(t, err)
    require.Empty(t, second.Error)
    require.Equal(t, float64(10), second.Result)
}
```

#### Test persistent const declaration

```go
func TestEvalToolPersistsConstDeclarationsAcrossCalls(t *testing.T) {
    _, _ = tool.Eval(ctx, scopedjs.EvalInput{Code: `const base = 100; base`})
    out, _ := tool.Eval(ctx, scopedjs.EvalInput{Code: `base + 23`})
    require.Equal(t, float64(123), out.Result)
}
```

#### Test top-level return gives a helpful error

```go
func TestEvalToolRejectsTopLevelReturn(t *testing.T) {
    out, _ := tool.Eval(ctx, scopedjs.EvalInput{Code: `return 42;`})
    require.Contains(t, out.Error, "return")
}
```

If the raw parser error is too cryptic, add a preflight check or improve the tool documentation before adding custom errors.

#### Test input is per-call

```go
func TestEvalToolSetsPerCallInput(t *testing.T) {
    out1, _ := tool.Eval(ctx, scopedjs.EvalInput{
        Code:  `input.value * 2`,
        Input: map[string]any{"value": 21},
    })
    require.Equal(t, float64(42), out1.Result)

    out2, _ := tool.Eval(ctx, scopedjs.EvalInput{
        Code:  `input.value * 2`,
        Input: map[string]any{"value": 5},
    })
    require.Equal(t, float64(10), out2.Result)
}
```

#### Test global aliases

```go
func TestEvalToolProvidesGlobalAliases(t *testing.T) {
    out, _ := tool.Eval(ctx, scopedjs.EvalInput{
        Code: `
window.answer = 42;
global.answer === globalThis.answer && globalThis.answer
`,
    })
    require.Equal(t, true, out.Result) // or 42 depending final expression
}
```

### Step 6: tests in go-go-goja

Add or update tests under:

```text
/home/manuel/code/wesen/corporate-headquarters/go-go-goja/pkg/replsession
```

Existing tests already cover some relevant behavior:

- `TestServiceInstrumentedAwaitChained`
- `TestServiceFunctionSourceMappingIsPopulated`

Add tests for exact JSON:

```go
func TestServiceInstrumentedResultJSONForFinalExpression(t *testing.T) {
    service := NewService(..., WithDefaultSessionOptions(PersistentSessionOptions()))
    session, _ := service.CreateSession(ctx)

    resp, err := service.Evaluate(ctx, session.ID, `
const x = 40;
x + 2
`)

    require.NoError(t, err)
    require.Equal(t, "ok", resp.Cell.Execution.Status)
    require.JSONEq(t, `{"result":42}`, resp.Cell.Execution.ResultJSON)
}
```

Test object result:

```go
resp, _ := service.Evaluate(ctx, session.ID, `({ slug: "a", count: 2 })`)
require.JSONEq(t, `{"result":{"slug":"a","count":2}}`, resp.Cell.Execution.ResultJSON)
```

Test non-serializable result:

```go
resp, _ := service.Evaluate(ctx, session.ID, `const x = {}; x.self = x; x`)
require.Contains(t, resp.Cell.Execution.ResultJSON, "not JSON-serializable")
```

## API references

### Geppetto scopedjs tool schema

File:

```text
/home/manuel/code/wesen/corporate-headquarters/geppetto/pkg/inference/tools/scopedjs/schema.go
```

Important types:

```go
type EvalInput struct {
    Code  string         `json:"code"`
    Input map[string]any `json:"input,omitempty"`
}

type EvalOutput struct {
    Result     any           `json:"result,omitempty"`
    Console    []ConsoleLine `json:"console,omitempty"`
    Error      string        `json:"error,omitempty"`
    DurationMs int64         `json:"durationMs,omitempty"`
}
```

We should preserve this external shape. The model still sends `{ code, input }` and receives `{ result, console, error, durationMs }`.

### go-go-agent EvalTool

File:

```text
internal/logdb/eval_tool.go
```

Current responsibilities:

- Validate replapi backend is configured.
- Build a source string.
- Evaluate it in the persistent repl session.
- Extract exact JSON result.
- Convert console/error/duration fields.
- Insert `eval_tool_calls` correlation rows into private SQLite.

After this ticket, the responsibilities remain the same except source building and exact-result extraction change.

### replapi App

File:

```text
/home/manuel/code/wesen/corporate-headquarters/go-go-goja/pkg/replapi/app.go
```

Important methods:

```go
func (a *App) Evaluate(ctx context.Context, sessionID string, source string) (*replsession.EvaluateResponse, error)
func (a *App) WithRuntime(ctx context.Context, sessionID string, fn func(*engine.Runtime) error) error
func (a *App) Bindings(ctx context.Context, sessionID string) ([]replsession.BindingView, error)
func (a *App) History(ctx context.Context, sessionID string) ([]repldb.EvaluationRecord, error)
```

Use `WithRuntime` for host setup. Use `Evaluate` for the user's real cell.

### replsession report types

File:

```text
/home/manuel/code/wesen/corporate-headquarters/go-go-goja/pkg/replsession/types.go
```

Current relevant fields:

```go
type CellReport struct {
    Source    string
    Rewrite   RewriteReport
    Execution ExecutionReport
    Runtime   RuntimeReport
}

type ExecutionReport struct {
    Status string
    Result string // display preview only
    Error  string
}

type RuntimeReport struct {
    PersistedByWrap  []string
    CurrentCellValue string // also preview string
}
```

Proposed addition:

```go
ResultJSON string `json:"resultJson,omitempty"`
```

## Implementation plan

### Phase 1: go-go-goja exact result support

1. Add `LastValueJSON` to the internal `executionOutcome` struct in `evaluate.go`.
2. Add `ResultJSON` to `ExecutionReport` in `types.go`.
3. Extend instrumented execution to compute a JSON envelope for the captured last expression.
4. Populate `ExecutionReport.ResultJSON` in `evaluateInstrumented`.
5. Decide whether raw mode should also populate `ResultJSON`.
   - Nice to have, but not required for this chat agent because the persistent profile uses instrumented mode.
6. Add replsession tests for scalar, object, array, undefined, and non-serializable final expressions.
7. Run go-go-goja tests.

### Phase 2: go-go-agent adapter change

1. Replace wrapper source construction with direct cell source.
2. Add `prepareEvalGlobals` to set `input`, `window`, and `global` through `WithRuntime`.
3. Replace `readLastResultJSON` with response-based extraction.
4. Update `convertReplResponseToEvalOutput` to decode the new envelope.
5. Update tests for persistent functions/constants, per-call input, aliases, and top-level return behavior.
6. Run `go test ./... -count=1` in go-go-agent.

### Phase 3: tool instruction change

1. Update `internal/evaljs/runtime.go` notes.
2. Replace all starter snippets that use `return`.
3. Add at least one snippet demonstrating persistent helper functions.
4. If any design docs still say "return a value", update them.
5. Run a live LLM smoke test where the model defines a helper in one `eval_js` call and uses it in another.

### Phase 4: private DB validation

Check that the private DB still records:

- `repldb` evaluations for each cell.
- `eval_tool_calls` correlation rows.
- final turns only by default unless snapshots are enabled.

Useful SQL:

```sql
SELECT COUNT(*) FROM evaluations WHERE session_id = ?;
SELECT repl_cell_id, code, error_text FROM eval_tool_calls ORDER BY created_at_ms;
SELECT phase, COUNT(*) FROM turn_block_membership GROUP BY phase ORDER BY phase;
```

### Phase 5: live acceptance test

Manual REPL script:

```text
Use eval_js to define a JavaScript function named titleOf that takes a row and returns row.title.
Then use eval_js again in a separate call to query the first help doc and call titleOf on the row.
```

Expected stream shape:

```text
[tool eval_js call ...]
code:
function titleOf(row) {
  return row.title;
}
titleOf
[tool eval_js done ...]

[tool eval_js call ...]
code:
const rows = inputDB.query("SELECT title FROM docs ORDER BY slug LIMIT 1");
titleOf(rows[0])
[tool eval_js done ...]
result:
{
  "result": "chat REPL User Guide"
}
```

## Edge cases and risks

### Top-level `return`

Top-level `return` is invalid JavaScript in a script/REPL cell. The model may keep generating it because the old prompt examples taught it to.

Mitigation:

- Update tool description strongly.
- Update starter snippets.
- Consider preflight detection for a clearer error:

```go
if looksLikeTopLevelReturn(code) {
    return EvalOutput{Error: "eval_js now uses REPL-cell semantics: use a final expression instead of top-level return"}, nil
}
```

Do not over-engineer top-level return detection unless raw parse errors are too confusing.

### Re-declaring `const` names

JavaScript normally rejects redeclaring a lexical `const` in the same global scope. `replsession` currently mirrors captured values onto the runtime global object, so its behavior may not match browser lexical environments exactly.

Test the current behavior before documenting it. The safe guidance is:

- Use unique helper names.
- Use assignment for mutable state:

```js
globalThis.cache = globalThis.cache ?? new Map();
globalThis.cache.set("x", 1);
globalThis.cache.size
```

### `input` collisions

Because `input` is a host-provided global, user code should not declare persistent top-level `const input = ...`.

Mitigation:

- Document `input` as reserved.
- If needed, set it under a less collision-prone name like `__toolInput` and provide `input` as an alias.

### Non-JSON-serializable results

Tool output must be JSON-compatible. JavaScript can produce values that JSON cannot represent:

- cyclic objects,
- BigInt,
- functions,
- symbols,
- `undefined`.

Mitigation:

- Require final expression to be JSON-serializable.
- Return a clear tool error if exact JSON serialization fails.
- Keep display previews in `ExecutionReport.Result` for humans.

### Security and privacy

The private logging DB must remain host-only. Do not expose it as a JS global.

Allowed globals:

- `inputDB`
- `outputDB`
- `input`
- `globalThis`
- optional aliases `window`, `global`

Forbidden globals:

- private log DB handle,
- repl store DB handle,
- chat turn store DB handle,
- raw `*sql.DB` objects.

## Acceptance criteria

The ticket is complete when all of these are true:

1. A function declared in one `eval_js` call is callable in a later `eval_js` call.
2. A constant declared in one `eval_js` call is readable in a later `eval_js` call.
3. The `eval_js` result is the final expression value.
4. Top-level `return` is no longer used in tool examples.
5. `input` still works as a per-call object.
6. `globalThis` works as the documented global object.
7. `window` and `global` aliases work if we choose to provide them.
8. Tool output uses exact JSON, not truncated display previews.
9. Private log DB internals are not exposed to JavaScript.
10. Tests cover persistent declarations, exact results, input, aliases, and invalid top-level return.
11. Live smoke evidence proves two separate tool calls can share a helper function.
12. `docmgr doctor --ticket CHAT-EVALJS-REPL-CELL --stale-after 30` passes.

## Suggested commit sequence

1. `go-go-goja`: add exact replsession result JSON.
2. `go-go-goja`: add tests for final expression JSON envelopes.
3. `go-go-agent`: switch eval_js adapter to direct REPL-cell evaluation.
4. `go-go-agent`: update eval_js tool docs and tests.
5. `go-go-agent`: add live smoke evidence and diary/changelog updates.

Keep commits small because this change crosses repository boundaries.

## Quick reference for interns

### If you only remember five things

- `replsession` already knows how to persist top-level declarations.
- Our current `eval_js` wrapper hides declarations inside a function.
- The fix is to evaluate the model's code directly as a cell.
- The result should be the final expression, not `return`.
- Exact JSON result support belongs in `go-go-goja`, not in a new wrapper around user code.

### Correct new JavaScript style

```js
function score(row) {
  return row.title.length;
}

const rows = inputDB.query("SELECT title FROM docs LIMIT 5");
rows.map(score)
```

### Correct later call

```js
const rows = inputDB.query("SELECT title FROM docs LIMIT 2");
rows.map(score)
```

### Correct explicit global state

```js
globalThis.seen = globalThis.seen ?? [];
globalThis.seen.push("chat-repl-user-guide");
globalThis.seen
```

### Compatibility aliases if implemented

```js
window.seen === globalThis.seen
global.seen === globalThis.seen
```

## Open questions

1. Should `window` and `global` aliases be installed once in the runtime initializer or before each tool call?
   - Recommendation: runtime initializer for aliases; per-call `input` before each call.
2. Should non-serializable final values be a tool error or should they fall back to a preview string?
   - Recommendation: tool error, because the schema promises JSON-like output.
3. Should `undefined` final expression produce omitted `result`, `null`, or an explicit metadata envelope?
   - Recommendation: envelope internally; external `EvalOutput` may omit `result` unless we need to distinguish it.
4. Should raw mode also support `ResultJSON`?
   - Recommendation: nice-to-have; persistent instrumented mode is required for this ticket.
5. Should we support legacy function-body mode behind a compatibility flag?
   - Recommendation: no by default. If needed, add an explicit separate tool or mode later, but avoid ambiguous semantics for the model.

## References

- `internal/logdb/eval_tool.go` — current adapter and wrapper that must change.
- `internal/evaljs/runtime.go` — tool docs, globals, and starter snippets.
- `/home/manuel/code/wesen/corporate-headquarters/go-go-goja/pkg/replsession/evaluate.go` — evaluation pipeline and result reporting.
- `/home/manuel/code/wesen/corporate-headquarters/go-go-goja/pkg/replsession/rewrite.go` — binding and final-expression capture rewrite.
- `/home/manuel/code/wesen/corporate-headquarters/go-go-goja/pkg/replsession/types.go` — report structures for adding exact result JSON.
- `/home/manuel/code/wesen/corporate-headquarters/go-go-goja/pkg/replapi/app.go` — public app facade used by go-go-agent.
- `/home/manuel/code/wesen/corporate-headquarters/go-go-goja/pkg/replapi/config.go` — persistent profile defaults.
- `/home/manuel/code/wesen/corporate-headquarters/geppetto/pkg/inference/tools/scopedjs/schema.go` — external eval input/output shape.
