# Development

## Project layout

```
pt-k8s-summary/          Go module (main.go, templates, internal/)
  internal/collector/    Pluggable dump report sections
  internal/jpreport/     PXC, backup, pod report logic
reports/                 Generated HTML output (gitignored)
docs/                    User and maintainer documentation
CHANGELOG.md             Version history
```

### Per-technology files

Keep operator-specific code in separate files so a change for one technology does not break others:

| Technology | Collector sections | Report data |
|------------|-------------------|-------------|
| **Common** | `events_section.go`, `pvc_section.go` | `nodes.yaml` |
| **PXC** | `pxc_*_section.go`, `pxc_certificates_section.go` | `internal/jpreport/pxc.go` |
| **Percona Server** | `ps_*_section.go`, `ps_certificates_section.go` | `internal/jpreport/ps.go`, `ps_cr_load.go` |
| **PostgreSQL** | `pg_certificates_section.go`, `pg_pod_logs_section.go` | `jpreport/pg.go`, `pg_backup.go`, … |
| **Future** | `mongo_*`, … (same pattern) | matching `jpreport/` loaders |

Shared helpers (e.g. OpenSSL cert parsing, dump file location) live in neutral `*_common.go` or `internal/dumpfiles/locate.go`. Each section implements `Group()` for the grouped tab layout.

## Tests

```bash
cd pt-k8s-summary
go test ./...
```

Some tests require a local cluster-dump fixture and skip when it is not present.

## Adding a report section

1. Add `*_section.go` under `internal/collector/`
2. Implement `SectionCollector` (see `section.go`)
3. Register in `registry.go`
4. Add user-facing notes to `CHANGELOG.md` when shipping

PXC/backup/pod logic: prefer `internal/jpreport/`.

## Roadmap and design notes

| Topic | Approach |
|-------|----------|
| **Distribution** | Multi-arch binaries on GitHub Releases only; `SHA256SUMS` per release |
| **Report format** | Single HTML + optional `_logs/` sidecars (no server required) |
| **Versioning** | SemVer; user-visible changes in `CHANGELOG.md` before each tag |
| **Breaking changes** | Bump MAJOR when CLI or report contracts change; target `v1.0.0` when stable |

Possible future work:

- Config file for default flags and output paths
- Flags accepted after the archive positional argument
- JSON or machine-readable export alongside HTML
- Report section include/exclude toggles
- CI smoke test with a minimal fixture tarball
- Homebrew / apt packaging (community or official)

## What is not in git

- Generated HTML (`reports/`, `*_logs/`)
- Cluster dump archives and extracted trees
- Pre-built binaries ([GitHub Releases](https://github.com/yunushaikh/pt-k8s-summary/releases))
- Local `bin/` build output

## Maintainers

Release process: [RELEASE.md](RELEASE.md)
