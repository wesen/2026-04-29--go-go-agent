#!/usr/bin/env bash
set -euo pipefail
BASE=/home/manuel/code/wesen/corporate-headquarters
OUT_DIR="${1:-.}"
mkdir -p "$OUT_DIR"

# Doc inventories excluding ttmp.
for repo in go-go-goja glazed geppetto pinocchio; do
  (
    cd "$BASE/$repo"
    printf '== %s ==\n' "$repo"
    rg --files \
      -g '*.md' -g '*.mdx' -g '*.txt' -g '*.rst' -g '*.adoc' \
      -g '!**/ttmp/**' -g '!ttmp/**' -g '!**/.git/**' -g '!**/node_modules/**' -g '!**/dist/**' \
      | sort
  ) > "$OUT_DIR/${repo}-docs.txt"
done

# Helper for line-numbered snippets.
snippet() {
  local file=$1 start=$2 end=$3 label=$4
  printf '\n== %s (%s:%s-%s) ==\n' "$label" "$file" "$start" "$end"
  nl -ba "$file" | awk -v s="$start" -v e="$end" 'NR>=s && NR<=e {print}'
}

{
  snippet "$BASE/go-go-goja/README.md" 1 105 "go-go-goja runtime API and module flags"
  snippet "$BASE/go-go-goja/modules/database/database.go" 17 69 "database module options and preconfigured DB support"
  snippet "$BASE/go-go-goja/modules/database/database.go" 97 180 "database module JS API and query implementation"
  snippet "$BASE/geppetto/pkg/inference/tools/scopedjs/schema.go" 1 83 "scopedjs public contracts"
  snippet "$BASE/geppetto/pkg/inference/tools/scopedjs/runtime.go" 41 82 "scopedjs BuildRuntime lifecycle"
  snippet "$BASE/geppetto/pkg/inference/tools/scopedjs/eval.go" 19 55 "scopedjs RunEval contract"
  snippet "$BASE/geppetto/pkg/inference/tools/scopedjs/tool.go" 11 44 "scopedjs RegisterPrebuilt"
  snippet "$BASE/geppetto/pkg/doc/topics/07-tools.md" 29 116 "Geppetto tool-loop mental model"
  snippet "$BASE/geppetto/cmd/examples/runner-glazed-registry-flags/main.go" 37 92 "Geppetto runner profile-flag example"
  snippet "$BASE/pinocchio/cmd/examples/simple-chat/main.go" 80 143 "Pinocchio simple chat profile resolution"
  snippet "$BASE/pinocchio/pkg/cmds/profilebootstrap/profile_selection.go" 36 72 "Pinocchio bootstrap config and wrappers"
  snippet "$BASE/glazed/pkg/doc/topics/28-export-help-entries.md" 1 122 "Glazed help export docs"
  snippet "$BASE/glazed/pkg/doc/topics/01-help-system.md" 20 92 "Glazed help system and section shape"
} > "$OUT_DIR/evidence-snippets.txt"
