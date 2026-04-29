#!/usr/bin/env bash
set -euo pipefail
OUT_DIR="${1:-.}"
mkdir -p "$OUT_DIR"
ROOT=/home/manuel/code/wesen/2026-04-29--go-go-agent
CH=/home/manuel/code/wesen/corporate-headquarters

# Relevant source/doc inventory for postprocessing.
{
  echo "# Current chat app files"
  (cd "$ROOT" && find cmd internal -type f | sort)
  echo
  echo "# Geppetto docs"
  (cd "$CH/geppetto" && rg --files -g '*.md' -g '!**/ttmp/**' pkg/doc cmd/examples | sort | rg 'turn|session|runner|tool|profile|sqlite|structured|event' || true)
  echo
  echo "# Pinocchio persistence docs/code-adjacent docs"
  (cd "$CH/pinocchio" && rg --files -g '*.md' -g '!**/ttmp/**' pkg cmd | sort | rg 'chat|webchat|profile|runtime|persistence|timeline|turn|sqlite' || true)
  echo
  echo "# go-go-goja REPL persistence docs/code-adjacent docs"
  (cd "$CH/go-go-goja" && rg --files -g '*.md' -g '!**/ttmp/**' pkg cmd | sort | rg 'repl|session|docs|api|persistence|sqlite' || true)
} > "$OUT_DIR/relevant-files-and-docs.txt"

snippet() {
  local file=$1 start=$2 end=$3 label=$4
  printf '\n== %s (%s:%s-%s) ==\n' "$label" "$file" "$start" "$end"
  nl -ba "$file" | awk -v s="$start" -v e="$end" 'NR>=s && NR<=e {print}'
}

{
  snippet "$ROOT/cmd/chat/main.go" 1 220 "current chat app setup, profile resolution, runner, REPL"
  snippet "$ROOT/internal/evaljs/runtime.go" 1 220 "current eval_js scopedjs runtime"
  snippet "$ROOT/internal/jsdb/facade.go" 1 180 "current JS database facade"
  snippet "$CH/geppetto/pkg/inference/toolloop/enginebuilder/builder.go" 17 230 "Geppetto TurnPersister and runner persistence hook"
  snippet "$CH/geppetto/pkg/inference/runner/types.go" 1 90 "Geppetto runner.Runtime and StartRequest include Persister"
  snippet "$CH/geppetto/pkg/inference/runner/prepare.go" 17 120 "Geppetto runner prepares session/turn and applies metadata"
  snippet "$CH/geppetto/pkg/turns/serde/serde.go" 1 85 "Turn YAML serialization for Pinocchio chatstore"
  snippet "$CH/pinocchio/pkg/persistence/chatstore/turn_store.go" 1 42 "Pinocchio TurnStore interface"
  snippet "$CH/pinocchio/pkg/persistence/chatstore/turn_store_sqlite.go" 40 130 "Pinocchio SQLiteTurnStore constructor and schema"
  snippet "$CH/pinocchio/pkg/persistence/chatstore/turn_store_sqlite.go" 228 385 "Pinocchio SQLiteTurnStore Save normalizes turn blocks"
  snippet "$CH/go-go-goja/pkg/replapi/app.go" 1 180 "go-go-goja replapi app/session/evaluate/history"
  snippet "$CH/go-go-goja/pkg/replapi/config.go" 1 170 "go-go-goja replapi profiles and persistent config"
  snippet "$CH/go-go-goja/pkg/replsession/policy.go" 1 145 "replsession persistent policy"
  snippet "$CH/go-go-goja/pkg/replsession/service.go" 40 180 "replsession service persistence and session creation"
  snippet "$CH/go-go-goja/pkg/replsession/persistence.go" 1 95 "replsession persistCell writes evaluation records"
  snippet "$CH/go-go-goja/pkg/repldb/schema.go" 1 90 "repldb SQLite schema"
  snippet "$CH/go-go-goja/pkg/repldb/write.go" 1 145 "repldb CreateSession and PersistEvaluation"
} > "$OUT_DIR/evidence-snippets.txt"
