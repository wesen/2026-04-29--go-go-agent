---
Title: Geppetto eval_js chatbot design and implementation guide
Ticket: LLM-EVAL-JS-CHATBOT
Status: active
Topics:
    - geppetto
    - goja
    - glazed
    - pinocchio
    - sqlite
    - llm-tools
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: ../../../../../../../corporate-headquarters/geppetto/pkg/inference/runner/types.go
      Note: Defines runner.Runtime and StartRequest used for the chatbot runner
    - Path: ../../../../../../../corporate-headquarters/geppetto/pkg/inference/tools/scopedjs/runtime.go
      Note: Defines BuildRuntime lifecycle for the prepared go-go-goja sandbox
    - Path: ../../../../../../../corporate-headquarters/geppetto/pkg/inference/tools/scopedjs/schema.go
      Note: Defines EvalInput/EvalOutput and EnvironmentSpec contracts used by eval_js design
    - Path: ../../../../../../../corporate-headquarters/geppetto/pkg/inference/tools/scopedjs/tool.go
      Note: Defines RegisterPrebuilt/NewLazyRegistrar tool registration choices
    - Path: ../../../../../../../corporate-headquarters/glazed/pkg/help/store/store.go
      Note: Defines the current Glazed SQLite help schema and sections table
    - Path: ../../../../../../../corporate-headquarters/go-go-goja/modules/database/database.go
      Note: Defines preconfigured SQLite module behavior and JS query/exec API
    - Path: ../../../../../../../corporate-headquarters/pinocchio/pkg/cmds/profilebootstrap/profile_selection.go
      Note: Defines Pinocchio bootstrap wrappers for profile resolution
ExternalSources: []
Summary: Intern-facing design for a stdin/stdout Geppetto chatbot with a single eval_js tool backed by a go-go-goja runtime and two Glazed-help SQLite databases.
LastUpdated: 2026-04-29T09:18:00-04:00
WhatFor: Use this when implementing the first simple REPL chatbot that can query Glazed documentation databases through JavaScript.
WhenToUse: Before writing the chatbot command, wiring Pinocchio profiles, or exposing SQLite-backed inputDB/outputDB globals to go-go-goja.
---


# Geppetto eval_js chatbot design and implementation guide

## Executive summary

We want a first, deliberately small LLM chatbot that runs in a terminal, reads user messages from stdin, sends them to Geppetto, and exposes exactly one model-callable tool: `eval_js`. The tool executes JavaScript in a prepared `go-go-goja` runtime. That runtime must contain two documented global database facades, `inputDB` and `outputDB`, connected to two SQLite databases that were populated up front from Glazed help export data.

The recommended implementation path is to **reuse Geppetto's existing `scopedjs` eval-tool package** instead of writing a bespoke tool loop. `scopedjs` already defines the public eval input/output contract, the runtime-building lifecycle, timeout handling, console capture, prebuilt/lazy tool registration, and tool-description synthesis. The application should own only the domain-specific parts: exporting/loading Glazed help databases, opening SQLite handles, binding `inputDB` and `outputDB` into the JavaScript runtime, resolving Pinocchio profiles, and running the stdin/stdout loop.

At a high level:

```text
terminal user
  -> simple REPL reads one message
  -> Pinocchio profilebootstrap resolves Geppetto inference settings
  -> Geppetto runner starts a tool-loop inference
  -> model may call eval_js({ code, input })
  -> scopedjs executes code in go-go-goja
  -> JS queries inputDB/outputDB
  -> tool result is appended to the Turn
  -> model writes final answer
  -> REPL prints answer and waits for next message
```

The only source mismatch discovered during the cursory investigation is terminology: the user prompt says the Glazed export data lands in a `docs` table, while the current Glazed store code creates a table named `sections`. The design below treats `sections` as the current evidence-backed schema and recommends adding a compatibility view named `docs` if the JavaScript-facing contract should use `docs`.

## Problem statement and scope

### Problem

A model can answer questions from its training data, but it cannot inspect the current Glazed documentation exports unless the host gives it a safe retrieval surface. We need a small chatbot that lets the model compose ad hoc documentation queries without exposing arbitrary host capabilities.

The desired tool surface is intentionally compact:

```json
{
  "name": "eval_js",
  "arguments": {
    "code": "const rows = inputDB.query('SELECT slug, title FROM sections LIMIT 5'); return rows;",
    "input": {}
  }
}
```

The JavaScript runtime should provide:

- `inputDB`: read-oriented facade over the primary Glazed help export database.
- `outputDB`: write-capable or scratch facade over a second SQLite database, depending on product decision.
- no broad file-system or process-execution access in the first version.
- bounded execution time and bounded output.
- a tool description clear enough that the LLM knows the schema, starter snippets, and safety rules.

### In scope for version 1

- A CLI command with a simple stdin/stdout REPL.
- Pinocchio/Geppetto profile resolution using the existing Pinocchio bootstrap path.
- Up-front preparation of two SQLite DBs from Glazed help exports or copied/exported database files.
- A single Geppetto tool named `eval_js`.
- A `go-go-goja` runtime built through `geppetto/pkg/inference/tools/scopedjs`.
- `inputDB.query(...)` and `outputDB.query(...)` at minimum.
- optional `outputDB.exec(...)` if `outputDB` is meant to be writable scratch space.
- clear tests for:
  - DB export/load,
  - JS global availability,
  - query behavior,
  - tool-loop registration,
  - a smoke REPL path.

### Out of scope for version 1

- A web UI.
- A Bubble Tea TUI.
- Multi-user session storage.
- Long-term conversation persistence.
- Arbitrary native module exposure (`fs`, `exec`, network, etc.).
- Full SQL sandboxing beyond SQLite handle permissions and wrapper-level restrictions.
- Automatic schema migration for arbitrary external help exports.

## Evidence-backed current-state analysis

### `go-go-goja`: runtime ownership and native modules

`go-go-goja` is the JavaScript runtime layer. Its README states that the project is for wiring Go-implemented native modules into a `goja` JavaScript runtime through Node-style `require()` and for composing runtime behavior explicitly through `engine.NewBuilder() -> Build() -> Factory.NewRuntime(...)` (`go-go-goja/README.md:3-11`). It also documents the canonical lifecycle:

1. create a builder,
2. add module/runtime options,
3. build an immutable factory,
4. create an owned runtime,
5. close the runtime explicitly (`go-go-goja/README.md:34-40`).

That explicit lifecycle matters for the chatbot because the command will own DB handles and a JavaScript runtime. It needs a deterministic cleanup path when the REPL exits.

The database module is directly relevant. `go-go-goja/modules/database/database.go` defines:

- `WithName(name string)`, which changes the module name (`database.go:22-28`).
- `WithPreconfiguredDB(db QueryExecer)`, which injects a Go-owned `Query`/`Exec` handle and disables runtime `configure(...)` (`database.go:31-38`).
- `WithCloseFn(closeFn func() error)`, which lets the module close a host-owned resource if desired (`database.go:41-47`).

The same file documents the JavaScript API: `query(sql, ...args)`, `exec(sql, ...args)`, `close()`, and optionally `configure(...)` (`database.go:127-148`). The loader exposes those functions to JavaScript exports (`database.go:153-160`).

