---
Title: Chat Agent User Guide
Slug: user-guide
Short: Day-to-day guide for running conversations, streaming output, using eval_js, and inspecting saved runs.
Topics:
  - chat
  - user-guide
  - eval-js
  - sqlite
Commands:
  - chat run
  - chat inspect
Flags:
  - profile
  - stream
  - stream-tool-details
  - print-final-turn
  - log-db
IsTopLevel: true
IsTemplate: false
ShowPerDefault: true
SectionType: Tutorial
Order: 50
---

The chat agent is designed for interactive work where you want both a live answer and an audit trail. It streams assistant output while the model runs, lets the model call a persistent JavaScript REPL through `eval_js`, and stores final turns plus JavaScript history in a private SQLite database.

This guide describes the user-facing workflow. It avoids implementation details unless they explain behavior you can observe at the terminal.

## Running the agent

Use `chat run` for both REPL and one-shot mode:

```bash
chat run --profile gpt-5-nano-low
```

When no prompt is provided, the command starts a REPL. When prompt words are provided, it runs one request and exits:

```bash
chat run --profile gpt-5-nano-low "Summarize the available embedded help pages."
```

The `--profile` flag selects a Pinocchio profile. Profiles decide which provider and model settings Geppetto uses.

## Streaming output

Streaming is on by default:

```bash
chat run --stream
```

The stream can contain several kinds of lines:

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

Thinking output is printed whenever the provider emits plaintext thinking events. Some providers do not emit them, or emit encrypted reasoning that is only visible later in persisted blocks.

Tool details are also on by default:

```bash
chat run --stream-tool-details=true
```

Turn them off when you want quieter terminal output:

```bash
chat run --stream-tool-details=false
```

## Final transcript printing

Streaming mode suppresses the full final turn printout by default because the answer already appeared while it streamed. For debugging, you can request both streaming and final transcript rendering:

```bash
chat run --stream --print-final-turn
```

Use this when you want to compare live event output with the canonical final turn stored by the runner.

## Working with eval_js

The model has one tool: `eval_js`. It runs JavaScript as a persistent REPL cell. The final expression becomes the tool result.

Good style:

```js
const rows = inputDB.query("SELECT slug, title FROM docs ORDER BY slug LIMIT 5");
rows.map(row => row.title)
```

Persistent helper style:

```js
function shortTitle(row) {
  return row.slug + ": " + row.title;
}

const rows = inputDB.query("SELECT slug, title FROM docs LIMIT 3");
rows.map(shortTitle)
```

Later calls can use `shortTitle` because top-level declarations persist in the replsession.

Do not use top-level `return`:

```js
return rows;
```

That syntax belongs inside a function. In a REPL cell, put `rows` as the final expression.

## Database globals

The JavaScript runtime exposes two database facades:

| Global | Purpose | Typical use |
| --- | --- | --- |
| `inputDB` | Read-only embedded help database | Query documentation available to the agent |
| `outputDB` | Writable scratch database | Store notes or intermediate findings |

Example:

```js
const docs = inputDB.query(
  "SELECT slug, title FROM docs WHERE content LIKE ? LIMIT 5",
  "%eval_js%"
);

outputDB.exec(
  "INSERT INTO notes(key, value) VALUES (?, ?)",
  "last_search",
  JSON.stringify(docs)
);

docs
```

The private logging database is not exposed to JavaScript. Inspect it from the host with `chat inspect`.

## Saving a run for inspection

Pass `--log-db` when you want a stable evidence file:

```bash
chat run \
  --profile gpt-5-nano-low \
  --log-db /tmp/chat-agent.sqlite \
  "Use eval_js to inspect the docs table."
```

Then inspect it:

```bash
chat inspect sessions --log-db /tmp/chat-agent.sqlite
chat inspect eval-calls --log-db /tmp/chat-agent.sqlite
chat inspect repl-evals --log-db /tmp/chat-agent.sqlite
chat inspect turns --log-db /tmp/chat-agent.sqlite
```

Use Glazed output flags when needed:

```bash
chat inspect eval-calls --log-db /tmp/chat-agent.sqlite --output json
```

## What to inspect first

If you are debugging a run, inspect in this order:

1. `inspect schema` — confirm the file is the expected DB and has rows.
2. `inspect sessions` — find the chat and eval session IDs.
3. `inspect eval-calls` — see what the model asked the tool to run.
4. `inspect repl-evals` — see the durable JavaScript cells and results.
5. `inspect bindings` — confirm helper functions/constants persisted.
6. `inspect turns` and `inspect blocks` — inspect the conversation record.

This sequence moves from broad evidence to narrow details.

## Troubleshooting

| Problem | Cause | Solution |
| --- | --- | --- |
| `chat run` exits before answering | One-shot prompt completed or provider returned an error | Re-run with a REPL and inspect logs |
| The model uses `return` in eval_js | Old function-body examples or model habit | Tell it: “Use final expression style; no top-level return.” |
| `inputDB.exec` fails | `inputDB` is read-only | Use `outputDB.exec` for writes |
| `inspect repl-evals` result is truncated | Default table output previews long JSON | Use `--output json` or `--source` for full source |
| No final full transcript appears | Streaming suppresses final turn printing by default | Add `--print-final-turn` |

## See Also

- `chat help getting-started`
- `chat help internals`
- `chat help developer-guide`
- `chat help eval-js-api`
- `chat help database-globals`
