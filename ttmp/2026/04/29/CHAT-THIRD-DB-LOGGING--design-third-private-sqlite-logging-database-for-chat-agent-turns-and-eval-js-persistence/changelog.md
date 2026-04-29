# Changelog

## 2026-04-29

- Initial workspace created


## 2026-04-29

Re-uploaded replapi-only design PDF, added implementation tasks, and completed private log DB lifecycle/schema/eval session scaffolding.

### Related Files

- /home/manuel/code/wesen/2026-04-29--go-go-agent/cmd/chat/main.go — Initial CLI lifecycle wiring for private log DB
- /home/manuel/code/wesen/2026-04-29--go-go-agent/internal/evaljs/runtime.go — Replapi-backed eval_js registration contract and engine factory setup
- /home/manuel/code/wesen/2026-04-29--go-go-agent/internal/logdb/logdb.go — Private log DB Open/Close


## 2026-04-29

Completed task 10 private log DB lifecycle and replapi session scaffolding (commit 6ef38c4447f755fe2dff5ce31dddb04932b8f663).

### Related Files

- /home/manuel/code/wesen/2026-04-29--go-go-agent/internal/evaljs/runtime.go — EvalTool injection and engine factory setup
- /home/manuel/code/wesen/2026-04-29--go-go-agent/internal/logdb/logdb.go — Private log DB lifecycle and schema setup
- /home/manuel/code/wesen/2026-04-29--go-go-agent/internal/logdb/logdb_test.go — Schema/session setup coverage


## 2026-04-29

Completed task 11 by adding replapi-backed eval_js execution tests and validating repldb plus eval_tool_calls persistence.

### Related Files

- /home/manuel/code/wesen/2026-04-29--go-go-agent/internal/logdb/eval_tool.go — Replapi-backed eval_js implementation
- /home/manuel/code/wesen/2026-04-29--go-go-agent/internal/logdb/eval_tool_test.go — Direct success/error persistence tests


## 2026-04-29

Completed task 11 eval_js replapi behavior test coverage (commit a45a973a1ab1531934f1a63bcee4ede604a1f9cf).

### Related Files

- /home/manuel/code/wesen/2026-04-29--go-go-agent/internal/logdb/eval_tool_test.go — Success/error eval_js persistence coverage


## 2026-04-29

Completed task 12 CLI flag and runner hook wiring; verified chat help lists log DB flags (commit 6ef38c4447f755fe2dff5ce31dddb04932b8f663).

### Related Files

- /home/manuel/code/wesen/2026-04-29--go-go-agent/cmd/chat/main.go — CLI flags


## 2026-04-29

Completed task 13 test coverage for schema creation, replapi eval persistence, turn snapshots, serialization errors, and JS non-exposure.

### Related Files

- /home/manuel/code/wesen/2026-04-29--go-go-agent/internal/logdb/eval_tool_test.go — Eval success/error/serialization persistence tests
- /home/manuel/code/wesen/2026-04-29--go-go-agent/internal/logdb/logdb_test.go — Schema and eval session setup test
- /home/manuel/code/wesen/2026-04-29--go-go-agent/internal/logdb/privacy_test.go — Private table non-exposure test
- /home/manuel/code/wesen/2026-04-29--go-go-agent/internal/logdb/turn_persister_test.go — Turn snapshot persistence test


## 2026-04-29

Completed task 13 persistence/privacy test coverage (commit 7504e6c32329943ee641b93a3b195ce710a18342).

### Related Files

- /home/manuel/code/wesen/2026-04-29--go-go-agent/internal/logdb/privacy_test.go — JS non-exposure coverage
- /home/manuel/code/wesen/2026-04-29--go-go-agent/internal/logdb/turn_persister_test.go — Turn snapshot coverage


## 2026-04-29

Completed task 14 final validation: go test ./... passed, docmgr doctor passed, and implementation bundle uploaded to reMarkable.

### Related Files

- /home/manuel/code/wesen/2026-04-29--go-go-agent/ttmp/2026/04/29/CHAT-THIRD-DB-LOGGING--design-third-private-sqlite-logging-database-for-chat-agent-turns-and-eval-js-persistence/reference/01-investigation-diary.md — Final validation and reMarkable upload evidence


## 2026-04-29

Live LLM tmux smoke test found replsession preview truncation in eval_js result conversion; fixed EvalTool to read exact JSON from runtime global via replapi.WithRuntime, then verified inputDB, outputDB, turn store, repldb, and correlation rows.

### Related Files

- /home/manuel/code/wesen/2026-04-29--go-go-agent/internal/logdb/eval_tool.go — Fix exact result extraction for replapi-backed eval_js
- /home/manuel/code/wesen/2026-04-29--go-go-agent/ttmp/2026/04/29/CHAT-THIRD-DB-LOGGING--design-third-private-sqlite-logging-database-for-chat-agent-turns-and-eval-js-persistence/sources/live-llm-smoke-2026-04-29.txt — Live tmux smoke evidence and DB counts


## 2026-04-29

Committed live smoke fix for exact replapi eval result extraction (commit 9a6ea2d9f2c06168c8779b2745e295ad4f48c94d); live tmux test verified inputDB, outputDB, private turn store, repldb, and correlation rows.

### Related Files

- /home/manuel/code/wesen/2026-04-29--go-go-agent/internal/logdb/eval_tool.go — Exact JSON result extraction from live repl runtime
- /home/manuel/code/wesen/2026-04-29--go-go-agent/ttmp/2026/04/29/CHAT-THIRD-DB-LOGGING--design-third-private-sqlite-logging-database-for-chat-agent-turns-and-eval-js-persistence/sources/live-llm-smoke-2026-04-29.txt — Live tmux smoke evidence


## 2026-04-29

Changed turn persistence defaults: final turns are persisted by default, while intermediate turn snapshots require --log-db-turn-snapshots; live tmux check verified only final memberships are written without the flag.

### Related Files

- /home/manuel/code/wesen/2026-04-29--go-go-agent/cmd/chat/main.go — Added --log-db-turn-snapshots and gated SnapshotHook wiring
- /home/manuel/code/wesen/2026-04-29--go-go-agent/ttmp/2026/04/29/CHAT-THIRD-DB-LOGGING--design-third-private-sqlite-logging-database-for-chat-agent-turns-and-eval-js-persistence/design-doc/01-private-logging-database-for-chat-agent-turns-and-eval-js-execution.md — Updated CLI flag design
- /home/manuel/code/wesen/2026-04-29--go-go-agent/ttmp/2026/04/29/CHAT-THIRD-DB-LOGGING--design-third-private-sqlite-logging-database-for-chat-agent-turns-and-eval-js-persistence/sources/live-final-only-turn-persistence-2026-04-29.txt — Final-only live verification evidence


## 2026-04-29

Committed final-only turn persistence default and opt-in intermediate snapshots via --log-db-turn-snapshots (commit 17e6e71023dde1e78f981b4a54b22d19db98f93b).

### Related Files

- /home/manuel/code/wesen/2026-04-29--go-go-agent/cmd/chat/main.go — SnapshotHook is attached only when --log-db-turn-snapshots is enabled
- /home/manuel/code/wesen/2026-04-29--go-go-agent/ttmp/2026/04/29/CHAT-THIRD-DB-LOGGING--design-third-private-sqlite-logging-database-for-chat-agent-turns-and-eval-js-persistence/sources/live-final-only-turn-persistence-2026-04-29.txt — Final-only live verification evidence

