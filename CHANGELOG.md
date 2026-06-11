# Changelog

All notable changes to this project are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- User documentation moved under `docs/` (install, usage, build, development); root README is a short entry point

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
