---
Title: eval_js Tool API
Slug: eval-js-api
Short: API reference for the chat agent's eval_js tool.
Topics:
  - chat
  - eval-js
  - tools
Commands:
  - chat
Flags: []
IsTopLevel: true
IsTemplate: false
ShowPerDefault: true
SectionType: GeneralTopic
Order: 10
---

# eval_js Tool API

The `eval_js` tool executes JavaScript in the chat agent's constrained go-go-goja runtime.

## Input schema

```json
{
  "code": "return inputDB.query('SELECT slug, title FROM docs LIMIT 5')",
  "input": {}
}
```

- `code` is JavaScript source. The runtime wraps it in an async function, so top-level `return` is supported.
- `input` is an optional object passed to the script as the `input` parameter.

## Output schema

```json
{
  "result": {},
  "console": [{"level": "log", "text": "message"}],
  "error": "",
  "durationMs": 12
}
```

Return JSON-serializable values from scripts. Use `console.log(...)` for diagnostics.

## Available globals

- `inputDB`: read-only database facade containing embedded chat help entries.
- `outputDB`: writable scratch database facade for derived notes and temporary tables.

The main help table is `sections`. The app also creates a compatibility view named `docs`.
