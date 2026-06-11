# Install

Pre-built binaries are published on **[GitHub Releases](https://github.com/yunushaikh/pt-k8s-summary/releases)** only (not stored in git).

Pick the asset for your machine, download it, install as `pt-k8s-summary` on your `PATH`, then run it from anywhere.

## Choose your binary

| Platform | GitHub asset |
|----------|----------------|
| Linux x86_64 | `pt-k8s-summary_linux_amd64` |
| Linux ARM64 | `pt-k8s-summary_linux_arm64` |
| macOS Apple Silicon (M1/M2/M3) | `pt-k8s-summary_darwin_arm64` |
| macOS Intel | `pt-k8s-summary_darwin_amd64` |

Windows is not supported.

Set the release version (use [latest](https://github.com/yunushaikh/pt-k8s-summary/releases/latest) or a specific tag):

```bash
VERSION=v0.4.0
```

## Linux x86_64

```bash
VERSION=v0.4.0
ASSET=pt-k8s-summary_linux_amd64
curl -fsSL -o "$ASSET" \
  "https://github.com/yunushaikh/pt-k8s-summary/releases/download/${VERSION}/${ASSET}"
chmod +x "$ASSET"
sudo install -m 0755 "$ASSET" /usr/local/bin/pt-k8s-summary
pt-k8s-summary -version
```

## Linux ARM64

```bash
VERSION=v0.4.0
ASSET=pt-k8s-summary_linux_arm64
curl -fsSL -o "$ASSET" \
  "https://github.com/yunushaikh/pt-k8s-summary/releases/download/${VERSION}/${ASSET}"
chmod +x "$ASSET"
sudo install -m 0755 "$ASSET" /usr/local/bin/pt-k8s-summary
pt-k8s-summary -version
```

## macOS Apple Silicon

```bash
VERSION=v0.4.0
ASSET=pt-k8s-summary_darwin_arm64
curl -fsSL -o "$ASSET" \
  "https://github.com/yunushaikh/pt-k8s-summary/releases/download/${VERSION}/${ASSET}"
chmod +x "$ASSET"
sudo install -m 0755 "$ASSET" /usr/local/bin/pt-k8s-summary
pt-k8s-summary -version
```

## macOS Intel

```bash
VERSION=v0.4.0
ASSET=pt-k8s-summary_darwin_amd64
curl -fsSL -o "$ASSET" \
  "https://github.com/yunushaikh/pt-k8s-summary/releases/download/${VERSION}/${ASSET}"
chmod +x "$ASSET"
sudo install -m 0755 "$ASSET" /usr/local/bin/pt-k8s-summary
pt-k8s-summary -version
```

`install` copies the downloaded file and renames it to `pt-k8s-summary` in one step. You can remove the download artifact afterward: `rm "$ASSET"`.

## Install without `sudo`

```bash
mkdir -p ~/.local/bin
install -m 0755 "$ASSET" ~/.local/bin/pt-k8s-summary
```

Ensure `~/.local/bin` is on your `PATH`. Then run `pt-k8s-summary` from any directory.

## Verify checksums

Download `SHA256SUMS` from the same release page, then:

```bash
sha256sum -c SHA256SUMS --ignore-missing
```

## macOS first run

Gatekeeper may block a freshly downloaded binary. After install, run `pt-k8s-summary` once from Terminal, or right-click → **Open**. You do not need Go installed to use a release binary.

## Next steps

- [USAGE.md](USAGE.md) — run the tool and read the report
- [BUILD.md](BUILD.md) — build from source instead of a release binary
