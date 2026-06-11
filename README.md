# pt-k8s-summary

Generate a single **HTML report** from a Kubernetes cluster dump collected with [pt-k8s-debug-collector](https://github.com/percona/k8spxc-debug-collector).

The report summarizes nodes, Percona XtraDB Cluster (PXC) resources, backups, pod logs, events, certificates, and related configuration.

## Documentation

**Start here:** [docs/README.md](docs/README.md)

| Guide | Description |
|-------|-------------|
| [docs/INSTALL.md](docs/INSTALL.md) | Download and install (Linux / macOS, all architectures) |
| [docs/USAGE.md](docs/USAGE.md) | Run the tool, flags, reading and sharing reports |
| [docs/BUILD.md](docs/BUILD.md) | Build from source |
| [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) | Contributors: layout, tests, roadmap |

## Releases

- [GitHub Releases](https://github.com/yunushaikh/pt-k8s-summary/releases) — pre-built binaries and `SHA256SUMS`
- [CHANGELOG.md](CHANGELOG.md) — version history and release notes

Check your installed version: `pt-k8s-summary -version`

## Quick example

```bash
pt-k8s-summary /path/to/cluster-dump.tar.gz
# → reports/<archive-stem>-summary.html
```

See [docs/USAGE.md](docs/USAGE.md) for directory mode, flags, and report tips.

## Maintainers

[docs/RELEASE.md](docs/RELEASE.md) — versioning and tagging.
