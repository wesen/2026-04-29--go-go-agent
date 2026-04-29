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