### `geppetto`: tool loops, runner, and scoped JavaScript tools

Geppetto's tool documentation describes the LLM tool flow:

```text
model emits tool_call
host executes tool
host appends tool_use/tool result
model continues with fresh information
```

The docs state that provider engines emit `tool_call` blocks, the tool loop runner executes tools and appends `tool_use` blocks, and engines re-run with the updated Turn so the model can continue (`geppetto/pkg/doc/topics/07-tools.md:36-43`). They also describe a `tools.ToolRegistry` as the holder of callable tools (`07-tools.md:65-71`).

The `runner` package is the simplest command-facing API. `runner.Runtime` accepts fully resolved application-owned runtime input, including `InferenceSettings`, `SystemPrompt`, middleware uses, tool names, and tool registrars (`geppetto/pkg/inference/runner/types.go:18-36`). `runner.StartRequest` carries the prompt/seed turn, runtime, event sinks, and persistence hooks (`types.go:38-49`). `runner.New(...)` constructs a runner with sensible defaults, including the default engine factory and default tool-loop/tool configs (`runner/options.go:16-26`). `runner.Run(...)` prepares the run, starts inference, and waits for completion (`runner/run.go:25-35`).

For this project, the most relevant package is `geppetto/pkg/inference/tools/scopedjs`:

- `schema.go` defines `ToolDefinitionSpec`, `EvalOptions`, `EnvironmentSpec`, `EvalInput`, and `EvalOutput` (`schema.go:12-78`).
- `runtime.go` defines `BuildRuntime(...)`, which calls the application's `Configure` callback, builds the `go-go-goja` runtime with modules/globals/initializers, loads bootstrap JavaScript, and returns a `BuildResult` with a cleanup function (`runtime.go:50-82`).
- `eval.go` defines `RunEval(...)`, applies default eval options, adds a timeout, captures console output, runs the wrapped JavaScript body, and returns structured output (`eval.go:27-55`).
- `tool.go` defines `RegisterPrebuilt(...)`, which creates a Geppetto tool from a prepared runtime and registers it under `spec.Tool.Name` (`tool.go:11-39`). It also provides `NewLazyRegistrar(...)` for building a fresh runtime per call (`tool.go:41ff`).

There is already an intern-facing tutorial, `geppetto/pkg/doc/tutorials/07-build-scopedjs-eval-tools.md`, whose architecture matches this task: application scope -> `EnvironmentSpec` -> `Builder` -> `BuildRuntime(...)` -> `RegisterPrebuilt(...)` or `NewLazyRegistrar(...)` -> LLM-facing `eval_xxx` tool -> `RunEval(...)`.

### `pinocchio`: standard profile loading

Pinocchio already wraps the Geppetto bootstrap/profile machinery. `pinocchio/pkg/cmds/profilebootstrap/profile_selection.go` builds an app bootstrap config with:

- `AppName: "pinocchio"`,
- `EnvPrefix: "PINOCCHIO"`,
- the Pinocchio config file mapper,
- Geppetto profile settings sections,
- Geppetto base sections (`profile_selection.go:41-53`).

It exposes convenience wrappers such as `NewProfileSettingsSection(...)`, `NewCLISelectionValues(...)`, and `ResolveCLIConfigFilesResolved(...)` (`profile_selection.go:56-69`).

The `pinocchio/cmd/examples/simple-chat/main.go` example shows how a chat-like command uses this path. It creates a profile settings section via `profilebootstrap.NewProfileSettingsSection()` and includes it in the command description (`simple-chat/main.go:80-100`). At runtime it calls `profilebootstrap.ResolveCLIEngineSettings(ctx, parsedLayers)`, defers the returned close function, and uses `FinalInferenceSettings` as the step settings for inference (`simple-chat/main.go:132-142`).

This is the correct starting point for loading the standard Pinocchio profiles instead of inventing a parallel profile loader.

### `glazed`: help export and SQLite schema

Glazed's help docs describe a structured help system with typed sections, metadata, and query support. The help system stores sections in SQLite and can export the help tree to JSON, CSV, YAML, Markdown files, or a standalone SQLite database (`glazed/pkg/doc/topics/01-help-system.md:31-37`). A `Section` contains fields such as `Slug`, `Title`, `Short`, `Content`, `SectionType`, `Topics`, `Commands`, `Flags`, `IsTopLevel`, `ShowPerDefault`, and `Order` (`01-help-system.md:50-74`).

The export docs say `glaze help export --format sqlite --output-path ./help.db` produces a standalone SQLite database (`glazed/pkg/doc/topics/28-export-help-entries.md:58-67`). The same docs list the tabular columns: `slug`, `title`, `short`, `content`, `section_type`, `topics`, `commands`, `flags`, `is_top_level`, `show_per_default`, and `order` (`28-export-help-entries.md:83-97`).

The implementation confirms that SQLite export creates a normal help store and upserts each section into it (`glazed/pkg/help/cmd/export.go:269-295`). The store code creates a table named `sections`, not `docs`, with columns including `slug`, `section_type`, `title`, `sub_title`, `short`, `content`, `topics`, `flags`, `commands`, `is_top_level`, `is_template`, `show_per_default`, and `order_num` (`glazed/pkg/help/store/store.go:42-66`).

## Proposed architecture

### Component diagram

```text
┌────────────────────────────────────────────────────────────┐
│ CLI command: docs-chat                                     │
│ - flags: profile, profile-registries, input-db, output-db  │
│ - stdin/stdout REPL                                        │
└───────────────────────────┬────────────────────────────────┘
                            │
                            ▼
┌────────────────────────────────────────────────────────────┐
│ Pinocchio profile bootstrap                                │
│ profilebootstrap.ResolveCLIEngineSettings(...)             │
│ -> FinalInferenceSettings                                  │
└───────────────────────────┬────────────────────────────────┘
                            │
                            ▼
┌────────────────────────────────────────────────────────────┐
│ Geppetto runner                                            │
│ runner.New(...).Run(StartRequest{Runtime, Prompt/SeedTurn})│
│ - engine factory                                           │
│ - tool loop                                                │
│ - tool registry                                            │
└───────────────────────────┬────────────────────────────────┘
                            │ registers
                            ▼
┌────────────────────────────────────────────────────────────┐
│ eval_js tool                                               │
│ scopedjs.RegisterPrebuilt(...)                             │
│ input:  { code: string, input?: object }                   │
│ output: { result?, console?, error?, durationMs? }         │
└───────────────────────────┬────────────────────────────────┘
                            │ executes
                            ▼
┌────────────────────────────────────────────────────────────┐
│ go-go-goja Runtime                                         │
│ globals: inputDB, outputDB                                 │
│ no fs/exec/network in v1                                   │
│ timeout + output truncation via scopedjs EvalOptions        │
└───────────────┬──────────────────────────────┬─────────────┘
                │                              │
                ▼                              ▼
┌─────────────────────────────┐  ┌─────────────────────────────┐
│ input help SQLite DB         │  │ output/scratch SQLite DB     │
│ table: sections              │  │ table: sections and/or notes │
│ optional view: docs          │  │ optional view: docs          │
└─────────────────────────────┘  └─────────────────────────────┘
```

### Runtime data flow

