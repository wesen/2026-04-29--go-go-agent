---
Title: eval_js Tool API
Slug: eval-js-api
Short: API reference for the chat agent's eval_js tool.
Topics:
  - chat
  - eval-js
  - tools
Commands:
  - chat run
Flags: []
IsTopLevel: true
IsTemplate: false
ShowPerDefault: true
SectionType: GeneralTopic
Order: 10
---

The `eval_js` tool executes JavaScript in the chat agent's constrained go-go-goja runtime. The code runs as a persistent REPL cell, so top-level declarations survive later tool calls and the final expression becomes the result.

## Input schema

```json
{
  "code": "const rows = inputDB.query('SELECT slug, title FROM docs LIMIT 5'); rows",
  "input": {}
}
```

- `code` is JavaScript source evaluated as a replsession cell. Use a final expression for the result; do not use top-level `return`.
- `input` is an optional object exposed as the per-call `input` global.

## Output schema

```json
{
  "result": {},
  "console": [{"level": "log", "text": "message"}],
  "error": "",
  "durationMs": 12
}
```

Make the final expression JSON-serializable when you want a structured tool result. Use `console.log(...)` for diagnostics. If the final expression is a function or `undefined`, the tool returns metadata such as `kind` and `preview` instead of silently dropping the value.

## Available globals

- `inputDB`: read-only database facade containing embedded chat help entries.
- `outputDB`: writable scratch database facade for derived notes and temporary tables.
- `input`: per-call input object.
- `globalThis`: canonical persistent global object.
- `window`: alias of `globalThis` for browser-style snippets.
- `global`: alias of `globalThis` for Node-style snippets.

The main help table is `sections`. The app also creates a compatibility view named `docs`.
