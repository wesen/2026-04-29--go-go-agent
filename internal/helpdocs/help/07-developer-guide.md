---
Title: Chat Agent Developer Guide
Slug: developer-guide
Short: Developer guide for changing commands, eval_js behavior, streaming, and log inspection safely.
Topics:
  - chat
  - developer-guide
  - glazed
  - goja
  - sqlite
Commands:
  - chat run
  - chat inspect
Flags:
  - log-db
  - stream
  - output
IsTopLevel: true
IsTemplate: false
ShowPerDefault: true
SectionType: GeneralTopic
Order: 70
---

The chat agent is a compact integration point for several libraries: Glazed for command structure and help, Geppetto for inference, go-go-goja for JavaScript execution, and Pinocchio chatstore for turn persistence. A safe change usually touches more than one layer.

This guide explains how to make changes without breaking the core guarantees: clean CLI help, persistent `eval_js` state, stream-only display behavior, and private host-only logging.

## Repository map

Start with these files:

| Path | Role |
| --- | --- |
| `cmd/chat/main.go` | Root command, logging/help setup, Glazed command registration, chat runtime orchestration |
| `cmd/chat/run_command.go` | Glazed `run` command definition and flag decoding |
| `cmd/chat/inspect.go` | Glazed `inspect` commands and read-only SQLite queries |
| `cmd/chat/stream_stdout.go` | Geppetto event sink for live stdout streaming |
| `internal/evaljs/runtime.go` | Tool description, runtime globals, and eval_js registration |
| `internal/logdb/eval_tool.go` | replapi-backed eval_js adapter and correlation persistence |
| `internal/logdb/logdb.go` | private SQLite app tables and replsession store setup |
| `internal/logdb/turn_persister.go` | final/snapshot turn persistence |
| `internal/helpdocs/help/*.md` | Embedded Glazed help pages and materialized `inputDB` documentation |

When a behavior changes, update both code and help. The help pages are not external docs; they are embedded into the binary and queryable by the model through `inputDB`.

## Command development

Behavioral commands should be Glazed commands.

Use this rule:

- Use `cmds.WriterCommand` when the command owns human-oriented streaming or REPL output.
- Use `cmds.GlazeCommand` when the command emits structured rows.

`chat run` is a writer command because it writes a conversation stream. `chat inspect ...` commands are Glaze commands because they emit database rows.

A minimal Glazed command looks like this:

```go
type MyCommand struct {
    *cmds.CommandDescription
}

type MySettings struct {
    Limit int `glazed:"limit"`
}

func NewMyCommand() (*MyCommand, error) {
    desc := cmds.NewCommandDescription(
        "my-command",
        cmds.WithShort("Explain what this command does"),
        cmds.WithFlags(
            fields.New("limit", fields.TypeInteger, fields.WithDefault(20)),
        ),
    )
    return &MyCommand{CommandDescription: desc}, nil
}
```

For row output:

```go
func (c *MyCommand) RunIntoGlazeProcessor(
    ctx context.Context,
    vals *values.Values,
    gp middlewares.Processor,
) error {
    settings := &MySettings{}
    if err := vals.DecodeSectionInto(schema.DefaultSlug, settings); err != nil {
        return err
    }
    return gp.AddRow(ctx, types.NewRow(types.MRP("limit", settings.Limit)))
}
```

## Root command expectations

The root command is still a Cobra command because Glazed integrates with Cobra for command-line parsing. That does not mean behavior should be hand-written as Cobra flag handlers.

The root should:

- add logging flags once;
- load embedded help once;
- call `help_cmd.SetupCobraRootCommand` once;
- register Glazed child commands;
- avoid run-specific flags on root.

The root should not:

- manually define `--stream`, `--profile`, or `--log-db`;
- run chat by default;
- create a second help system in child commands.

## Avoiding command settings conflicts

Glazed has a built-in command settings section with flags like `--config-file`. The chat run command also needs a Pinocchio `--config-file`. Register these commands with `SkipCommandSettingsSection: true` unless you intentionally rename one of the flags.

