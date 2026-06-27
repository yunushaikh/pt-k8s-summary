# pt-k8s-summary

Generate a single **HTML report** from a Kubernetes cluster dump collected with [pt-k8s-debug-collector](https://github.com/percona/k8spxc-debug-collector).

The report summarizes nodes, Percona XtraDB Cluster (PXC) resources, Percona Server for MySQL, backups, pod logs, events, certificates, and related configuration.

> **Testing in progress — v0.7.0-beta.1:** The new **grouped tab layout** (`-layout grouped`) is available in the latest beta release but still under evaluation. Default output is unchanged (`classic`). Try both layouts on your dumps and report issues before we promote grouped to default. See [docs/USAGE.md](docs/USAGE.md#grouped-layout-beta).

## Documentation

**Start here:** [docs/README.md](docs/README.md)

| Guide | Description |
|-------|-------------|
| [docs/INSTALL.md](docs/INSTALL.md) | Download and install (Linux / macOS, all architectures) |
| [docs/USAGE.md](docs/USAGE.md) | Run the tool, flags, reading and sharing reports |
| [docs/BUILD.md](docs/BUILD.md) | Build from source |
| [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) | Contributors: layout, tests, roadmap |

## Releases

- **Latest beta:** [v0.7.0-beta.1](https://github.com/yunushaikh/pt-k8s-summary/releases/tag/v0.7.0-beta.1) — grouped layout beta (`-layout grouped`); testing in progress
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
