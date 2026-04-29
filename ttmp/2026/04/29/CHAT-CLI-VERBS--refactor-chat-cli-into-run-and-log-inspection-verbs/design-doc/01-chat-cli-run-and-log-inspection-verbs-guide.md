---
Title: Chat CLI run and log inspection verbs guide
Ticket: CHAT-CLI-VERBS
Status: active
Topics:
    - chat
    - geppetto
    - goja
    - glazed
    - sqlite
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: cmd/chat/main.go
      Note: Current root command and execution logic to refactor into run plus inspect verbs.
    - Path: cmd/chat/stream_stdout.go
      Note: Streaming sink remains used by the run verb.
    - Path: internal/logdb/logdb.go
      Note: Private app log DB tables chat_log_sessions and eval_tool_calls.
    - Path: /home/manuel/code/wesen/corporate-headquarters/go-go-goja/pkg/repldb/schema.go
      Note: replsession persistence tables to inspect after a run.
    - Path: /home/manuel/code/wesen/corporate-headquarters/pinocchio/pkg/persistence/chatstore/turn_store_sqlite.go
      Note: turn/block membership tables to inspect after a run.
ExternalSources: []
Summary: Design and intern guide for refactoring chat CLI help around run plus post-run SQLite inspection verbs.
LastUpdated: 2026-04-29T12:10:00-04:00
WhatFor: "Guide implementation of a Glazed-style command tree with a run verb and after-the-fact log database inspection verbs."
WhenToUse: "Use before changing cmd/chat/main.go command structure or adding SQLite inspection commands."
---

# Chat CLI `run` and log inspection verbs guide

## Executive Summary

The current chat binary uses a runnable root Cobra command named `chat`. It attaches all execution flags directly to the root command. That makes `go run ./cmd/chat --help` mix global logging/help flags with the default chat execution flags. As the program grows from a prototype into a tool-using agent with a private SQLite log database, that shape is no longer sufficient.

This ticket introduces an explicit command tree:

```text
chat
  run                    # start REPL or execute one-shot prompt
  inspect sessions       # chat/repl session overview
  inspect eval-calls     # eval_js tool-call correlation rows
  inspect repl-evals     # replsession evaluation rows
  inspect bindings       # persistent JS bindings
  inspect turns          # persisted final/intermediate turns
  inspect blocks         # unique chat blocks
  inspect turn-blocks    # turn/block membership timeline
  inspect schema         # SQLite schema/table counts
```

The root command becomes the application shell: logging setup, embedded help, and command registration. The `run` verb owns chat execution flags. The `inspect` verbs own read-only after-the-fact database inspection.

The implementation can use plain Cobra first while keeping Glazed conventions in mind: clear verbs, separated flag groups, tabular output, stable JSON output, and root-level logging/help. If we later convert each verb to full `cmds.CommandDescription` objects, this command tree remains the same.

## Problem Statement

Today:

```bash
go run ./cmd/chat --help
```

shows a single root command with everything on it:

- Pinocchio profile flags,
- input/output scratch DB flags,
- private log DB flags,
- streaming flags,
- final transcript flags,
- logging flags,
- help command wiring.

This creates three problems:

1. **Help clarity:** users cannot tell which flags are global and which flags only matter for running chat.
2. **Extensibility:** adding post-run inspection commands would make the root even more overloaded.
3. **Glazed shape:** Glazed CLIs usually reserve root for app-wide concerns and put behavior behind verbs.

The new private log DB makes inspection commands especially important. A user should not have to run raw `sqlite3` queries to answer questions like:

- Which chat sessions are in this DB?
- Which eval_js calls happened?
- What JS cells did replsession persist?
- What bindings/functions exist?
- What final turns were persisted?
- What blocks belong to a turn?
- What tables and row counts exist?

## Proposed Solution

### Command tree

```text
chat [global flags]

chat run [run flags] [prompt...]
chat inspect sessions --log-db PATH
chat inspect eval-calls --log-db PATH [--limit N] [--json]
chat inspect repl-evals --log-db PATH [--limit N] [--source]
chat inspect bindings --log-db PATH [--session-id ID]
chat inspect turns --log-db PATH [--limit N] [--json]
chat inspect blocks --log-db PATH [--limit N] [--json]
chat inspect turn-blocks --log-db PATH [--turn-id ID] [--limit N]
chat inspect schema --log-db PATH
```

### Root command responsibilities

Root command:

- initializes Glazed logging,
- loads embedded help docs,
- registers subcommands,
- does not own chat execution flags,
- does not run the chat by default.

Pseudocode:

```go
func main() {
    root := newRootCommand()
    root.AddCommand(newRunCommand())
    root.AddCommand(newInspectCommand())
    setupLogging(root)
    setupHelp(root)
    root.ExecuteContext(ctx)
}
```

### Run verb responsibilities

`chat run` contains the previous root flags:

- `--profile`
- `--config-file`
- `--profile-registries`
- `--input-db`
- `--output-db`
- `--eval-timeout`
- `--max-output-chars`
- `--log-db`
- `--log-db-strict`
- `--no-log-db`
- `--log-db-keep-temp`
- `--log-db-turn-snapshots`
- `--stream`
- `--print-final-turn`
- `--stream-tool-details`
- `--stream-max-preview-chars`

The existing `run(ctx, settings, args, in, out, errOut)` function can remain mostly unchanged. Only the command wiring moves.

### Inspect verb responsibilities

