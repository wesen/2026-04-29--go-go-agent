---
Title: chat REPL User Guide
Slug: chat-repl-user-guide
Short: How to use the stdin/stdout chat REPL.
Topics:
  - chat
  - repl
  - profiles
Commands:
  - chat run
Flags:
  - profile
  - profile-registries
  - config-file
IsTopLevel: true
IsTemplate: false
ShowPerDefault: true
SectionType: Tutorial
Order: 30
---

The `chat run` command starts a stdin/stdout REPL backed by Geppetto inference and Pinocchio profile resolution.

## Basic usage

```bash
chat run --profile openai-fast
```

Type a message and press Enter. Use `:quit` to exit and `:reset` to clear the in-memory conversation.

## Useful prompts

Ask the model to inspect the embedded help database:

```text
Use eval_js to list the available help entries and summarize what APIs I can call.
```

Ask for a targeted lookup:

```text
Use eval_js to find documentation about outputDB.exec and give me a short example.
```

## Profile loading

The app uses Pinocchio's standard profile bootstrap path. Use `--profile`, `--profile-registries`, and `--config-file` to select the same profile sources used by Pinocchio commands.
