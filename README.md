# pt-k8s-summary

Generate an HTML report from a Kubernetes cluster dump collected with [pt-k8s-debug-collector](https://github.com/percona/k8spxc-debug-collector).

The report summarizes cluster nodes, Percona XtraDB Cluster (PXC) resources, backups, pod logs, events, and related configuration from the dump.

## Requirements

- Go 1.21 or later
- Input: a cluster dump tarball (`.tar.gz` / `.tgz`) or an extracted dump directory

Cluster dump data is **not** included in this repository. You must provide your own archive or extracted tree (typically containing `nodes.yaml`, PXC YAML files, `pods.yaml`, and related artifacts).

## Clone

```bash
git clone git@github.com:yunushaikh/pt-k8s-summary.git
cd pt-k8s-summary
```

Or with HTTPS:

```bash
git clone https://github.com/yunushaikh/pt-k8s-summary.git
cd pt-k8s-summary
```

## Build

The Go module lives in `pt-k8s-summary/` (where `go.mod` is). The repo root includes a `go.work` file so you can build from the repository root without `cd`:

```bash
mkdir -p bin
go build -o bin/pt-k8s-summary ./pt-k8s-summary
```

Alternatively, build from inside the module:

```bash
cd pt-k8s-summary
go build -o ../bin/pt-k8s-summary .
```

Verify the binary:

```bash
./bin/pt-k8s-summary -h
```

### Alternative: build script

The module includes a helper script that backs up any previous binary and builds into `bin/`:

```bash
./pt-k8s-summary/build.sh
```

Then run:

```bash
./bin/pt-k8s-summary -h
```

## Usage

### From a tarball

```bash
./bin/pt-k8s-summary /path/to/cluster-dump.tar.gz
```

Default output: `reports/<archive-stem>-summary.html`

### From an extracted dump directory

```bash
./bin/pt-k8s-summary -dump /path/to/cluster-dump
```

Default output: `reports/report.html`

### Custom output path

```bash
./bin/pt-k8s-summary -dump ./cluster-dump -out reports/my-report.html
```

### Offline mode (skip certified image fetch)

By default the tool may fetch Percona certified image lists over the network. To disable that:

```bash
./bin/pt-k8s-summary -dump ./cluster-dump -certified-images=false
```

### Galera log timeline filter

To limit Galera log analysis to events on or after a given time:

```bash
./bin/pt-k8s-summary -dump ./cluster-dump -galera-since 2023-01-05T03:24:26.000000Z
```

## Flags

| Flag | Description |
|------|-------------|
| `-version` | Print version and exit |
| `-dump` | Path to an extracted cluster dump root |
| `-nodes` | Path to `nodes.yaml` (default: `<dump>/nodes.yaml`) |
| `-out` | Output HTML path |
| `-galera-since` | RFC3339 timestamp passed to pt-galera-log-explainer `--since=` |
| `-certified-images` | Fetch and compare Percona certified images (default: `true`) |

Positional argument: cluster dump archive (`.tar.gz` or `.tgz`). Use either the archive **or** `-dump`, not both.

## Large pod logs

Pod logs embedded in the HTML are capped at ~750 KiB per file so the report stays usable in a browser. When a log exceeds that limit, the tool writes the **full** raw and formatted copies next to the report:

```
reports/my-report.html
reports/my-report_logs/<namespace>/<pod>/…/mysqld-error.log
reports/my-report_logs/<namespace>/<pod>/…/mysqld-error.log.formatted
```

In the report viewer, truncated logs show a banner with **Load full**, **Open in new tab**, and **Download**. Keep the `_logs` directory alongside the HTML when sharing the report.

## Download releases

- **From git:** [`releases/bin/pt-k8s-summary-linux-amd64`](releases/bin/pt-k8s-summary-linux-amd64) — latest pre-built Linux binary (see [`releases/README.md`](releases/README.md))
- [GitHub Releases](https://github.com/yunushaikh/pt-k8s-summary/releases) — same binary attached to each tag
- [CHANGELOG.md](CHANGELOG.md) — release history and what changed in each version
- [docs/RELEASE.md](docs/RELEASE.md) — how to cut a release (maintainers)

Check your build:

```bash
./bin/pt-k8s-summary -version
```

## Project layout

```
pt-k8s-summary/          Main Go module and source
  main.go                CLI entry point
  template_html.go       HTML template assembly (go:embed)
  report_*.html          Embedded report templates
  *.tmpl                 Additional template fragments
  internal/collector/    Dump parsing and report sections
  internal/jpreport/     PXC, backup, and pod report logic
  build.sh               Build helper script
reports/                 Generated HTML output (gitignored)
bin/                     Suggested location for the built binary (gitignored)
```

## Development

Run tests from the module directory:

```bash
cd pt-k8s-summary
go test ./...
```

Some tests require a local cluster dump fixture and are skipped when the fixture is not present.

## What is not in git

The following are intentionally excluded (see `.gitignore`):

- Generated HTML reports (`reports/`, `*.html` except embedded templates)
- Cluster dump archives and extracted dump directories
- Built binaries and backup copies
- Local logs
