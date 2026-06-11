# pt-k8s-summary

Generate a single **HTML report** from a Kubernetes cluster dump collected with [pt-k8s-debug-collector](https://github.com/percona/k8spxc-debug-collector).

The report summarizes nodes, Percona XtraDB Cluster (PXC) resources, backups, pod logs, events, certificates, and related configuration.

**Current version:** check with `-version` or see [CHANGELOG.md](CHANGELOG.md) / [GitHub Releases](https://github.com/yunushaikh/pt-k8s-summary/releases).

## Requirements

- **Input:** cluster dump `.tar.gz` / `.tgz` or an extracted dump directory (`nodes.yaml`, PXC YAML, `pods.yaml`, pod folders, etc.)
- **Network (optional):** only if `-certified-images=true` (default) — fetches Percona certified image lists
- **Optional:** [Percona Toolkit](https://docs.percona.com/percona-toolkit/) with `pt-galera-log-explainer` on `PATH` for the Galera timeline section in the report

Cluster dumps are **not** in this repo. You provide your own archive or extracted tree.

## Download pre-built binaries

Binaries are published on **[GitHub Releases](https://github.com/yunushaikh/pt-k8s-summary/releases)** only (not committed to git).

| Your machine | Download asset |
|--------------|----------------|
| Linux x86_64 (most servers, WSL, Intel/AMD PC) | `pt-k8s-summary_linux_amd64` |
| Linux ARM64 (Graviton, Ampere, ARM VMs) | `pt-k8s-summary_linux_arm64` |
| macOS Apple Silicon (M1/M2/M3) | `pt-k8s-summary_darwin_arm64` |
| macOS Intel | `pt-k8s-summary_darwin_amd64` |

Windows is not supported. Build from source on Windows only if you accept an unsupported, untested path.

### Install (Linux / macOS)

```bash
# Example: Apple Silicon Mac — pick the asset that matches your machine
VERSION=v0.4.0
curl -fsSL -o pt-k8s-summary \
  "https://github.com/yunushaikh/pt-k8s-summary/releases/download/${VERSION}/pt-k8s-summary_darwin_arm64"
chmod +x pt-k8s-summary
./pt-k8s-summary -version
```

Verify checksums from `SHA256SUMS` on the same release page.

### macOS first run

Downloaded binaries may be blocked by Gatekeeper. Either run from Terminal after `chmod +x`, or right-click the binary → **Open** once. You do not need to install Go to use a release binary.

### Flags must come before the archive path

```bash
# Good
./pt-k8s-summary -certified-images=false /path/to/dump.tar.gz

# Bad — -certified-images is ignored (Go flag parsing)
./pt-k8s-summary /path/to/dump.tar.gz -certified-images=false
```

## Usage

### From a tarball

```bash
./pt-k8s-summary /path/to/cluster-dump.tar.gz
```

Default output: `reports/<archive-stem>-summary.html`

### From an extracted dump directory

```bash
./pt-k8s-summary -dump /path/to/cluster-dump
```

Default output: `reports/report.html`

### Common options

```bash
# Custom output path
./pt-k8s-summary -dump ./cluster-dump -out reports/my-report.html

# Offline (no certified-image network fetch)
./pt-k8s-summary -dump ./cluster-dump -certified-images=false

# Galera timeline: only events on/after a time (needs pt-galera-log-explainer on PATH)
./pt-k8s-summary -dump ./cluster-dump -galera-since 2023-01-05T03:24:26.000000Z
```

## Flags

| Flag | Description |
|------|-------------|
| `-version` | Print version and exit |
| `-dump` | Path to an extracted cluster dump root |
| `-nodes` | Path to `nodes.yaml` (default: `<dump>/nodes.yaml`) |
| `-out` | Output HTML path |
| `-galera-since` | RFC3339 timestamp for pt-galera-log-explainer `--since=` |
| `-certified-images` | Fetch/compare Percona certified images (default: `true`) |

Positional argument: cluster dump archive (`.tar.gz` or `.tgz`). Use either the archive **or** `-dump`, not both.

## Reading the report

| Section | Tips |
|---------|------|
| **Processing details** | Shows dump inputs; collector `errors.txt` if present |
| **PXC · pod logs** | Formatted vs Full logs; large files use sidecar `_logs/` directory |
| **Nodes** | Collapsible table with **filter** |
| **Backups** | Collapsed by default; **filter** matches status/columns (not log text); **View logs** in Logs column |
| **Events** | Collapsible, filterable, newest first |
| **Footer** | Report shows `pt-k8s-summary` version used to generate it |

### Large pod logs

Embedded logs are capped at ~750 KiB per file. Larger logs are copied next to the report:

```
reports/my-report.html
reports/my-report_logs/<namespace>/<pod>/…/mysqld-error.log
reports/my-report_logs/<namespace>/<pod>/…/mysqld-error.log.formatted
```

When sharing a report, include the HTML **and** the `_logs` folder. Use **Load full** / **Open in new tab** in the viewer for truncated embeds.

## Build from source

Requires Go 1.21+. The repo root has `go.work` so you can build without `cd`:

```bash
git clone https://github.com/yunushaikh/pt-k8s-summary.git
cd pt-k8s-summary
mkdir -p bin
go build -o bin/pt-k8s-summary ./pt-k8s-summary
./bin/pt-k8s-summary -version
```

Or use `./pt-k8s-summary/build.sh` (writes to `bin/pt-k8s-summary`).

## Project layout

```
pt-k8s-summary/          Go module (main.go, templates, internal/)
reports/                 Generated HTML (gitignored)
docs/RELEASE.md          Maintainer release checklist
CHANGELOG.md             Version history and release notes
```

## Development

```bash
cd pt-k8s-summary
go test ./...
```

Some tests need a local cluster-dump fixture and skip when missing.

Maintainers: [docs/RELEASE.md](docs/RELEASE.md).

## Roadmap and design notes

These choices keep the project maintainable as it grows:

| Topic | Approach |
|-------|----------|
| **Distribution** | Multi-arch binaries on GitHub Releases only; `SHA256SUMS` per release |
| **Report format** | Single self-contained HTML + optional `_logs/` sidecars (no server required) |
| **New sections** | Add `SectionCollector` under `internal/collector/`, register in `registry.go` |
| **PXC / backup logic** | Prefer `internal/jpreport/` for cluster-specific parsing |
| **Versioning** | SemVer; user-visible changes in `CHANGELOG.md` before each tag |
| **Breaking changes** | Bump MAJOR when CLI flags or report contracts change; aim for `v1.0.0` when API stabilizes |

Possible future improvements (not committed yet):

- Config file for default flags and output paths
- `pullKnownFlags` extended so flags work after the archive path
- JSON or machine-readable summary export alongside HTML
- Report section toggles (include/exclude sections)
- Integration test with a minimal checked-in fixture tarball
- Homebrew formula / apt metadata (community or official)

## What is not in git

- Generated HTML reports (`reports/`, `*_logs/`)
- Cluster dump archives and extracted trees
- Pre-built binaries (use [GitHub Releases](https://github.com/yunushaikh/pt-k8s-summary/releases))
- Local `bin/` build output
