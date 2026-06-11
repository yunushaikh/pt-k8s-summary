# Release process

## Versioning

This project uses [Semantic Versioning](https://semver.org/):

| Change | Bump | Example |
|--------|------|---------|
| Bug fix, docs, small improvement | PATCH | `0.2.0` → `0.2.1` |
| New flag, report section, notable feature | MINOR | `0.2.1` → `0.3.0` |
| Breaking CLI or report contract | MAJOR | `0.x` → `1.0.0` |

The release version lives in [`pt-k8s-summary/internal/version/version.go`](../pt-k8s-summary/internal/version/version.go). Bump it only when cutting a release.

## Day to day

1. Implement the fix or feature on `main`.
2. Add a one-line entry under `## [Unreleased]` in [`CHANGELOG.md`](../CHANGELOG.md) (Added / Changed / Fixed / Removed).
3. Do **not** bump `version.go` or create tags yet.

## Cutting a release

When ready to ship version `X.Y.Z`:

1. Move items from `[Unreleased]` to `## [X.Y.Z] - YYYY-MM-DD` in `CHANGELOG.md`.
2. Set `Version = "X.Y.Z"` in `pt-k8s-summary/internal/version/version.go`.
3. Build the release binary and stage it under `releases/bin/`:

   ```bash
   ./pt-k8s-summary/build.sh
   cp bin/pt-k8s-summary releases/bin/pt-k8s-summary-linux-amd64
   mkdir -p releases/bin/vX.Y.Z
   cp bin/pt-k8s-summary releases/bin/vX.Y.Z/pt-k8s-summary-linux-amd64
   chmod +x releases/bin/pt-k8s-summary-linux-amd64 releases/bin/vX.Y.Z/pt-k8s-summary-linux-amd64
   ```

4. Commit: `Release vX.Y.Z` (include `CHANGELOG.md`, `version.go`, and `releases/bin/` changes)
5. Tag and push:

   ```bash
   git tag vX.Y.Z
   git push origin main
   git push origin vX.Y.Z
   ```

6. GitHub Actions (`.github/workflows/release.yml`) builds `linux/amd64`, attaches the binary to a **GitHub Release**, and uses the matching `CHANGELOG` section as release notes. Binaries under `releases/bin/` in git come from step 4 (the release commit), not from CI.

## Verify locally before tagging

```bash
./pt-k8s-summary/build.sh
./bin/pt-k8s-summary -version
```

## Download releases

Published binaries and release notes: [GitHub Releases](https://github.com/yunushaikh/pt-k8s-summary/releases).
