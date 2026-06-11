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

Windows is intentionally not built.

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
