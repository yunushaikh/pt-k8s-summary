# Usage

Examples assume `pt-k8s-summary` is on your `PATH` after [install](INSTALL.md). If you [built from source](BUILD.md), use `./bin/pt-k8s-summary` instead.

## Quick start

### From a tarball

```bash
pt-k8s-summary /path/to/cluster-dump.tar.gz
```

Default output: `reports/<archive-stem>-summary.html`

### From an extracted dump directory

```bash
pt-k8s-summary -dump /path/to/cluster-dump
```

Default output: `reports/report.html`

## Common options

```bash
# Custom output path
pt-k8s-summary -dump ./cluster-dump -out reports/my-report.html

# Offline (no certified-image network fetch)
pt-k8s-summary -dump ./cluster-dump -certified-images=false

# Galera timeline: only events on/after a time (needs pt-galera-log-explainer on PATH)
pt-k8s-summary -dump ./cluster-dump -galera-since 2023-01-05T03:24:26.000000Z

# Experimental grouped layout (technology tabs: Kubernetes | PXC | Percona Server)
pt-k8s-summary -dump ./cluster-dump -layout grouped -out reports/grouped.html

# Print version
pt-k8s-summary -version
```

## Flags

| Flag | Description |
|------|-------------|
| `-version` | Print version and exit |
| `-dump` | Path to an extracted cluster dump root |
| `-nodes` | Path to `nodes.yaml` (default: auto-detect at `<dump>/nodes.yaml` or `<dump>/cluster-scope/nodes.yaml`) |
| `-out` | Output HTML path |
| `-galera-since` | RFC3339 timestamp for pt-galera-log-explainer `--since=` |
| `-certified-images` | Fetch/compare Percona certified images (default: `true`) |
| `-layout` | Report layout: `classic` (default, linear sections) or `grouped` (beta: tabbed Kubernetes / PXC / Percona Server) |

**Positional argument:** cluster dump archive (`.tar.gz` or `.tgz`). Use either the archive **or** `-dump`, not both.

### Grouped layout (beta)

`-layout grouped` reorganizes the HTML report into tabs:

| Tab | Content |
|-----|---------|
| **Kubernetes** | Always shown — nodes, cluster events, PVC inventory |
| **Percona XtraDB Cluster** | Shown only when PXC CRs, backups, pod logs, or PXC collector sections exist |
| **Percona Server for MySQL** | Shown only when PS CRs, backups, pod logs, or PS collector sections exist |

Compare layouts from the same dump:

```bash
pt-k8s-summary cluster-dump.tar.gz -out reports/classic.html
pt-k8s-summary cluster-dump.tar.gz -layout grouped -out reports/grouped.html
```

Classic layout remains the default until you decide to adopt grouped permanently.

## Flag order

Flags must appear **before** the archive path:

```bash
# Good
pt-k8s-summary -certified-images=false /path/to/dump.tar.gz

# Bad — flag after archive is ignored
pt-k8s-summary /path/to/dump.tar.gz -certified-images=false
```

## Requirements

- **Input:** cluster dump from [pt-k8s-debug-collector](https://github.com/percona/k8spxc-debug-collector) (tarball or extracted tree with `nodes.yaml` at the dump root or under `cluster-scope/`, PXC YAML, `pods.yaml`, pod folders, etc.)
- **Network (optional):** only when `-certified-images=true` (default)
- **Optional:** [Percona Toolkit](https://docs.percona.com/percona-toolkit/) with `pt-galera-log-explainer` on `PATH` for the Galera timeline section

## Reading the HTML report

| Section | Tips |
|---------|------|
| **Processing details** | Dump inputs; collector `errors.txt` if present |
| **PXC · pod logs** | Formatted vs Full logs; large files use sidecar `_logs/` directory |
| **Nodes** | Collapsible table with **filter** |
| **Backups** | Collapsed by default; **filter** matches status/columns (not log text); **View logs** in Logs column |
| **Events** | Collapsible, filterable, newest first |
| **Footer** | Shows `pt-k8s-summary` version used to generate the report |

### Large pod logs

Embedded logs are capped at ~750 KiB per file. Larger logs are copied next to the report:

```
reports/my-report.html
reports/my-report_logs/<namespace>/<pod>/…/mysqld-error.log
reports/my-report_logs/<namespace>/<pod>/…/mysqld-error.log.formatted
```

When sharing a report, include the HTML **and** the `_logs` folder. In the viewer, use **Load full** / **Open in new tab** for truncated embeds.

## See also

- [INSTALL.md](INSTALL.md) — download and install
- [CHANGELOG.md](../CHANGELOG.md) — what changed in each version