```text
startup:
  parse flags
  ensure input/output DB paths exist, or export/copy them
  open inputDB sql.DB read-only
  open outputDB sql.DB according to policy
  build scopedjs EnvironmentSpec
  BuildRuntime(ctx, spec, scope)
  create runner with eval_js registrar

per user message:
  read line/block from stdin
  append user message to current conversation state
  call runner.Run(ctx, StartRequest{SeedTurn or Prompt, Runtime})
  if model calls eval_js:
      tool loop executes scopedjs RunEval
      JS queries inputDB/outputDB
      result returns to model
      model continues
  print final assistant text
```

### Why `scopedjs` should be the default path

`scopedjs` should be the implementation spine because it already has the contract we need:

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

It also handles details that are easy to get subtly wrong:

- creating the runtime from a declarative spec,
- documenting globals/modules/helpers in the model-facing description,
- wrapping user code in an async function,
- passing structured `input`,
- capturing console output,
- enforcing timeout defaults,
- truncating output,
- registering the resulting function as a Geppetto tool.

The application should not duplicate those mechanics.

## Detailed design

### CLI command shape

Name is open, but use something explicit during implementation, for example:

```text
docs-chat repl
```

Recommended first flags:

| Flag | Type | Required | Meaning |
|---|---:|---:|---|
| `--profile` | string | no | Pinocchio/Geppetto profile slug. |
| `--profile-registries` | string list | no | Registry source(s) for standard Pinocchio profiles. |
| `--input-db` | path | yes or defaulted | SQLite DB with Glazed help export. |
| `--output-db` | path | yes or temp default | SQLite DB for scratch/output data. |
| `--export-input-help` | bool | optional | If true, run Glazed help export before startup. |
| `--eval-timeout` | duration | optional | Default `5s`, maps to `scopedjs.EvalOptions.Timeout`. |
| `--max-output-chars` | int | optional | Default `16000`, maps to `EvalOptions.MaxOutputChars`. |
| `--debug-tool-results` | bool | optional | Print raw tool results for development. |

For the very first prototype, hardcode the DB paths and profile defaults if that is faster. The design should still keep the pieces separate so they can become flags later.

### Conversation state for the simple REPL

The simplest REPL is stateless across user turns: each line is one independent inference call. That is easy but less useful. A slightly better first version keeps an in-memory `turns.Turn` or session history while the process runs.

Recommended v1 behavior:

- Keep in-memory conversation during one process.
- Do not persist it yet.
- Support commands:
  - `:quit` / `:exit` to stop,
  - `:reset` to clear conversation,
  - `:help` to show REPL commands and an `eval_js` reminder.

Pseudocode:

```go
func repl(ctx context.Context, runtime runner.Runtime, r *runner.Runner) error {
    scanner := bufio.NewScanner(os.Stdin)
    seed := &turns.Turn{}
    turns.AppendBlock(seed, turns.NewSystemTextBlock(systemPromptForDocsChat()))

    for {
        fmt.Fprint(os.Stdout, "> ")
        if !scanner.Scan() { return scanner.Err() }
        line := strings.TrimSpace(scanner.Text())

        switch line {
        case "", ":help":
            printHelp()
            continue
        case ":quit", ":exit":
            return nil
        case ":reset":
            seed = &turns.Turn{}
            turns.AppendBlock(seed, turns.NewSystemTextBlock(systemPromptForDocsChat()))
            continue
        }

        _, out, err := r.Run(ctx, runner.StartRequest{
            SeedTurn: seed,
            Prompt:   line,
            Runtime:  runtime,
        })
        if err != nil { fmt.Fprintf(os.Stderr, "error: %v\n", err); continue }

        printAssistant(out)
        seed = out.Clone()
    }
}
```

Implementation note: check how `runner.Prepare(...)` clones a seed turn before appending a prompt. It clones `SeedTurn`, clears the turn ID, appends the user prompt, and appends it to the session (`geppetto/pkg/inference/runner/prepare.go:70-88`). That means the caller can keep the returned `out` as the next seed.

### Profile resolution

Use the Pinocchio path, not a custom parser.

Recommended command setup:

```go
profileSettingsSection, err := profilebootstrap.NewProfileSettingsSection()
if err != nil { return err }

cmds.NewCommandDescription(
    "docs-chat",
    cmds.WithShort("Chat with Glazed documentation through eval_js"),
    cmds.WithFlags(...),
    cmds.WithSections(profileSettingsSection),
)
```

Recommended runtime setup:

```go
resolved, err := profilebootstrap.ResolveCLIEngineSettings(ctx, parsedValues)
if err != nil { return fmt.Errorf("resolve inference settings: %w", err) }
if resolved.Close != nil { defer resolved.Close() }
if resolved.FinalInferenceSettings == nil { return errors.New("nil final inference settings") }

runtime := runner.Runtime{
    InferenceSettings: resolved.FinalInferenceSettings,
    ToolRegistrars: []runner.ToolRegistrar{
        evalJSRegistrar,
    },
    ToolNames: []string{"eval_js"}, // if profile gating requires explicit tool name
}
```

The exact field names on the resolved object should be confirmed during implementation, but `pinocchio/cmd/examples/simple-chat/main.go` demonstrates the pattern of `ResolveCLIEngineSettings(...)`, `Close`, and `FinalInferenceSettings`.

### Database preparation

There are two possible startup modes.

#### Mode A: accept prebuilt DB paths

This is the simplest and best first implementation:

```bash
glaze help export --format sqlite --output-path ./var/input-help.db
cp ./var/input-help.db ./var/output-help.db
```

Then run:

```bash
docs-chat repl \
  --input-db ./var/input-help.db \
  --output-db ./var/output-help.db \
  --profile openai-fast \
  --profile-registries ~/.config/pinocchio/profiles.yaml
```

The command only opens DBs; it does not run Glazed export itself.

#### Mode B: export DBs during startup

This is more convenient but introduces subprocess and binary-location questions. Use it only after Mode A works.

```go
func ensureHelpExport(ctx context.Context, glazeBinary, outPath string) error {
    cmd := exec.CommandContext(ctx, glazeBinary,
        "help", "export",
        "--format", "sqlite",
        "--output-path", outPath,
    )
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    return cmd.Run()
}
```

If the final command lives inside a repo that already has Glazed docs embedded, a better long-term approach is to call the Glazed help APIs directly. For the first intern task, use prebuilt DB paths to reduce moving parts.

### SQLite schema contract

Current Glazed export produces a `sections` table, not a `docs` table. Use this as the real schema:

```sql
CREATE TABLE sections (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    slug TEXT NOT NULL UNIQUE,
    section_type INTEGER NOT NULL,
    title TEXT NOT NULL,
    sub_title TEXT,
    short TEXT,
    content TEXT,
    topics TEXT,
    flags TEXT,
    commands TEXT,
    is_top_level BOOLEAN DEFAULT FALSE,
    is_template BOOLEAN DEFAULT FALSE,
    show_per_default BOOLEAN DEFAULT FALSE,
    order_num INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

If the JavaScript prompt/product contract must say `docs`, add a compatibility view:

```sql
CREATE VIEW IF NOT EXISTS docs AS
SELECT
    id,
    slug,
    section_type,
    title,
    sub_title,
    short,
    content,
    topics,
    flags,
    commands,
    is_top_level,
    is_template,
    show_per_default,
    order_num,
    created_at,
    updated_at
