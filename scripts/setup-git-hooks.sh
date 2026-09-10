#!/usr/bin/env bash
# Point this clone at repo-managed git hooks (strips Cursor co-author trailers).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
chmod +x "$ROOT/.githooks/prepare-commit-msg"
git -C "$ROOT" config core.hooksPath .githooks
mkdir -p "$ROOT/.cursor/rules"
for rule in no-cursor-agent-contributor.mdc feature-documentation.mdc; do
  if [[ ! -f "$ROOT/.cursor/rules/$rule" ]]; then
    cp "$ROOT/docs/agent-rules/$rule" "$ROOT/.cursor/rules/"
    echo "Installed local Cursor rule $rule from docs/agent-rules/ (not tracked in git)."
  fi
done
echo "Git hooks path set to .githooks (prepare-commit-msg strips Cursor attribution)."