The current registration uses the parser config directly:

```go
cli.CobraParserConfig{
    ShortHelpSections: []string{schema.DefaultSlug},
    MiddlewaresFunc: cli.CobraCommandDefaultMiddlewares,
    SkipCommandSettingsSection: true,
}
```

If you remove this, the command may fail during startup with a duplicate flag error.

## eval_js development

`eval_js` must preserve REPL-cell semantics:

- evaluate user code directly as a replsession cell;
- do not wrap user code in an async function body;
- final expression is the result;
- top-level declarations persist;
- top-level `return` remains invalid;
- private log DB remains host-only.

The adapter should set per-call globals through `replapi.WithRuntime`, not by prepending source code:

```go
ReplApp.WithRuntime(ctx, sessionID, func(rt *engine.Runtime) error {
    vm := rt.VM
    vm.Set("input", input)
    global := vm.GlobalObject()
    global.Set("window", global)
    global.Set("global", global)
    return nil
})
```

This keeps the user's cell source clean and lets replsession analyze top-level declarations correctly.

## Streaming development

Streaming output is a display layer over Geppetto events. Do not use streamed deltas as the canonical transcript.

The invariant is:

```text
streaming events -> human feedback
final turns.Turn -> durable conversation state
```

Thinking deltas are printed when `--stream` is true and the provider emits plaintext thinking events. Do not add a separate thinking flag unless there is a product decision to hide them again.

Tool details are controlled by `--stream-tool-details`. If you add new tool event rendering, keep it separate from assistant text so the user can distinguish answer content from operational traces.

## Log inspection development

Inspect commands should open the database read-only:

```go
sql.Open("sqlite3", "file:" + absPath + "?mode=ro&_busy_timeout=5000")
```

Use parameterized SQL for values. The only acceptable dynamic SQL today is safe table counting in `inspect schema`, and table names must pass identifier validation before interpolation.

When adding an inspect command:

1. Add a new Glazed command definition in `NewInspectCommands`.
2. Add a `kind` case in `inspectRows`.
3. Return rows plus an explicit header order.
4. Preview large JSON/text columns by default.
5. Add a test with a temp SQLite fixture.
6. Update this help page if the command changes the user workflow.

## Testing checklist

Run these before committing:

```bash
go test ./cmd/chat -count=1 -v
go test ./... -count=1
```

For CLI behavior:

```bash
chat --help
chat run --help
chat inspect --help
chat inspect schema --log-db /tmp/chat.sqlite
chat inspect schema --log-db /tmp/chat.sqlite --output json
```

For `eval_js` behavior, test two separate tool calls:

1. define a helper function as a top-level declaration;
2. call it in a later `eval_js` call;
3. inspect `bindings`, `eval-calls`, and `repl-evals`.

## Documentation checklist

Whenever behavior changes, update the embedded help pages:

- user-facing workflow goes in `user-guide`;
- architecture and data flow go in `internals`;
- implementation guidance goes in `developer-guide`;
- first-run instructions go in `getting-started`.

Then verify the pages load:

```bash
chat help getting-started
chat help internals
chat help user-guide
chat help developer-guide
```

## Troubleshooting

| Problem | Cause | Solution |
| --- | --- | --- |
| Duplicate `--config-file` error | Glazed command settings section conflicts with Pinocchio flag | Use `SkipCommandSettingsSection: true` or rename one flag |
| Inspect command prints plain text instead of JSON | Glazed defaults to table output | Pass `--output json` |
| Helper functions no longer persist | User code may be wrapped again | Ensure eval_js evaluates direct replsession cells |
| Private DB appears in JavaScript | A host-only handle was exposed by mistake | Remove the global and add a privacy test |
| Root help lists run flags | Flags were added to root instead of `run` | Move them into the Glazed `RunCommand` |

## See Also

- `chat help getting-started`
- `chat help user-guide`
- `chat help internals`
- `chat help eval-js-api`
- `chat help database-globals`
