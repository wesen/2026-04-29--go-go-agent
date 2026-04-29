# Changelog

## 2026-04-29

- Initial workspace created


## 2026-04-29

Created design guide for chat run verb and log DB inspection verbs.

### Related Files

- /home/manuel/code/wesen/2026-04-29--go-go-agent/ttmp/2026/04/29/CHAT-CLI-VERBS--refactor-chat-cli-into-run-and-log-inspection-verbs/design-doc/01-chat-cli-run-and-log-inspection-verbs-guide.md — Design and implementation guide


## 2026-04-29

Implemented chat run verb and inspect sessions/eval-calls/repl-evals/bindings/turns/blocks/turn-blocks/schema verbs; added tests and smoke evidence. Implementation commit d8c8a49.

### Related Files

- /home/manuel/code/wesen/2026-04-29--go-go-agent/cmd/chat/inspect.go — Inspect command tree
- /home/manuel/code/wesen/2026-04-29--go-go-agent/cmd/chat/inspect_test.go — Regression tests
- /home/manuel/code/wesen/2026-04-29--go-go-agent/cmd/chat/main.go — Run verb and root command separation
- /home/manuel/code/wesen/2026-04-29--go-go-agent/ttmp/2026/04/29/CHAT-CLI-VERBS--refactor-chat-cli-into-run-and-log-inspection-verbs/reference/01-implementation-diary.md — Detailed diary


## 2026-04-29

Converted run and inspect verbs from hand-written Cobra handlers to Glazed command implementations. Run now implements cmds.WriterCommand; inspect leaves implement cmds.GlazeCommand and support Glazed output formats such as --output json. Commit cb4b01e.

### Related Files

- /home/manuel/code/wesen/2026-04-29--go-go-agent/cmd/chat/inspect.go — Glazed inspect commands
- /home/manuel/code/wesen/2026-04-29--go-go-agent/cmd/chat/inspect_test.go — Updated command registration tests
- /home/manuel/code/wesen/2026-04-29--go-go-agent/cmd/chat/main.go — Registers Glazed commands with root
- /home/manuel/code/wesen/2026-04-29--go-go-agent/cmd/chat/run_command.go — Glazed WriterCommand for chat run

