---
Title: Add streaming stdout output to chat REPL
Ticket: CHAT-STREAMING-STDOUT
Status: active
Topics:
    - geppetto
    - goja
    - glazed
    - llm-tools
DocType: index
Intent: long-term
Owners: []
RelatedFiles:
    - Path: /home/manuel/code/wesen/corporate-headquarters/geppetto/pkg/inference/runner/run.go
      Note: Runner lifecycle to inspect for streaming events
    - Path: /home/manuel/code/wesen/corporate-headquarters/geppetto/pkg/inference/runner/types.go
      Note: Runner StartRequest event sink and streaming integration point
    - Path: /home/manuel/code/wesen/obsidian-vault/Projects/2026/04/29/ARTICLE - From eval_js to Persistent Agent Runtime - Replsession Logging and Streaming Events.md
      Note: Obsidian follow-up article source
    - Path: /home/manuel/code/wesen/2026-04-29--go-go-agent/cmd/chat/main.go
      Note: Current chat REPL stdout rendering and runner invocation
    - Path: /home/manuel/code/wesen/2026-04-29--go-go-agent/cmd/chat/stream_stdout.go
      Note: stdout streaming event sink implementation
    - Path: /home/manuel/code/wesen/2026-04-29--go-go-agent/cmd/chat/stream_stdout_test.go
      Note: streaming sink formatting tests
    - Path: /home/manuel/code/wesen/2026-04-29--go-go-agent/ttmp/2026/04/29/CHAT-STREAMING-STDOUT--add-streaming-stdout-output-to-chat-repl/reference/02-article-from-eval-js-to-persistent-agent-runtime.md
      Note: Ticket-local copy of follow-up article
    - Path: /home/manuel/code/wesen/2026-04-29--go-go-agent/ttmp/2026/04/29/CHAT-STREAMING-STDOUT--add-streaming-stdout-output-to-chat-repl/sources/live-streaming-smoke-2026-04-29.txt
      Note: live streaming smoke evidence
ExternalSources: []
Summary: ""
LastUpdated: 2026-04-29T10:31:13.01104851-04:00
WhatFor: ""
WhenToUse: ""
---




# Add streaming stdout output to chat REPL

## Overview

<!-- Provide a brief overview of the ticket, its goals, and current status -->

## Key Links

- **Related Files**: See frontmatter RelatedFiles field
- **External Sources**: See frontmatter ExternalSources field

## Status

Current status: **active**

## Topics

- geppetto
- goja
- glazed
- llm-tools

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
