# Release process (maintainers)

## Versioning

[Semantic Versioning](https://semver.org/): PATCH = fixes, MINOR = features, MAJOR = breaking changes.

Version lives in [`pt-k8s-summary/internal/version/version.go`](../pt-k8s-summary/internal/version/version.go).

## Day to day

1. Land changes on `main`.
2. Add entries under `## [Unreleased]` in [`CHANGELOG.md`](../CHANGELOG.md).
3. Do **not** bump version or tag until ready to ship.

## Cutting a release

1. Move `[Unreleased]` items to `## [X.Y.Z] - YYYY-MM-DD` in `CHANGELOG.md`.
   - Include **user-facing** notes: what changed, how to use new behavior, any flag or workflow gotchas.
2. Set `Version = "X.Y.Z"` in `version.go`.
3. Commit: `Release vX.Y.Z` (source + CHANGELOG only — **no binaries in git**).
4. Tag and push:

   ```bash
   git tag vX.Y.Z
   git push origin main
   git push origin vX.Y.Z
   ```

5. GitHub Actions builds four binaries and publishes a [GitHub Release](https://github.com/yunushaikh/pt-k8s-summary/releases):
   - `pt-k8s-summary_linux_amd64`
   - `pt-k8s-summary_linux_arm64`
   - `pt-k8s-summary_darwin_amd64` (Intel Mac)
   - `pt-k8s-summary_darwin_arm64` (Apple Silicon)
   - `SHA256SUMS`

   If the workflow fails or stays on **Waiting for a runner** for many minutes, see [Failed or stuck release workflow](#failed-or-stuck-release-workflow) below.

Windows is intentionally not built.

## Failed or stuck release workflow

The release workflow (`.github/workflows/release.yml`) is triggered by pushing a `v*` tag. It can also be started manually from **Actions → Release → Run workflow** (enter the tag, e.g. `v0.8.1`).

### Common cause: GitHub Actions queue delays

If the job log shows **Waiting for a runner** and never reaches **Set up job** / checkout, the workflow YAML is fine — GitHub did not assign an `ubuntu-latest` runner in time. Check [GitHub Status](https://www.githubstatus.com/) for incidents such as “Delays starting Actions runs”. The v0.8.1 tag push (2026-07-09) failed this way: the job was **cancelled after ~15 minutes** with **0 billable runner time** (no build steps ran).

When a tag push run fails for this reason:

1. Wait until [GitHub Status](https://www.githubstatus.com/) shows Actions runners healthy, or try again later.
2. **Re-run** the failed job from the Actions run page, **or**
3. Use **Actions → Release → Run workflow** and enter the tag (e.g. `v0.8.1`), **or**
4. Re-push the tag:
   ```bash
   git push origin :refs/tags/vX.Y.Z
   git push origin vX.Y.Z
   ```

A successful run usually finishes in **1–2 minutes** once a runner is assigned (v0.8.0 took ~67 seconds).

### Build or CHANGELOG failures

If the runner starts but the job fails during build or release creation, read the step log: missing `CHANGELOG.md` section for the version, Go build error, or `softprops/action-gh-release` permission issues.

## CHANGELOG tips for each release

Good release notes answer:

- **What** changed (features / fixes)
- **Who cares** (backup filter, Mac download, etc.)
- **How to use** new flags or report UI (one line if non-obvious)
- **Breaking** behavior, if any

Users read `CHANGELOG.md` on git and the same section on the GitHub Release page.

## Verify locally before tagging

```bash
./pt-k8s-summary/build.sh
./bin/pt-k8s-summary -version
go test ./pt-k8s-summary/...
```

Optional: install repo git hooks so Cursor never adds `Co-authored-by: Cursor` to commits (see [CONTRIBUTING.md](CONTRIBUTING.md)).

Optional cross-build smoke test:

```bash
cd pt-k8s-summary
GOOS=darwin GOARCH=arm64 go build -o /tmp/pt-k8s-summary-darwin-arm64 .
```

## Design choices (for future growth)

- **Binaries only on GitHub Releases** — keeps the git repo small; old versions stay on the Releases page.
- **No Windows** — primary users are Linux/macOS k8s operators; add later if demand appears.
- **Pure Go / CGO_ENABLED=0** — cross-compiles reliably in CI without macOS runners.
- **Optional runtime tools** — `pt-galera-log-explainer` (Percona Toolkit) is not bundled; see [USAGE.md](USAGE.md)
- **HTML + sidecar `_logs/`** — large logs stay beside the report; see [USAGE.md](USAGE.md)