FROM sections;
```

Then both of these work:

```javascript
return inputDB.query("SELECT slug, title FROM sections LIMIT 5");
return inputDB.query("SELECT slug, title FROM docs LIMIT 5");
```

### Opening SQLite handles

For the input DB, prefer read-only mode:

```go
inputSQL, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?mode=ro", inputPath))
if err != nil { return err }
```

For the output DB, choose policy:

1. **Writable scratch DB**: open normal read/write/create.
2. **Read-only comparison DB**: open read-only like input.
3. **Copy-on-start output DB**: copy input DB to output path, then open writable.

Recommended v1: input DB is read-only; output DB is writable scratch. That gives the model somewhere to store derived notes without risking the source export.

### Binding `inputDB` and `outputDB` globals

There are two implementation choices.

#### Option 1: expose native DB modules and assign globals from `require(...)`

This uses `go-go-goja/modules/database.DBModule` directly:

```go
inputMod := databasemod.New(
    databasemod.WithName("inputDBModule"),
    databasemod.WithPreconfiguredDB(inputSQL),
)
outputMod := databasemod.New(
    databasemod.WithName("outputDBModule"),
    databasemod.WithPreconfiguredDB(outputSQL),
)

spec := scopedjs.EnvironmentSpec[Scope, Meta]{
    RuntimeLabel: "glazed-docs-chat",
    Tool: scopedjs.ToolDefinitionSpec{Name: "eval_js", ...},
    DefaultEval: scopedjs.DefaultEvalOptions(),
    Configure: func(ctx context.Context, b *scopedjs.Builder, scope Scope) (Meta, error) {
        if err := b.AddNativeModule(inputMod); err != nil { return Meta{}, err }
        if err := b.AddNativeModule(outputMod); err != nil { return Meta{}, err }
        b.AddBootstrapSource("db-globals.js", `
            globalThis.inputDB = require("inputDBModule");
            globalThis.outputDB = require("outputDBModule");
        `)
        return Meta{}, nil
    },
}
```

Pros:

- Minimal custom code.
- Reuses existing DB module docs and conversion behavior.
- Keeps `query`, `exec`, and `close` behavior consistent.

Cons:

- By default both globals expose `exec`. If `inputDB` must be strictly read-only at the JS API level, add a wrapper global instead.
- Internal module names are visible if documented poorly.

#### Option 2: bind custom global facades directly

This creates small Go objects and binds them with `Builder.AddGlobal(...)`:

```go
type JSDBFacade struct {
    db       *sql.DB
    readonly bool
}

func (f *JSDBFacade) Query(query string, args ...any) ([]map[string]any, error) { ... }
func (f *JSDBFacade) Exec(query string, args ...any) (map[string]any, error) {
    if f.readonly { return nil, errors.New("inputDB is read-only") }
    ...
}

b.AddGlobal("inputDB", func(rc *gojengine.RuntimeContext) error {
    return rc.VM.Set("inputDB", &JSDBFacade{db: inputSQL, readonly: true})
}, scopedjs.GlobalDoc{
    Type: "object",
    Description: "Read-only Glazed help DB facade with query(sql, ...args).",
})
```

Pros:

- Exact global shape requested by the product.
- Can enforce `inputDB` read-only before SQL reaches SQLite.
- Can hide `close()` from the model.
- Can normalize blobs/bytes and add guardrails.

Cons:

- More code to write and test.
- Duplicates some behavior already present in `modules/database`.

Recommended v1: **Option 2** if the requirement is truly global `inputDB`/`outputDB` with precise policy. Use the existing database module code as the implementation reference. If speed matters more than policy precision, start with Option 1 and immediately add a bootstrap wrapper that exposes only selected methods.

### JavaScript-facing API

Document this in the tool description so the model knows what to do:

```javascript
// Read rows from the exported Glazed help docs.
const rows = inputDB.query(
  "SELECT slug, title, short FROM sections WHERE title LIKE ? LIMIT 10",
  "%profile%"
);

// Return values become tool result.result.
return rows;
```

Recommended `inputDB` methods:

| Method | Arguments | Returns | Notes |
|---|---|---|---|
| `query(sql, ...args)` | SQL string + bind args | `Array<Object>` | Only `SELECT`/`WITH` should be allowed for input. |
| `schema()` | none | schema summary object | Optional but very helpful for the model. |

Recommended `outputDB` methods:

| Method | Arguments | Returns | Notes |
|---|---|---|---|
| `query(sql, ...args)` | SQL string + bind args | `Array<Object>` | Reads scratch/output data. |
| `exec(sql, ...args)` | SQL string + bind args | result summary | Only if output is writable. |
| `schema()` | none | schema summary object | Optional. |

Do not expose `close()` to the model in v1. The Go process should own handle cleanup.

### Tool description

The tool description is not an afterthought. It is part of the runtime contract. The model will perform better if the description includes:

- what `eval_js` does,
- the fact that code is wrapped in an async function,
- available globals,
- available tables/views,
- examples,
- safety rules,
- expected return style.

Draft:

```text
Execute JavaScript in a constrained documentation-analysis runtime.
Use this tool when you need to inspect Glazed help exports.
Available globals:
- inputDB: read-only SQLite facade for exported Glazed help sections.
- outputDB: writable scratch SQLite facade for derived notes/results.
The main table is sections; a compatibility view docs may also exist.
Return a JSON-serializable value from the script. Use console.log for diagnostics.
Do not attempt filesystem, network, or process access.
Prefer parameterized SQL: inputDB.query("SELECT ... WHERE slug = ?", slug).
```

Starter snippets:

```javascript
const rows = inputDB.query(`
  SELECT slug, title, short
  FROM sections
  WHERE title LIKE ? OR content LIKE ?
  LIMIT 10
`, "%profile%", "%profile%");
return rows;
```

```javascript
const count = inputDB.query("SELECT COUNT(*) AS n FROM sections");
return count[0];
```

```javascript
const docs = inputDB.query(`
  SELECT slug, title, section_type, topics
  FROM docs
  WHERE topics LIKE ?
  ORDER BY title
  LIMIT 20
`, "%sqlite%");
return docs;
```

### System prompt

The REPL should seed a system prompt that teaches the assistant when to use the tool:

```text
You are a documentation assistant for Glazed/Geppetto/Pinocchio/go-go-goja sources.
When a user asks about available docs, APIs, schemas, examples, or implementation details,
use eval_js to inspect the SQLite help databases before answering.
The eval_js runtime has inputDB and outputDB globals.
Prefer small, targeted SELECT queries.
Summarize results in prose and cite slugs/titles when possible.
If a query fails, explain the failure and try a simpler query.
```

This prompt should be combined with any profile-provided system prompt carefully. If Pinocchio profiles already define system prompts, decide whether to prepend or append the docs-chat instruction. Recommended v1: prepend a short non-negotiable runtime instruction, then allow profile system prompt after it.

## Pseudocode implementation guide

### Package layout suggestion

Assuming implementation happens in Pinocchio or a sibling command repo:

```text
cmd/docs-chat/
  main.go                 # Cobra/Glazed command and REPL loop
  settings.go             # CLI settings structs and sections
  profile.go              # Pinocchio profile resolution wrapper
  db.go                   # DB opening, schema/view setup, facade
  evaljs.go               # scopedjs EnvironmentSpec and registrar
  printing.go             # Turn/assistant output helpers
