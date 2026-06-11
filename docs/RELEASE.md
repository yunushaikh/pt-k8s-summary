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
3. Commit: `Release vX.Y.Z`
4. Tag and push:

   ```bash
   git tag vX.Y.Z
   git push origin main
   git push origin vX.Y.Z
   ```

5. GitHub Actions (`.github/workflows/release.yml`) builds `pt-k8s-summary` for `linux/amd64`, creates a GitHub Release, attaches the binary, and uses the matching `CHANGELOG` section as release notes.

## Verify locally before tagging

```bash
./pt-k8s-summary/build.sh
./bin/pt-k8s-summary -version
```

## Download releases

Published binaries and release notes: [GitHub Releases](https://github.com/yunushaikh/pt-k8s-summary/releases).
