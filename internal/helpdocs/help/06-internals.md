---
Title: Chat Agent Internals
Slug: internals
Short: Conceptual map of the runtime, eval_js, streaming events, and private SQLite persistence model.
Topics:
  - chat
  - internals
  - geppetto
  - goja
  - sqlite
Commands:
  - chat run
  - chat inspect
Flags:
  - log-db
  - stream
IsTopLevel: true
IsTemplate: false
ShowPerDefault: true
SectionType: GeneralTopic
Order: 60
---

The chat agent is small at the command line but layered internally. Each layer has a separate job: command parsing, model execution, JavaScript tool execution, event streaming, and persistence. Understanding those layers helps you debug behavior without guessing.

This page is a conceptual map. It explains how the pieces fit together and why the system is structured this way.

## System overview

The system looks like this:

```text
chat CLI
  |
  |-- chat run
  |     |
  |     |-- Pinocchio profile bootstrap
  |     |-- Geppetto runner
  |     |-- stdout event sink
  |     |-- eval_js tool
  |     |     |
  |     |     |-- go-go-goja replapi
  |     |     |-- replsession persistent cells
  |     |     |-- inputDB / outputDB globals
  |     |
  |     |-- private SQLite log DB
  |
  |-- chat inspect
        |
        |-- read-only SQLite queries
        |-- Glazed tabular / JSON output
```

The important design choice is separation of concerns. The model sees a safe JavaScript environment with `inputDB` and `outputDB`. The host keeps a separate private logging database that records turns and tool calls. The model cannot query that private database directly.

## Command layer

The command layer uses Glazed command descriptions for behavioral verbs. The root command is an application shell. It loads embedded help, installs logging flags, and registers commands.

`chat run` is a writer command because it streams human-facing text to stdout. It does not emit a normal result table. `chat inspect ...` commands are Glazed row-emitting commands because they report structured database rows.

This distinction matters:

| Command family | Glazed shape | Output style |
| --- | --- | --- |
| `chat run` | `cmds.WriterCommand` | REPL text, streaming events, final answers |
| `chat inspect ...` | `cmds.GlazeCommand` | Rows, tables, JSON, CSV, other Glazed formats |

## Inference layer

The inference layer is Geppetto. `chat run` resolves a Pinocchio profile and builds a Geppetto runner request. The runner owns the canonical final turn. Streaming is display-only; it does not replace the final turn.

The runner request can include event sinks. The chat agent adds a stdout sink when `--stream` is true. That sink receives partial assistant text, thinking deltas, tool-call lifecycle events, tool results, and errors.

```text
runner.Start(request)
  -> provider streams events
  -> stdoutStreamSink prints live output
  -> runner returns final turns.Turn
  -> turn persister stores final blocks
```

This design prevents a common bug: treating the stream as the authoritative transcript. The stream is for humans. The final turn is the durable conversation state.

## Streaming layer

The stdout sink handles several event types:

| Event | Printed form | Purpose |
| --- | --- | --- |
| assistant partial | `assistant: ...` | Live answer text |
| thinking partial | `thinking: ...` | Provider reasoning when emitted in plaintext |
| tool call | `[tool eval_js call ...]` | Tool request boundary |
| tool execute | `[tool eval_js running ...]` | Execution started |
| tool result | `[tool eval_js done ...]` plus result | Execution completed |
| error | `[error] ...` | Visible error reporting |

Thinking streaming is always on when `--stream` is on. If no thinking text appears, the provider probably did not emit plaintext thinking deltas.

## JavaScript layer

The model has one tool: `eval_js`. It executes JavaScript in a persistent go-go-goja replsession.

Earlier prototypes treated tool code as a function body. The current system treats tool code as a REPL cell. This is why top-level declarations persist:

```js
function titleOf(row) {
  return row.title;
}

titleOf
```

A later cell can use `titleOf`:

```js
const rows = inputDB.query("SELECT title FROM docs LIMIT 1");
titleOf(rows[0])
```

The result is the final expression value. Top-level `return` is invalid because this is a real cell, not a generated wrapper function.

## JavaScript globals

The runtime exposes these user-visible globals:

| Global | Description |
| --- | --- |
| `inputDB` | Read-only facade over embedded help tables |
| `outputDB` | Writable scratch SQLite facade |
| `input` | Per-call input object from the tool request |
| `globalThis` | Canonical persistent global object |
| `window` | Alias of `globalThis` for browser-style snippets |
| `global` | Alias of `globalThis` for Node-style snippets |

The private log database is deliberately absent. Do not add it as a global.

## Persistence layer

The private SQLite database combines three table families:

| Family | Tables | Source |
| --- | --- | --- |
| app log | `chat_log_sessions`, `eval_tool_calls` | `internal/logdb` |
| replsession | `sessions`, `evaluations`, `bindings`, `binding_versions`, `binding_docs`, `console_events` | go-go-goja repldb |
| turn store | `turns`, `blocks`, `turn_block_membership` | Pinocchio chatstore |

The app log tables connect the chat world to the JavaScript world. For example, `eval_tool_calls.repl_cell_id` points to the replsession cell that executed the tool code.

## Inspection layer

`chat inspect` opens the SQLite database read-only. It does not need the model, provider credentials, or a live runtime. It is a forensic layer over persisted data.

Use it when:

- a run succeeded but you want to know what happened;
- a run failed and you need the exact JavaScript source;
- you want to verify final-only turn persistence;
- you want to see whether a helper function persisted;
- you want to inspect reasoning/tool/message blocks.

## Failure modes

The layers make failures easier to localize:

| Symptom | Likely layer | Inspection path |
| --- | --- | --- |
| No answer streamed | provider or runner | Check terminal error and final turn |
| Tool code failed | eval_js / replsession | `chat inspect eval-calls`, then `repl-evals` |
| Helper missing later | replsession binding persistence | `chat inspect bindings` |
| Log DB missing rows | log persistence | `chat inspect schema`, then row counts |
| Thinking not shown | provider event mapping | Inspect blocks for reasoning artifacts |

## See Also

- `chat help getting-started`
- `chat help user-guide`
- `chat help developer-guide`
- `chat help eval-js-api`
- `chat help database-globals`