Inspection verbs open the log DB read-only where possible and print compact tables by default. They also support `--json` for exact machine-readable output.

Common inspect settings:

```go
type inspectSettings struct {
    LogDBPath string
    Limit     int
    JSON      bool
    SessionID string
    TurnID    string
    Source    bool
}
```

Common open helper:

```go
func openInspectDB(path string) (*sql.DB, error) {
    if strings.TrimSpace(path) == "" {
        return nil, fmt.Errorf("--log-db is required")
    }
    return sql.Open("sqlite3", fmt.Sprintf("file:%s?mode=ro", path))
}
```

### Inspect command behaviors

#### `inspect sessions`

Join app session rows with repl sessions where useful.

Core query:

```sql
SELECT chat_session_id, eval_session_id, conv_id, profile, log_db_path,
       started_at_ms, strict, log_schema_version
FROM chat_log_sessions
ORDER BY started_at_ms DESC;
```

#### `inspect eval-calls`

Shows tool-call correlation rows.

```sql
SELECT eval_tool_call_id, chat_session_id, eval_session_id, repl_cell_id,
       created_at_ms, error_text, code, eval_output_json
FROM eval_tool_calls
ORDER BY created_at_ms DESC
LIMIT ?;
```

Default table columns:

- id
- cell
- time
- error
- code preview
- result preview

#### `inspect repl-evals`

Shows durable replsession cells.

```sql
SELECT evaluation_id, session_id, cell_id, created_at, ok,
       error_text, raw_source, result_json
FROM evaluations
ORDER BY created_at DESC
LIMIT ?;
```

#### `inspect bindings`

Shows persistent JS bindings/functions.

```sql
SELECT b.session_id, b.name, b.latest_cell_id,
       bv.runtime_type, bv.display_value, bv.action
FROM bindings b
LEFT JOIN binding_versions bv ON bv.binding_id = b.binding_id
 AND bv.cell_id = b.latest_cell_id
WHERE (? = '' OR b.session_id = ?)
ORDER BY b.session_id, b.name;
```

#### `inspect turns`

Shows persisted turns from Pinocchio chatstore.

```sql
SELECT conv_id, session_id, turn_id, turn_created_at_ms,
       runtime_key, inference_id, updated_at_ms
FROM turns
ORDER BY updated_at_ms DESC
LIMIT ?;
```

#### `inspect blocks`

Shows unique blocks.

```sql
SELECT block_id, kind, role, first_seen_at_ms, payload_json
FROM blocks
ORDER BY first_seen_at_ms DESC
LIMIT ?;
```

#### `inspect turn-blocks`

Shows membership/timeline of blocks in turns.

```sql
SELECT conv_id, session_id, turn_id, phase,
       snapshot_created_at_ms, ordinal, block_id, content_hash
FROM turn_block_membership
WHERE (? = '' OR turn_id = ?)
ORDER BY snapshot_created_at_ms DESC, ordinal ASC
LIMIT ?;
```

#### `inspect schema`

Shows tables and row counts.

```sql
SELECT name FROM sqlite_master WHERE type='table' ORDER BY name;
SELECT COUNT(*) FROM <safe-table-name>;
```

## Output strategy

Use simple tab-separated/default table output initially. This keeps implementation low-risk and avoids mixing Glazed processor setup into the command refactor. Add `--json` to every inspect command for automation.

Default row printing helper:

```go
func printRows(out io.Writer, headers []string, rows [][]string) {
    fmt.Fprintln(out, strings.Join(headers, "\t"))
    for _, row := range rows {
        fmt.Fprintln(out, strings.Join(row, "\t"))
    }
}
```

JSON helper:

```go
func printJSON(out io.Writer, v any) error {
    enc := json.NewEncoder(out)
    enc.SetIndent("", "  ")
    return enc.Encode(v)
}
```

## Implementation Plan

1. Split command construction in `cmd/chat/main.go`:
   - `newRootCommand(ctx)`
   - `newRunCommand(ctx, *settings)`
   - `newInspectCommand()`
2. Move existing flags from root to `run`.
3. Keep `run(ctx, settings, args, ...)` unchanged.
4. Add `cmd/chat/inspect.go` with inspect settings, DB helpers, output helpers, and subcommands.
5. Add tests for:
   - root help does not show run-only flags,
   - `run --help` shows `--stream`,
   - inspect commands can read a fixture DB or temp DB.
6. Add diary entries and changelog updates.
7. Validate:
   - `go test ./... -count=1`
   - `go run ./cmd/chat --help`
   - `go run ./cmd/chat run --help`
   - `go run ./cmd/chat inspect schema --log-db /tmp/chat.sqlite`
   - `docmgr doctor --ticket CHAT-CLI-VERBS --stale-after 30`

## Acceptance Criteria

- `chat --help` is root/app help and does not list run-only flags like `--stream`.
- `chat run --help` lists run flags.
- `chat run` starts the REPL.
- `chat run "prompt"` executes a one-shot prompt.
- `chat inspect schema --log-db PATH` shows tables and row counts.
- `chat inspect eval-calls --log-db PATH` shows eval_js calls.
- `chat inspect repl-evals --log-db PATH` shows replsession cells.
- `chat inspect bindings --log-db PATH` shows persisted JS bindings.
- `chat inspect turns --log-db PATH` shows persisted turns.
- `chat inspect blocks --log-db PATH` shows unique blocks.
- `chat inspect turn-blocks --log-db PATH` shows turn membership.
- Inspect commands support `--json`.
- Tests and docmgr doctor pass.