```

If implementation happens in a new repo, keep the same conceptual split.

### Settings structs

```go
type ChatSettings struct {
    InputDBPath    string        `glazed:"input-db"`
    OutputDBPath   string        `glazed:"output-db"`
    EvalTimeout    time.Duration `glazed:"eval-timeout"`
    MaxOutputChars int           `glazed:"max-output-chars"`
    DebugTools     bool          `glazed:"debug-tool-results"`
}
```

### Scope and metadata

```go
type EvalJSScope struct {
    InputDB  *sql.DB
    OutputDB *sql.DB
    Schema   DBSchemaSummary
}

type EvalJSMeta struct {
    InputPath  string
    OutputPath string
    Tables     []string
}
```

### Build the eval tool

```go
func NewEvalJSSpec() scopedjs.EnvironmentSpec[EvalJSScope, EvalJSMeta] {
    return scopedjs.EnvironmentSpec[EvalJSScope, EvalJSMeta]{
        RuntimeLabel: "glazed-help-eval-js",
        Tool: scopedjs.ToolDefinitionSpec{
            Name: "eval_js",
            Description: scopedjs.ToolDescription{
                Summary: "Execute JavaScript against Glazed help SQLite databases exposed as inputDB and outputDB.",
                Notes: []string{
                    "inputDB is read-only; use SELECT/WITH queries.",
                    "The Glazed export table is sections; docs may exist as a compatibility view.",
                    "Return a JSON-serializable value.",
                },
                StarterSnippets: []string{
                    `const rows = inputDB.query("SELECT slug, title, short FROM sections LIMIT 5"); return rows;`,
                },
            },
            Tags: []string{"javascript", "sqlite", "glazed", "docs"},
            Version: "0.1.0",
        },
        DefaultEval: scopedjs.EvalOptions{
            Timeout:        5 * time.Second,
            MaxOutputChars: 16_000,
            CaptureConsole: true,
        },
        Configure: configureEvalJSRuntime,
    }
}
```

### Configure globals

```go
func configureEvalJSRuntime(ctx context.Context, b *scopedjs.Builder, scope EvalJSScope) (EvalJSMeta, error) {
    inputFacade := NewDBFacade(scope.InputDB, DBFacadeOptions{Readonly: true, Name: "inputDB"})
    outputFacade := NewDBFacade(scope.OutputDB, DBFacadeOptions{Readonly: false, Name: "outputDB"})

    if err := b.AddGlobal("inputDB", func(rc *gojengine.RuntimeContext) error {
        return rc.VM.Set("inputDB", inputFacade)
    }, scopedjs.GlobalDoc{
        Type: "object",
        Description: "Read-only SQLite facade for Glazed help sections. Methods: query(sql, ...args), schema().",
    }); err != nil { return EvalJSMeta{}, err }

    if err := b.AddGlobal("outputDB", func(rc *gojengine.RuntimeContext) error {
        return rc.VM.Set("outputDB", outputFacade)
    }, scopedjs.GlobalDoc{
        Type: "object",
        Description: "Writable scratch SQLite facade. Methods: query(sql, ...args), exec(sql, ...args), schema().",
    }); err != nil { return EvalJSMeta{}, err }

    if err := b.AddHelper(scopedjs.HelperDoc{
        Name: "parameterized SQL",
        Signature: `inputDB.query("SELECT ... WHERE slug = ?", slug)`,
        Description: "Use ? placeholders instead of string concatenation.",
    }); err != nil { return EvalJSMeta{}, err }

    return EvalJSMeta{Tables: []string{"sections", "docs"}}, nil
}
```

The exact `AddGlobal`/`AddHelper` signatures should be verified against `scopedjs/builder.go` during coding, but this is the intended shape.

### Register as a Geppetto tool

```go
func NewEvalJSToolRegistrar(scope EvalJSScope, evalOpts scopedjs.EvalOptionOverrides) runner.ToolRegistrar {
    spec := NewEvalJSSpec()
    return func(ctx context.Context, reg tools.ToolRegistry) error {
        handle, err := scopedjs.BuildRuntime(ctx, spec, scope)
        if err != nil { return err }
        // Store handle cleanup somewhere command-owned, or bind it to process cleanup.
        return scopedjs.RegisterPrebuilt(reg, spec, handle, evalOpts)
    }
}
```

Important cleanup note: `RegisterPrebuilt` reuses one runtime instance. The command must call `handle.Cleanup()` when the REPL exits. If the registrar hides the handle, cleanup becomes awkward. Prefer an application struct:

```go
type EvalJSToolRuntime struct {
    Spec   scopedjs.EnvironmentSpec[EvalJSScope, EvalJSMeta]
    Handle *scopedjs.BuildResult[EvalJSMeta]
}

func (e *EvalJSToolRuntime) Registrar() runner.ToolRegistrar {
    return func(ctx context.Context, reg tools.ToolRegistry) error {
        return scopedjs.RegisterPrebuilt(reg, e.Spec, e.Handle, scopedjs.EvalOptionOverrides{})
    }
}

