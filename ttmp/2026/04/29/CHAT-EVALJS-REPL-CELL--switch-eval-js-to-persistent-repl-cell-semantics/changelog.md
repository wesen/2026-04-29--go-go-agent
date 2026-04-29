# Changelog

## 2026-04-29

- Initial workspace created


## 2026-04-29

Created intern-oriented design and implementation guide for switching eval_js to persistent REPL-cell semantics, including go-go-goja exact-result API work if needed.

### Related Files

- /home/manuel/code/wesen/2026-04-29--go-go-agent/ttmp/2026/04/29/CHAT-EVALJS-REPL-CELL--switch-eval-js-to-persistent-repl-cell-semantics/design-doc/01-repl-cell-eval-js-semantics-implementation-guide.md — Detailed REPL-cell eval_js guide


## 2026-04-29

Uploaded REPL-cell eval_js implementation guide to reMarkable at /ai/2026/04/29/CHAT-EVALJS-REPL-CELL.


## 2026-04-29

Implemented REPL-cell eval_js semantics: go-go-goja now exposes exact ResultJSON envelopes, go-go-agent evaluates tool code as direct replsession cells, and live smoke proved a helper function persisted across separate eval_js calls.

### Related Files

- /home/manuel/code/wesen/2026-04-29--go-go-agent/internal/evaljs/runtime.go — Updated tool docs for final-expression semantics
- /home/manuel/code/wesen/2026-04-29--go-go-agent/internal/logdb/eval_tool.go — Direct REPL-cell eval_js evaluation and per-call globals
- /home/manuel/code/wesen/2026-04-29--go-go-agent/ttmp/2026/04/29/CHAT-EVALJS-REPL-CELL--switch-eval-js-to-persistent-repl-cell-semantics/sources/live-repl-cell-eval-js-smoke-2026-04-29.txt — Live validation evidence
- /home/manuel/code/wesen/corporate-headquarters/go-go-goja/pkg/replsession/evaluate.go — Exact result JSON envelope support
- /home/manuel/code/wesen/corporate-headquarters/go-go-goja/pkg/replsession/types.go — ExecutionReport ResultJSON field


## 2026-04-29

Committed implementation: go-go-goja exact ResultJSON support in 848db80 and go-go-agent REPL-cell eval_js adapter in ed87553. Validation passed: go-go-goja go test ./... -count=1, go-go-agent go test ./... -count=1, docmgr doctor, and live smoke.

### Related Files

- /home/manuel/code/wesen/2026-04-29--go-go-agent/ttmp/2026/04/29/CHAT-EVALJS-REPL-CELL--switch-eval-js-to-persistent-repl-cell-semantics/sources/live-repl-cell-eval-js-smoke-2026-04-29.txt — Live smoke output and DB counts

