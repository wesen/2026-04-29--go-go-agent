---
Title: Switch eval_js to persistent REPL-cell semantics
Ticket: CHAT-EVALJS-REPL-CELL
Status: active
Topics:
    - chat
    - geppetto
    - goja
    - llm-tools
    - sqlite
DocType: index
Intent: long-term
Owners: []
RelatedFiles:
    - Path: /home/manuel/code/wesen/corporate-headquarters/go-go-goja/pkg/replsession/evaluate.go
      Note: Exact result JSON envelope computation
    - Path: /home/manuel/code/wesen/corporate-headquarters/go-go-goja/pkg/replsession/observe.go
      Note: Persist result carries LastValueJSON
    - Path: /home/manuel/code/wesen/corporate-headquarters/go-go-goja/pkg/replsession/result_json_test.go
      Note: ResultJSON regression tests
    - Path: /home/manuel/code/wesen/corporate-headquarters/go-go-goja/pkg/replsession/rewrite.go
      Note: Final expression trimming cleanup
    - Path: /home/manuel/code/wesen/corporate-headquarters/go-go-goja/pkg/replsession/types.go
      Note: ExecutionReport ResultJSON API field
    - Path: /home/manuel/code/wesen/2026-04-29--go-go-agent/internal/evaljs/runtime.go
      Note: Updated tool docs and final-expression examples
    - Path: /home/manuel/code/wesen/2026-04-29--go-go-agent/internal/logdb/eval_tool.go
      Note: Direct REPL-cell eval_js adapter and per-call globals
    - Path: /home/manuel/code/wesen/2026-04-29--go-go-agent/internal/logdb/eval_tool_test.go
      Note: Persistent declaration/input/error tests
    - Path: /home/manuel/code/wesen/2026-04-29--go-go-agent/internal/logdb/privacy_test.go
      Note: Privacy test updated to REPL-cell final expression style
    - Path: /home/manuel/code/wesen/2026-04-29--go-go-agent/ttmp/2026/04/29/CHAT-EVALJS-REPL-CELL--switch-eval-js-to-persistent-repl-cell-semantics/sources/live-repl-cell-eval-js-smoke-2026-04-29.txt
      Note: Live smoke evidence for persistent REPL-cell eval_js declarations
ExternalSources: []
Summary: ""
LastUpdated: 2026-04-29T11:41:51.689646095-04:00
WhatFor: ""
WhenToUse: ""
---



# Switch eval_js to persistent REPL-cell semantics

## Overview

<!-- Provide a brief overview of the ticket, its goals, and current status -->

## Key Links

- **Related Files**: See frontmatter RelatedFiles field
- **External Sources**: See frontmatter ExternalSources field

## Status

Current status: **active**

## Topics

- chat
- geppetto
- goja
- llm-tools
- sqlite

## Tasks

See [tasks.md](./tasks.md) for the current task list.

## Changelog

See [changelog.md](./changelog.md) for recent changes and decisions.

## Structure

- design/ - Architecture and design documents
- reference/ - Prompt packs, API contracts, context summaries
- playbooks/ - Command sequences and test procedures
- scripts/ - Temporary code and tooling
- various/ - Working notes and research
- archive/ - Deprecated or reference-only artifacts
