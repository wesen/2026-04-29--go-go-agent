# go-go-agent

`go-go-agent` is a small but production-shaped terminal chat agent. It combines a Geppetto/Pinocchio LLM conversation loop, a persistent go-go-goja JavaScript tool runtime, Glazed command/help ergonomics, and a private SQLite evidence database that can be inspected after a run.

The project started as a simple `eval_js` chat prototype. It is now a useful reference implementation for building tool-using Go agents where every important step can be streamed live and audited later.

## What it does

- Runs an interactive chat REPL or a one-shot prompt with `chat run`.
- Resolves model/provider settings through Pinocchio profiles.
- Gives the model one tool, `eval_js`, backed by a persistent go-go-goja `replsession`.
- Exposes safe JavaScript globals for embedded documentation and scratch work:
  - `inputDB` — read-only embedded help/documentation SQLite facade.
  - `outputDB` — writable scratch SQLite facade.
  - `input` — per-call tool input object.
  - `globalThis`, `window`, `global` — persistent JavaScript global object aliases.
- Streams assistant, thinking, and tool progress to stdout while the model runs.
- Persists final turns, JavaScript cells, eval tool calls, bindings, and blocks into a private host-only SQLite database.
- Provides `chat inspect ...` verbs for after-the-fact log database inspection.
- Embeds Glazed help pages that are available both through `chat help ...` and the `inputDB` documentation tables.

## Why this exists

Most toy chat agents answer a prompt and then forget what happened. This project takes the next step: it treats the agent run as something you should be able to replay, inspect, and explain.

That means the live terminal output is not the only artifact. A run can leave behind a SQLite database containing:

- chat sessions,
- final conversation turns,
- persisted message/tool/reasoning blocks,
- `eval_js` tool call correlations,
- durable go-go-goja REPL cells,
- persistent JavaScript bindings,
- console output and result envelopes.

This makes the project useful as both an agent and an architecture example.

## Installation

Build the binary:

```bash
go build -o ./dist/chat ./cmd/chat
```

Or use the Makefile:

```bash
make build
make install
```

`make install` builds `./dist/chat` and copies it over the `chat` binary on your `PATH` if one already exists.

## Quick start

Start an interactive REPL:

```bash
chat run --profile gpt-5-nano-low
```

Run a one-shot prompt:

```bash
chat run --profile gpt-5-nano-low \
  "Use eval_js to list three embedded help pages."
```

Keep an inspectable log database:

```bash
chat run \
  --profile gpt-5-nano-low \
  --log-db /tmp/chat-agent.sqlite \
  "Use eval_js to query the docs table and summarize the result."
```

Inspect it afterward:

```bash
chat inspect schema --log-db /tmp/chat-agent.sqlite
chat inspect eval-calls --log-db /tmp/chat-agent.sqlite
chat inspect repl-evals --log-db /tmp/chat-agent.sqlite
chat inspect turns --log-db /tmp/chat-agent.sqlite
```

Use Glazed output formats when scripting:

```bash
chat inspect eval-calls --log-db /tmp/chat-agent.sqlite --output json
```

## Command overview

```text
chat
  run                    Run the REPL or one-shot prompt
  inspect sessions       List chat log sessions
  inspect eval-calls     List eval_js tool call correlation rows
  inspect repl-evals     List replsession evaluation cells
  inspect bindings       List persistent JavaScript bindings
  inspect turns          List persisted chat turns
  inspect blocks         List unique persisted chat blocks
  inspect turn-blocks    List turn/block membership rows
  inspect schema         List SQLite tables and row counts
  help                   Browse embedded Glazed help pages
```

The root command is intentionally not the chat runner. Run-specific flags live under `chat run`, while root/global flags are reserved for logging and help.

```bash
chat --help
chat run --help
chat inspect --help
```

## The `eval_js` tool

The model can call a single tool named `eval_js`. The tool executes JavaScript in a persistent go-go-goja `replsession`.

The important rule is:

> `eval_js` code is a REPL cell. The result is the final expression. Do not use top-level `return`.

Good:

```js
const rows = inputDB.query("SELECT slug, title FROM docs ORDER BY slug LIMIT 3");
rows
```

Persistent helper functions work naturally:

```js
function titleOf(row) {
  return row.title;
}

titleOf
```

A later tool call can use the same function:

```js
const rows = inputDB.query("SELECT title FROM docs ORDER BY slug LIMIT 1");
titleOf(rows[0])
```

This is possible because top-level declarations are captured by `replsession` and mirrored back into the persistent JavaScript runtime.

## Streaming behavior

Streaming is enabled by default for `chat run`:

```bash
chat run --stream
```

The stream can include:

```text
thinking: ...
assistant: ...
[tool eval_js call call_...]
code:
...
[tool eval_js running call_...]
[tool eval_js done call_...]
result:
...
```

Thinking is printed whenever the provider emits plaintext thinking events. If no `thinking:` line appears, the provider likely did not emit such events for that run.

