---
Title: Getting Started with the Chat Agent
Slug: getting-started
Short: First steps for running the chat agent, choosing a profile, and inspecting a log database afterward.
Topics:
  - chat
  - getting-started
  - profiles
  - sqlite
Commands:
  - chat
  - chat run
  - chat inspect
Flags:
  - profile
  - log-db
  - stream
IsTopLevel: true
IsTemplate: false
ShowPerDefault: true
SectionType: Tutorial
Order: 40
---

The chat agent is a terminal program that combines an LLM conversation loop, a persistent JavaScript tool runtime, and a private SQLite log database. You use `chat run` to talk to the model, and you use `chat inspect ...` later to understand what happened during that run.

This page teaches the smallest useful workflow first. The goal is not to explain every subsystem immediately. The goal is to help you run one conversation, preserve its evidence, and inspect the result without reading source code.

## The mental model

Think of the system as three cooperating parts:

```text
chat run
  -> talks to an LLM through Geppetto and Pinocchio profiles
  -> offers one JavaScript tool named eval_js
  -> writes final turns and eval_js history into a private SQLite DB

chat inspect
  -> opens that private SQLite DB read-only
  -> prints sessions, turns, JavaScript cells, bindings, and tool calls
```

This split matters because the live conversation is only half the product. The log database is the evidence trail. It lets you answer questions such as “Which JavaScript did the model run?” and “Which final assistant blocks were persisted?” after the terminal session is gone.

## Start a REPL

Run the chat REPL with a Pinocchio profile:

```bash
chat run --profile gpt-5-nano-low
```

The program prints a prompt:

```text
chat REPL. Type :help for commands, :quit to exit.
>
```

Type a message and press Enter. Use `:quit` to exit and `:reset` to clear the in-memory conversation seed.

If you do not pass `--profile`, profile resolution still follows the standard Pinocchio bootstrap path. In practice, most users should pass an explicit profile while learning because it makes runs reproducible.

## Run a one-shot prompt

You can also pass a prompt as positional arguments:

```bash
chat run --profile gpt-5-nano-low "Use eval_js to list three embedded help sections."
```

A one-shot prompt is useful for smoke tests and examples. The REPL is better when you want an iterative conversation.

## Keep the log database

By default, the private log database may be temporary. When you want to inspect a run afterward, provide an explicit path:

```bash
chat run \
  --profile gpt-5-nano-low \
  --log-db /tmp/chat-agent.sqlite \
  "Use eval_js to define a helper function and then query the docs table."
```

The log database is host-only. It is not exposed to JavaScript as a model-visible global. The model can use `inputDB` and `outputDB`, but not the private tables that record turns and tool calls.

## Inspect the run afterward

After the run finishes, inspect the database:

```bash
chat inspect schema --log-db /tmp/chat-agent.sqlite
```

This shows table names and row counts. Then inspect the tool calls:

```bash
chat inspect eval-calls --log-db /tmp/chat-agent.sqlite
```

And inspect the underlying replsession cells:

```bash
chat inspect repl-evals --log-db /tmp/chat-agent.sqlite
```

The difference is important:

| Command | What it shows | Why you use it |
| --- | --- | --- |
| `inspect eval-calls` | The chat tool-call correlation rows | To see what the LLM asked `eval_js` to run |
| `inspect repl-evals` | Durable JavaScript REPL cells | To see how go-go-goja persisted the cell |
| `inspect bindings` | Persistent JS bindings | To see functions/constants available to later tool calls |
| `inspect turns` | Persisted chat turns | To see final conversation snapshots |
| `inspect blocks` | Unique message/tool/reasoning blocks | To inspect persisted content blocks |

## Ask the model to use eval_js

The tool available to the model is named `eval_js`. It executes JavaScript as a persistent REPL cell. That means the result is the final expression, not a top-level `return` statement.

Good:

```js
const rows = inputDB.query("SELECT slug, title FROM docs ORDER BY slug LIMIT 3");
rows
```

Also good, because the function persists:

```js
function titleOf(row) {
  return row.title;
}

titleOf
```

Later:

```js
const rows = inputDB.query("SELECT title FROM docs ORDER BY slug LIMIT 1");
titleOf(rows[0])
```

Avoid top-level `return`:

```js
return rows;
```

That is invalid in a real REPL cell. Put the desired value as the final expression instead.

## Use Glazed output for inspection

Inspect commands are Glazed commands, so you can choose output formats:

```bash
chat inspect schema --log-db /tmp/chat-agent.sqlite --output json
```

This is useful when another script needs to consume the result. The default output is human-readable table output.

## Troubleshooting

| Problem | Cause | Solution |
| --- | --- | --- |
| `--stream` does not appear in `chat --help` | `--stream` belongs to `chat run`, not the root command | Run `chat run --help` |
| `inspect` says `--log-db is required` | Inspect commands need an existing SQLite DB path | Re-run with `--log-db /path/to/chat.sqlite` |
| No thinking text appears while streaming | The provider did not emit plaintext thinking events | Streaming is enabled; inspect persisted blocks for provider reasoning artifacts |
| A JavaScript helper is not available later | The helper may have been declared inside a local function or the call failed | Use top-level REPL-cell declarations and check `chat inspect bindings` |
| Tool result is missing for a function final expression | Functions are not JSON values | The result includes `kind` and `preview` metadata for non-JSON values |

## See Also

- `chat help user-guide`
- `chat help internals`
- `chat help developer-guide`
- `chat help eval-js-api`
- `chat help database-globals`
