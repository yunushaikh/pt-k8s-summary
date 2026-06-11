# Changelog

All notable changes to this project are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