func (e *EvalJSToolRuntime) Close() error {
    if e.Handle != nil && e.Handle.Cleanup != nil { return e.Handle.Cleanup() }
    return nil
}
```

### Main command flow

```go
func run(ctx context.Context, parsed *values.Values, stdin io.Reader, stdout io.Writer) error {
    settings := decodeChatSettings(parsed)

    resolved, err := profilebootstrap.ResolveCLIEngineSettings(ctx, parsed)
    if err != nil { return err }
    defer closeIfPresent(resolved.Close)

    inputDB, err := openInputDB(settings.InputDBPath)
    if err != nil { return err }
    defer inputDB.Close()

    outputDB, err := openOutputDB(settings.OutputDBPath)
    if err != nil { return err }
    defer outputDB.Close()

    if err := ensureDocsView(ctx, inputDB); err != nil { return err }
    if err := ensureOutputSchema(ctx, outputDB); err != nil { return err }

    evalRuntime, err := BuildEvalJSToolRuntime(ctx, EvalJSScope{InputDB: inputDB, OutputDB: outputDB})
    if err != nil { return err }
    defer evalRuntime.Close()

    r := runner.New(
        runner.WithToolRegistrars(evalRuntime.Registrar()),
    )

    runtime := runner.Runtime{
        InferenceSettings: resolved.FinalInferenceSettings,
        ToolRegistrars: []runner.ToolRegistrar{evalRuntime.Registrar()},
        ToolNames: []string{"eval_js"},
    }

    return repl(ctx, r, runtime, stdin, stdout)
}
```

Avoid registering the same registrar twice. Choose either `runner.New(WithToolRegistrars(...))` or `StartRequest.Runtime.ToolRegistrars`, not both.

## Testing and validation plan

### Unit tests

1. **DB facade tests**
   - Create temp SQLite DB.
   - Create `sections` table and insert sample docs.
   - Assert `inputDB.query(...)` returns rows.
   - Assert `inputDB.exec(...)` fails if input is read-only.
   - Assert `outputDB.exec(...)` succeeds if output is writable.

2. **Schema compatibility test**
   - Create `sections` only.
   - Run `ensureDocsView(...)`.
   - Assert `SELECT COUNT(*) FROM docs` works.

3. **Scoped JS runtime test**
   - Build runtime with temp DBs.
   - Run `scopedjs.RunEval(...)` or executor `RunEval(...)` with:
     ```javascript
     const rows = inputDB.query("SELECT slug FROM docs ORDER BY slug");
     return rows.map(r => r.slug);
     ```
   - Assert expected slugs.

4. **Tool registration test**
   - Create `tools.NewInMemoryToolRegistry()`.
   - Register `eval_js`.
   - Assert registry has exactly `eval_js`.

5. **Runner smoke test**
   - Use a fake engine if available, or a dry-run test around `runner.Prepare(...)`.
   - Assert prepared registry contains `eval_js`.

### Integration tests

1. **Glazed export smoke**
   ```bash
   cd /home/manuel/code/wesen/corporate-headquarters/glazed
   go run ./cmd/glaze help export --format sqlite --output-path /tmp/glazed-help.db
   sqlite3 /tmp/glazed-help.db '.schema sections'
   sqlite3 /tmp/glazed-help.db 'SELECT slug, title FROM sections LIMIT 5;'
   ```

2. **Manual REPL smoke**
   ```bash
   docs-chat repl --input-db /tmp/glazed-help.db --output-db /tmp/docs-chat-output.db
   > Find docs about profile registries. Use eval_js if needed.
   ```

3. **Tool-forcing prompt**
   Ask a prompt that requires exact DB content:
   ```text
   Use eval_js to count help sections by section_type and summarize the counts.
   ```

Expected behavior:

- model calls `eval_js`,
- tool queries SQLite,
- final answer includes counts.

### Failure-mode tests

- Missing input DB path -> clear startup error.
- Invalid SQL -> tool returns structured `error`, model explains it.
- Long-running JS -> timeout error.
- Huge result -> output truncation.
- `inputDB.exec("DELETE ...")` -> rejected.
- `require("fs")` -> unavailable unless explicitly enabled.

## Risks and guardrails

### Risk: JavaScript tool is too powerful

Even without `fs` or `exec`, JavaScript can run loops and produce huge data. Mitigate with:

- `EvalOptions.Timeout`, default 5 seconds.
- `MaxOutputChars`, default 16k.
- no filesystem/process/network modules in v1.
- small model-facing examples that prefer targeted queries.

### Risk: SQL writes to input DB

Mitigate with:

- opening input with SQLite `mode=ro`,
- wrapper-level read-only enforcement,
- no `exec` method on `inputDB`, or `exec` always returns an error.

### Risk: schema mismatch (`docs` vs `sections`)

Mitigate with:

- evidence-backed implementation using `sections`,
- compatibility view `docs`,
- tool description explicitly mentions both.

### Risk: profile resolution confusion

Mitigate by using Pinocchio's `profilebootstrap` wrappers and copying the shape from `cmd/examples/simple-chat`. Do not parse profile registry files manually.

### Risk: runtime cleanup leaks DB handles

Mitigate with command-owned structs and `defer` cleanup:

```go
defer evalRuntime.Close()
defer inputDB.Close()
defer outputDB.Close()
defer resolved.Close()
```

### Risk: duplicate tool registration

If both the runner and runtime register `eval_js` twice, registry construction may fail. Pick one registration path and test it.

## Alternatives considered

### Alternative 1: Write a custom Geppetto tool function by hand

This would use `tools.NewToolFromFunc("eval_js", ..., func(ctx, input) ...)` directly and call goja manually.

Rejected for v1 because `scopedjs` already handles eval input/output, console capture, promise handling, timeout defaults, runtime specs, and descriptions.

### Alternative 2: Use `scopeddb` instead of JavaScript

Geppetto also has `pkg/inference/tools/scopeddb`, which exposes a constrained SQL query tool. This is safer and simpler if the model only needs SQL.

Not chosen because the requested interface is explicitly `eval_js`, and JavaScript gives the model a composition layer for multiple queries and post-processing. However, `scopeddb` docs are worth reading for SQL safety ideas.

### Alternative 3: Make each DB a separate LLM tool

For example, `query_input_db` and `write_output_db`.

Not chosen because the requested product shape is a single tool call. Multiple tools also force the model to coordinate state across calls instead of composing a short script.

### Alternative 4: Persistent runtime vs fresh runtime per call

`scopedjs.RegisterPrebuilt(...)` reuses one runtime, so JavaScript global state can persist across calls. `scopedjs.NewLazyRegistrar(...)` builds a fresh runtime per call.

Recommended v1: prebuilt runtime for simplicity and speed, with clear warning that global JS state persists. If state leakage becomes confusing, switch to lazy/fresh runtime.

## Phased implementation plan

### Phase 0: Confirm target repo and command name

Decide whether the command lives in:

- `pinocchio/cmd/...`,
- `geppetto/cmd/examples/...`,
- a new app repo,
- or this current working repo.

For fastest learning, implement first as an example command, then promote.

### Phase 1: DB export and schema smoke

- Export Glazed help DB with `glaze help export --format sqlite`.
- Inspect schema.
- Add `docs` compatibility view if desired.
- Write a tiny Go test that opens the DB and reads from `sections`.

Deliverable: a repeatable command or script that creates `input-help.db` and `output-help.db`.

### Phase 2: DB facade

- Implement `DBFacade` with `query`, `exec`, and `schema`.
- Enforce read-only policy for `inputDB`.
- Unit test all methods.

Deliverable: Go object that can be bound into goja.

### Phase 3: scopedjs runtime

- Implement `EvalJSScope`, `EvalJSMeta`, and `NewEvalJSSpec()`.
- Bind globals with `Builder.AddGlobal(...)`.
- Add tool description and starter snippets.
- Build runtime and run one direct eval in a test.

Deliverable: `eval_js` runtime can run `return inputDB.query(...)`.

### Phase 4: Geppetto runner integration

- Build a `runner.ToolRegistrar` for `eval_js`.
- Resolve Pinocchio profiles.
- Create `runner.Runtime` with `FinalInferenceSettings` and the registrar.
- Test `runner.Prepare(...)` sees the tool.

Deliverable: one inference call can advertise/use `eval_js`.

### Phase 5: stdin/stdout REPL

- Add REPL loop.
- Add `:quit`, `:reset`, `:help`.
- Print assistant output cleanly.
- Optionally print tool debug output under `--debug-tool-results`.

Deliverable: interactive command.

### Phase 6: docs and polish

- Add README/help entry for the new command.
- Add example prompts.
- Add troubleshooting section.
- Decide whether to expose Glazed export as a command flag.

## Relevant documentation starting points

This section is intentionally explicit so a future intern can begin from the right docs without rediscovering the repositories.

### Primary starting points

Read these first, in this order:

1. `geppetto/pkg/doc/tutorials/07-build-scopedjs-eval-tools.md`
   - Best conceptual match for `eval_js`.
   - Explains `EnvironmentSpec`, `Builder`, globals, bootstrap, and registration.
2. `geppetto/pkg/doc/topics/07-tools.md`
   - Explains Geppetto tool calls and the turn-based tool loop.
3. `geppetto/pkg/doc/topics/10-runner.md`
   - Explains the higher-level runner API used by small commands.
4. `pinocchio/pkg/doc/topics/pinocchio-profile-resolution-and-runtime-switching.md`
   - Explains Pinocchio profile resolution concepts.
5. `pinocchio/pkg/doc/tutorials/07-migrating-cli-verbs-to-glazed-profile-bootstrap.md`
   - Explains how Pinocchio commands should use the Geppetto bootstrap path.
6. `glazed/pkg/doc/topics/28-export-help-entries.md`
   - Explains `glaze help export --format sqlite`.
7. `glazed/pkg/doc/topics/01-help-system.md`
   - Explains the help section model.
8. `go-go-goja/README.md`
   - Explains runtime builder/factory lifecycle and module loading.
9. `go-go-goja/pkg/doc/02-creating-modules.md`
   - Explains native module patterns if the DB facade becomes a native module.

### Relevant `go-go-goja` docs

- `go-go-goja/README.md`
- `go-go-goja/AGENT.md`
- `go-go-goja/pkg/doc/01-introduction.md`
- `go-go-goja/pkg/doc/02-creating-modules.md`
- `go-go-goja/pkg/doc/03-async-patterns.md`
- `go-go-goja/pkg/doc/04-repl-usage.md`
- `go-go-goja/pkg/doc/05-jsparse-framework-reference.md`
- `go-go-goja/pkg/doc/12-plugin-user-guide.md`
- `go-go-goja/pkg/doc/13-plugin-developer-guide.md`
- `go-go-goja/pkg/doc/14-plugin-tutorial-build-install.md`
- `go-go-goja/pkg/doc/15-docs-module-guide.md`
- `go-go-goja/pkg/doc/16-nodejs-primitives.md`
- `go-go-goja/pkg/doc/16-yaml-module.md`
- `go-go-goja/pkg/doc/17-connected-eventemitters-developer-guide.md`
- `go-go-goja/pkg/doc/bun-goja-bundling-playbook.md`
- `go-go-goja/cmd/goja-jsdoc/doc/01-jsdoc-system.md`
- `go-go-goja/plugins/examples/README.md`
- `go-go-goja/perf/goja/README.md`

Complete generated inventory for this investigation:

- `sources/go-go-goja-docs.txt`

### Relevant `glazed` docs

- `glazed/README.md`
- `glazed/AGENT.md`
- `glazed/changelog.md`
- `glazed/pkg/doc/topics/00-documentation-guidelines.md`
- `glazed/pkg/doc/topics/01-help-system.md`
- `glazed/pkg/doc/topics/02-markdown-style.md`
- `glazed/pkg/doc/topics/14-writing-help-entries.md`
- `glazed/pkg/doc/topics/25-serving-help-over-http.md`
- `glazed/pkg/doc/topics/26-export-help-as-static-website.md`
- `glazed/pkg/doc/topics/28-export-help-entries.md`
- `glazed/pkg/doc/topics/simple-query-dsl.md`
- `glazed/pkg/doc/topics/user-query-dsl.md`
- `glazed/pkg/doc/topics/using-the-query-api.md`
- `glazed/pkg/doc/topics/commands-reference.md`
- `glazed/pkg/doc/topics/how-to-write-good-documentation-pages.md`
- `glazed/pkg/doc/topics/sections-guide.md`
- `glazed/pkg/help/store/README.md`
- `glazed/cmd/examples/help-system/README.md`
- `glazed/cmd/examples/help-system/docs/configuration-topic.md`
- `glazed/cmd/examples/help-system/docs/database-tutorial.md`
- `glazed/cmd/examples/help-system/docs/data-pipeline-application.md`
- `glazed/cmd/examples/help-system/docs/json-example.md`
- `glazed/pkg/doc/examples/help/help-example-1.md`
- `glazed/pkg/doc/examples/help/help-example-2.md`
- `glazed/pkg/doc/examples/output/sql-output.md`
- `glazed/pkg/doc/applications/01-exposing-a-simple-sql-table.md`

Complete generated inventory for this investigation:

- `sources/glazed-docs.txt`

### Relevant `geppetto` docs

- `geppetto/README.md`
- `geppetto/AGENT.md`
- `geppetto/changelog.md`
- `geppetto/pkg/doc/topics/00-docs-index.md`
- `geppetto/pkg/doc/topics/01-profiles.md`
- `geppetto/pkg/doc/topics/04-events.md`
- `geppetto/pkg/doc/topics/06-inference-engines.md`
- `geppetto/pkg/doc/topics/07-tools.md`
- `geppetto/pkg/doc/topics/08-turns.md`
- `geppetto/pkg/doc/topics/09-middlewares.md`
- `geppetto/pkg/doc/topics/10-runner.md`
- `geppetto/pkg/doc/topics/10-sessions.md`
- `geppetto/pkg/doc/topics/11-structured-sinks.md`
- `geppetto/pkg/doc/topics/13-js-api-reference.md`
- `geppetto/pkg/doc/topics/14-js-api-user-guide.md`
- `geppetto/pkg/doc/tutorials/01-streaming-inference-with-tools.md`
- `geppetto/pkg/doc/tutorials/06-using-scoped-tool-databases.md`
- `geppetto/pkg/doc/tutorials/07-build-scopedjs-eval-tools.md`
- `geppetto/pkg/doc/tutorials/08-build-streaming-tool-loop-agent-with-glazed-flags.md`
- `geppetto/pkg/doc/tutorials/09-migrating-cli-commands-to-glazed-bootstrap-profile-resolution.md`
- `geppetto/pkg/doc/playbooks/01-add-a-new-tool.md`
- `geppetto/pkg/doc/playbooks/04-migrate-to-session-api.md`
- `geppetto/pkg/doc/playbooks/05-migrate-legacy-profiles-yaml-to-registry.md`
- `geppetto/pkg/doc/playbooks/06-operate-sqlite-profile-registry.md`
- `geppetto/pkg/doc/playbooks/07-wire-provider-credentials-for-js-and-go-runner.md`
- `geppetto/pkg/doc/playbooks/08-bootstrap-binary-step-settings-from-defaults-config-registries-profile.md`
- `geppetto/examples/js/geppetto/README.md`
- `geppetto/cmd/examples/README.md`
- `geppetto/cmd/examples/streaming-inference/README.md`

Complete generated inventory for this investigation:

- `sources/geppetto-docs.txt`

### Relevant `pinocchio` docs

- `pinocchio/README.md`
- `pinocchio/AGENT.md`
- `pinocchio/AGENTS.md`
- `pinocchio/changelog.md`
- `pinocchio/pkg/doc/topics/01-chat-builder-guide.md`
- `pinocchio/pkg/doc/topics/13-js-api-reference.md`
- `pinocchio/pkg/doc/topics/14-js-api-user-guide.md`
- `pinocchio/pkg/doc/topics/pinocchio-profile-resolution-and-runtime-switching.md`
- `pinocchio/pkg/doc/topics/pinocchio-tui-integration-playbook.md`
- `pinocchio/pkg/doc/topics/runtime-symbol-migration-playbook.md`
- `pinocchio/pkg/doc/topics/webchat-overview.md`
- `pinocchio/pkg/doc/topics/webchat-profile-registry.md`
- `pinocchio/pkg/doc/topics/webchat-engine-profile-migration-playbook.md`
- `pinocchio/pkg/doc/topics/webchat-runner-migration-guide.md`
- `pinocchio/pkg/doc/tutorials/01-building-a-middleware-with-renderer.md`
- `pinocchio/pkg/doc/tutorials/06-tui-integration-guide.md`
- `pinocchio/pkg/doc/tutorials/07-migrating-cli-verbs-to-glazed-profile-bootstrap.md`
- `pinocchio/pkg/doc/tutorials/08-migrating-legacy-pinocchio-config-to-unified-profile-documents.md`
- `pinocchio/cmd/web-chat/README.md`
- `pinocchio/cmd/examples/scopeddb-tui-demo/README.md`
- `pinocchio/cmd/examples/scopedjs-tui-demo/README.md`
- `pinocchio/examples/js/README.md`
- `pinocchio/cmd/pinocchio/doc/general/05-js-runner-scripts.md`

Complete generated inventory for this investigation:

- `sources/pinocchio-docs.txt`

## Code references to inspect before implementing

### `go-go-goja`

- `go-go-goja/engine/factory.go`
- `go-go-goja/engine/runtime.go`
- `go-go-goja/engine/options.go`
- `go-go-goja/engine/module_specs.go`
- `go-go-goja/engine/runtime_modules.go`
- `go-go-goja/modules/database/database.go`
- `go-go-goja/modules/common.go`
- `go-go-goja/cmd/goja-repl/...`

### `geppetto`

- `geppetto/pkg/inference/tools/scopedjs/schema.go`
- `geppetto/pkg/inference/tools/scopedjs/builder.go`
- `geppetto/pkg/inference/tools/scopedjs/runtime.go`
- `geppetto/pkg/inference/tools/scopedjs/eval.go`
- `geppetto/pkg/inference/tools/scopedjs/tool.go`
- `geppetto/pkg/inference/tools/scopedjs/description.go`
- `geppetto/pkg/inference/tools/scopeddb/...` for SQL safety patterns
- `geppetto/pkg/inference/runner/types.go`
- `geppetto/pkg/inference/runner/options.go`
- `geppetto/pkg/inference/runner/prepare.go`
- `geppetto/pkg/inference/runner/run.go`
- `geppetto/cmd/examples/runner-glazed-registry-flags/main.go`
- `geppetto/cmd/examples/internal/runnerexample/inference_settings.go`

### `pinocchio`

- `pinocchio/pkg/cmds/profilebootstrap/profile_selection.go`
- `pinocchio/pkg/cmds/profilebootstrap/...`
- `pinocchio/cmd/examples/simple-chat/main.go`
- `pinocchio/cmd/examples/internal/tuidemo/cli.go`
- `pinocchio/cmd/examples/internal/tuidemo/profile.go`

### `glazed`

- `glazed/pkg/help/cmd/export.go`
- `glazed/pkg/help/store/store.go`
- `glazed/pkg/help/model/...`
- `glazed/pkg/help/loader/sources.go`
- `glazed/pkg/help/cmd/cobra.go`
- `glazed/cmd/glaze/main.go`

## Open questions

1. **Where should the command live?** Pinocchio is natural for profile loading; Geppetto is natural for runner/tool examples; a separate app may be cleaner.
2. **Should `outputDB` start as a copy of `inputDB`, an empty scratch DB, or a second independent Glazed export?**
3. **Should JavaScript see a `docs` table, a `sections` table, or both?** Current source says `sections`; compatibility view can provide `docs`.
4. **Should runtime state persist across tool calls?** Prebuilt runtime is faster and simpler; lazy runtime is more isolated.
5. **Should the model be allowed to write to `outputDB`?** The prompt implies two DBs, but not the exact write policy.
6. **How should profile system prompts combine with docs-chat system instructions?** Prepend, append, or profile override?
7. **Should the first REPL be line-based or support multiline prompts?** Line-based is easiest; multiline becomes useful quickly.

## Review checklist for the intern implementation

Before opening a PR, verify:

- [ ] The command uses Pinocchio `profilebootstrap`, not a custom profile loader.
- [ ] The command can run against prebuilt Glazed SQLite exports.
- [ ] `inputDB` and `outputDB` are globals in JS.
- [ ] `inputDB` cannot mutate the source DB.
- [ ] `eval_js` has timeout and output limits.
- [ ] No `fs`, `exec`, network, or broad host modules are exposed by default.
- [ ] The model-facing tool description includes schema and examples.
- [ ] Unit tests cover DB facade, runtime build, eval execution, and tool registration.
- [ ] Manual smoke test proves the model can call `eval_js` and answer from DB rows.

## References

- `/home/manuel/code/wesen/corporate-headquarters/go-go-goja/README.md`
- `/home/manuel/code/wesen/corporate-headquarters/go-go-goja/modules/database/database.go`
- `/home/manuel/code/wesen/corporate-headquarters/geppetto/pkg/inference/tools/scopedjs/schema.go`
- `/home/manuel/code/wesen/corporate-headquarters/geppetto/pkg/inference/tools/scopedjs/runtime.go`
- `/home/manuel/code/wesen/corporate-headquarters/geppetto/pkg/inference/tools/scopedjs/eval.go`
- `/home/manuel/code/wesen/corporate-headquarters/geppetto/pkg/inference/tools/scopedjs/tool.go`
- `/home/manuel/code/wesen/corporate-headquarters/geppetto/pkg/doc/topics/07-tools.md`
- `/home/manuel/code/wesen/corporate-headquarters/geppetto/pkg/doc/tutorials/07-build-scopedjs-eval-tools.md`
- `/home/manuel/code/wesen/corporate-headquarters/geppetto/pkg/doc/tutorials/08-build-streaming-tool-loop-agent-with-glazed-flags.md`
- `/home/manuel/code/wesen/corporate-headquarters/geppetto/pkg/inference/runner/types.go`
- `/home/manuel/code/wesen/corporate-headquarters/geppetto/pkg/inference/runner/options.go`
- `/home/manuel/code/wesen/corporate-headquarters/geppetto/pkg/inference/runner/prepare.go`
- `/home/manuel/code/wesen/corporate-headquarters/geppetto/pkg/inference/runner/run.go`
- `/home/manuel/code/wesen/corporate-headquarters/pinocchio/cmd/examples/simple-chat/main.go`
- `/home/manuel/code/wesen/corporate-headquarters/pinocchio/pkg/cmds/profilebootstrap/profile_selection.go`
- `/home/manuel/code/wesen/corporate-headquarters/glazed/pkg/doc/topics/01-help-system.md`
- `/home/manuel/code/wesen/corporate-headquarters/glazed/pkg/doc/topics/28-export-help-entries.md`
- `/home/manuel/code/wesen/corporate-headquarters/glazed/pkg/help/cmd/export.go`
- `/home/manuel/code/wesen/corporate-headquarters/glazed/pkg/help/store/store.go`
- Ticket evidence bundle: `sources/evidence-snippets.txt`
- Ticket full doc inventories: `sources/*-docs.txt`
