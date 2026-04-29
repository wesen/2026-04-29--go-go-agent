---
Title: Refactor chat CLI into run and log inspection verbs
Ticket: CHAT-CLI-VERBS
Status: active
Topics:
    - chat
    - geppetto
    - goja
    - glazed
    - sqlite
DocType: index
Intent: long-term
Owners: []
RelatedFiles:
    - Path: cmd/chat/inspect.go
      Note: Read-only log DB inspection verbs
    - Path: cmd/chat/inspect_test.go
      Note: Help separation and inspect schema tests
    - Path: cmd/chat/main.go
      Note: Root command refactored into app shell plus run verb
    - Path: ttmp/2026/04/29/CHAT-CLI-VERBS--refactor-chat-cli-into-run-and-log-inspection-verbs/reference/01-implementation-diary.md
      Note: Implementation diary
    - Path: ttmp/2026/04/29/CHAT-CLI-VERBS--refactor-chat-cli-into-run-and-log-inspection-verbs/sources/inspect-smoke-2026-04-29.txt
      Note: Manual inspect smoke evidence
ExternalSources: []
Summary: ""
LastUpdated: 2026-04-29T12:06:20.799395219-04:00
WhatFor: ""
WhenToUse: ""
---


# Refactor chat CLI into run and log inspection verbs

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
- glazed
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
