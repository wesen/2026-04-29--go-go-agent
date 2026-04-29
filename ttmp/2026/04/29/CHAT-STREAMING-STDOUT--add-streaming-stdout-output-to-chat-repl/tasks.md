# Tasks

## TODO

- [ ] Add tasks here

- [x] Investigate Geppetto runner streaming/event APIs and current chat stdout rendering
- [x] Design stdout streaming behavior for assistant tokens, tool calls, tool results, and final transcript rendering
- [x] Implement streaming event sink wiring in cmd/chat without breaking final turn persistence
- [x] Add tests or smoke checks for streaming and non-streaming output behavior
- [x] Update diary/changelog and validate ticket with docmgr doctor
- [x] Implement stdout EventSink with formatting tests
- [x] Add --stream and --print-final-turn flags and wire runPrompt EventSinks
- [x] Expand streaming eval_js tool call code and tool results in REPL output
- [x] Add opt-in streaming support for thinking/reasoning token deltas
