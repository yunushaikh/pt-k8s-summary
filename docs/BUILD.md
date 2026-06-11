# Build from source

Use this when you prefer not to download a [release binary](INSTALL.md), or you are developing the tool.

## Requirements

- Go 1.21 or later
- Git

## Clone and build

The Go module lives in `pt-k8s-summary/`. The repo root includes `go.work` so you can build without `cd` into the module:

```bash
git clone https://github.com/yunushaikh/pt-k8s-summary.git
cd pt-k8s-summary
mkdir -p bin
go build -o bin/pt-k8s-summary ./pt-k8s-summary
./bin/pt-k8s-summary -version
```

## Build script

```bash
./pt-k8s-summary/build.sh
./bin/pt-k8s-summary -version
```

The script backs up any previous `bin/pt-k8s-summary` and injects the git commit into the binary.

## Alternative: build inside the module

```bash
cd pt-k8s-summary
go build -o ../bin/pt-k8s-summary .
```

## Cross-compile (local smoke test)

```bash
cd pt-k8s-summary
GOOS=darwin GOARCH=arm64 go build -o /tmp/pt-k8s-summary-darwin-arm64 .
```

Official multi-arch release binaries are built by CI on each tag — see [RELEASE.md](RELEASE.md).

## See also

- [USAGE.md](USAGE.md) — how to run the binary
- [DEVELOPMENT.md](DEVELOPMENT.md) — tests and project layout
