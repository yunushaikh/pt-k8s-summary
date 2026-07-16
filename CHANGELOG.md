# Changelog

All notable changes to this project are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.8.2] - 2026-07-16

### Fixed

- **New collector dump layout:** PXC cluster and backup discovery now uses `dumpfiles.FindListYAMLFiles` (legacy long basenames and short names such as `perconaxtradbclusters.yaml`), matching PG/PS. Everest/OLM dumps under namespace folders are detected again.
- **Events (`events.k8s.io/v1`):** parse `note` / `regarding` / `deprecatedCount` / `deprecated*Timestamp` while keeping core `v1` field names.
- **PG operator (OLM/Everest):** detect Deployments by name, image, or `operators.coreos.com/percona-postgresql-operator*` labels in addition to Helm control-plane labels.
- **PG certificates:** recognize `*-cluster-ca-cert` dump files alongside existing cert suffixes; parse `--- Decoded <file> ---` OpenSSL headers used by newer collectors (also unblocks PXC cert tables on those dumps).

## [0.8.1] - 2026-07-09

### Fixed

- **Grouped layout duplication:** PXC, Percona Server, and PostgreSQL operator sections no longer render twice. Classic section templates were split into `*_classic.tmpl` (linear layout) and `*_grouped.tmpl` (tab defines only); the grouped report chain no longer appends classic operator bodies after the tab panels.

## [0.8.0] - 2026-07-09

### Added

- **Percona PostgreSQL operator** support with a dedicated **Percona PostgreSQL** tab in grouped layout:
  - Cluster inventory (`kubectl get pg` style): endpoint, status, Postgres/pgBouncer counts, CR version, PG version, PMM, age — CR YAML linked by name
  - Workload pods table (PostgreSQL instances + pgBouncer) from `pods.yaml`
  - Workload and operator pod logs (same viewer as PXC/PS)
  - Operator deployment summary: version, `PGO_WORKERS` concurrency, created age, PMM across clusters
  - Backups, restores, and upgrades tables (`PerconaPGBackup`, `PerconaPGRestore`, `PerconaPGUpgrade`) with kind-based YAML discovery
  - TLS certificates from `*-cluster-cert` and `pgo-root-cacert` dump files
- All PG list YAMLs discovered dynamically via `internal/dumpfiles` (no hardcoded filenames)

## [0.7.0] - 2026-07-07

### Added

- **Grouped report layout (default):** reports open with technology tabs — **Kubernetes** | **Percona XtraDB Cluster** | **Percona Server for MySQL**. Use `-layout classic` for the previous linear layout. Empty operator tabs are hidden on single-technology dumps.
- **Section grouping:** collector sections tagged by technology (`common`, `pxc`, `ps`); PS extra sections (topology, status, backup schedules, storages, upgrade, sidecar/toolkit, PVC sizing).
- **Per-technology certificates:** separate PXC and Percona Server certificate sections under the matching tab (`pxc_certificates_section.go`, `ps_certificates_section.go`).
- **Dynamic dump file discovery:** `nodes.yaml`, `errors.txt`, and `events.yaml` are found anywhere under the dump tree (legacy dump-root paths and newer `cluster-scope/` layouts). Node and Event files are validated by Kubernetes kind, so similarly named files (e.g. `csinodes.yaml`) are ignored.
- **Dynamic page title** reflects which operators are present in the dump.

### Changed

- **Default layout** is now `grouped` (was `classic` in v0.6.x).
- Operator list YAML discovery (`internal/dumpfiles`) already walks the full dump tree; nodes/events/errors now follow the same pattern.

## [0.6.0] - 2026-06-23

### Added

- **Percona Server for MySQL operator** support in cluster dump reports: CR inventory (MySQL, HAProxy, Router, Orchestrator), backups, pod logs, Helm/PMM, PITR, updateStrategy, unsafeFlags/pause, expose, and TLS certificates
- Flexible list-YAML discovery (`internal/dumpfiles`) with preferred collector filenames plus kind-based fallback when `pt-k8s-debug-collector` naming changes

### Fixed

- **Percona Server pod logs section** no longer dumps thousands of lines of hidden log embeds onto the page; PS sections now use scoped CSS/JS ids (`#ps-pod-logs`, `ps-plg-stash-*`, `ps-log-modal-ps`) matching the PXC pod-logs layout

## [0.5.0] - 2026-06-18

### Added

- **Clickable report path on success:** after generating a report, the tool prints the **absolute path** to the HTML file; in a terminal, the path is a clickable `file://` link that opens the report in your browser

### Fixed

- **Certificates section** now reads all PXC TLS dump files present in the cluster dump (`<cluster>-ca-cert`, `<cluster>-ssl`, `<cluster>-ssl-internal`), not only `-ssl-internal`

## [0.4.0] - 2026-06-11

### Added

- **Multi-arch GitHub Releases:** `linux/amd64`, `linux/arm64`, `darwin/amd64` (Intel Mac), `darwin/arm64` (Apple Silicon) — Windows not built
- `SHA256SUMS` on each release for binary verification
- Expanded [README.md](README.md): per-platform download, macOS Gatekeeper note, flag ordering, report section guide, roadmap

### Changed

- **Binaries removed from git** — download only from [GitHub Releases](https://github.com/yunushaikh/pt-k8s-summary/releases); `releases/README.md` points there
- Maintainer release process no longer commits binaries to `main` ([docs/RELEASE.md](docs/RELEASE.md))

## [0.3.2] - 2026-06-11

### Fixed

- Backup **View logs** links visible again: log/manifest text moved out of table rows (stash div); **Logs** column placed after **Name** so it is not off-screen when scrolling wide tables

## [0.3.1] - 2026-06-11

### Fixed

- Backup table filter no longer matches hidden manifest or pod log text; searching `Failed` lists backups by status and visible columns only

## [0.3.0] - 2026-06-11

### Added

- Collapsible, filterable **backup inventory** section (large backup lists collapsed by default; search any column)
- **Filter** on the node inventory table (match hostname, role, IP, OS, kubelet, capacity, pressure, etc.)
- Release binaries committed under `releases/bin/` for direct download from the repository

### Changed

- Release CI also copies the built binary into `releases/bin/` on the default branch

## [0.2.0] - 2026-06-10

### Added

- `-version` flag and tool version in HTML report footer
- `CHANGELOG.md`, `docs/RELEASE.md`, and SemVer release workflow
- GitHub Actions workflow to build `linux/amd64` binary on `v*` tags
- Cursor rules reminding agents to update release notes with user-visible changes

### Changed

- `build.sh` injects git commit into the binary via `-ldflags`

## [0.1.0] - 2026-06-10

### Added

- HTML cluster dump report (nodes, PXC, backups, pod logs, events, config sections)
- Archive (`.tar.gz` / `.tgz`) and extracted directory input modes
- Root `go.work` for building from repository root without `cd`
- Large pod log sidecar files (`{report}_logs/`) with modal links to full content
- `README.md` with build and usage instructions
