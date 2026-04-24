#!/usr/bin/env bash
# Back up the previous binary, then build a new pt-k8s-summary at the repo root.
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BIN="$REPO_ROOT/pt-k8s-summary"
mkdir -p "$REPO_ROOT/backups"
if [[ -f "$BIN" ]]; then
	ts="$(date +%Y%m%d-%H%M%S)"
	cp -a "$BIN" "$REPO_ROOT/backups/pt-k8s-summary.${ts}.bak"
	cp -a "$BIN" "$REPO_ROOT/backups/pt-k8s-summary.latest.bak"
	echo "Backed up previous binary to backups/pt-k8s-summary.${ts}.bak"
fi
cd "$(dirname "$0")"
go build -o "$BIN" .
echo "Built $BIN"