Tool details are also enabled by default:

```bash
chat run --stream-tool-details=true
```

Disable them for quieter output:

```bash
chat run --stream-tool-details=false
```

## Private log database

When logging is enabled, the app stores data in a private SQLite database. If you do not provide `--log-db`, the app uses a temporary database. Provide an explicit path when you want to inspect the run later.

```bash
chat run --log-db /tmp/chat-agent.sqlite --profile gpt-5-nano-low
```

The database contains three families of tables:

| Family | Tables | Purpose |
| --- | --- | --- |
| App log | `chat_log_sessions`, `eval_tool_calls` | Connect chat sessions to eval_js repl cells |
| Replsession | `sessions`, `evaluations`, `bindings`, `binding_versions`, `binding_docs`, `console_events` | Persist JavaScript cells and bindings |
| Chatstore | `turns`, `blocks`, `turn_block_membership` | Persist final/snapshot conversation turns and blocks |

The private log DB is host-only. It is not exposed to JavaScript as `inputDB`, `outputDB`, or any other model-visible global.

## Inspecting a run

Start broad:

```bash
chat inspect schema --log-db /tmp/chat-agent.sqlite
chat inspect sessions --log-db /tmp/chat-agent.sqlite
```

Then inspect tool execution:

```bash
chat inspect eval-calls --log-db /tmp/chat-agent.sqlite
chat inspect repl-evals --log-db /tmp/chat-agent.sqlite
chat inspect bindings --log-db /tmp/chat-agent.sqlite
```

Then inspect conversation persistence:

```bash
chat inspect turns --log-db /tmp/chat-agent.sqlite
chat inspect blocks --log-db /tmp/chat-agent.sqlite
chat inspect turn-blocks --log-db /tmp/chat-agent.sqlite
```

All inspect leaf commands are Glazed commands. They support standard Glazed output flags such as:

```bash
--output json
--fields field1,field2
```

## Embedded help

The binary embeds several Glazed help pages:

```bash
chat help getting-started
chat help user-guide
chat help internals
chat help developer-guide
chat help eval-js-api
chat help database-globals-api
chat help chat-repl-user-guide
```

These same help entries are materialized into the embedded `inputDB` documentation database, so the model can inspect them with `eval_js`.

## Development

Run tests:

```bash
go test ./... -count=1
```

Run the focused chat command tests:

```bash
go test ./cmd/chat -count=1 -v
```

Run lint:

```bash
make lint
```

Build:

```bash
make build
```

The repo currently uses a local `replace` directive for `go-go-goja` during development because the chat agent depends on recent `replsession` result-envelope behavior:

```text
replace github.com/go-go-golems/go-go-goja => /home/manuel/code/wesen/corporate-headquarters/go-go-goja
```

If that repository is not present at the expected local path, either check it out there or update `go.mod` to point at a released `go-go-goja` version that contains the required `ExecutionReport.ResultJSON` support.

## Architecture map

```text
cmd/chat
  main.go              Root command, logging/help setup, Glazed registration
  run_command.go       Glazed WriterCommand for chat run
  inspect.go           Glazed inspect commands over SQLite logs
  stream_stdout.go     Geppetto EventSink for live stdout

internal/evaljs
  runtime.go           eval_js tool metadata and runtime globals

internal/logdb
  logdb.go             private DB setup and app log tables
  eval_tool.go         replapi-backed eval_js adapter
  turn_persister.go    final/snapshot turn persistence

internal/helpdocs
  help/*.md            embedded Glazed help pages
```

## Design guarantees

The current implementation tries to preserve these guarantees:

1. `eval_js` uses persistent REPL-cell semantics, not function-body wrappers.
2. Final expression values are returned through exact result JSON envelopes.
3. Top-level JavaScript declarations persist across tool calls.
4. The private logging database is never exposed to JavaScript.
5. Streaming is display-only; the final `turns.Turn` remains canonical.
6. Thinking deltas are printed whenever `--stream` is true and the provider emits them.
7. Inspect commands are read-only and Glazed row emitters.
8. Root help stays separate from run-specific flags.

## Troubleshooting

| Problem | Likely cause | Fix |
| --- | --- | --- |
| `--stream` is not in `chat --help` | `--stream` belongs to `chat run` | Run `chat run --help` |
| `eval_js` errors on `return ...` | `eval_js` uses REPL-cell semantics | Use a final expression instead |
| A helper function is missing later | It was not declared at top level or the cell failed | Check `chat inspect repl-evals` and `chat inspect bindings` |
| No thinking output appears | Provider did not emit plaintext thinking events | Inspect persisted blocks for reasoning artifacts |
| Inspect command needs JSON | Glazed defaults to table output | Add `--output json` |
| `--log-db is required` | Inspect commands need a saved DB path | Run `chat run --log-db /path/to/db.sqlite` first |

## License

This repository currently does not declare a license file. Add one before distributing outside the intended development environment.
