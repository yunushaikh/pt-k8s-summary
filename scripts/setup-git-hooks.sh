#!/usr/bin/env bash
# Point this clone at repo-managed git hooks (strips Cursor co-author trailers).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
chmod +x "$ROOT/.githooks/prepare-commit-msg"
git -C "$ROOT" config core.hooksPath .githooks
echo "Git hooks path set to .githooks (prepare-commit-msg strips Cursor attribution)."
echo "Cursor rules are local only (.cursor/rules/). Do not commit them."
