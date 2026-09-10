# Report features — data sources

How each user-visible part of the HTML report gets its data from a `pt-k8s-debug-collector` dump.
Update this file whenever a feature is added or its sources change (see project rule **feature documentation**).

---

## Template (copy for new features)

```markdown
## Feature: <name>

- **UI:** <tab / section / column>
- **Status:** planned | shipped (since vX.Y.Z)
- **Primary source:** …
- **Secondary / fallback:** …
- **Comparison:** yes/no — why
- **If missing:** what the cell/section shows
- **Caveats:** …
```

---

## Feature: PostgreSQL instance role (Leader / Secondary)

- **UI:** Percona PostgreSQL tab → **PostgreSQL workload pods** table — columns **Kind**, **Role (pod)**, **Role (Patroni)**.
- **Status:** shipped (since v0.8.3)
- **Primary source (pod):** `pods.yaml` label `postgres-operator.crunchydata.com/role`
  - `primary` → Leader
  - `replica` → Secondary
  - `pgbouncer` → Kind = pgBouncer; role columns show `—`
- **Secondary source (Patroni logs):** instance pod dump logs (`<ns>/<pod>/logs.txt`, also `summary.txt` / `log`; last 512 KiB). Latest matching Patroni line for this pod name:
  - `I am (<pod>), the leader with the lock` → Leader
  - `I am (<pod>), a secondary, and following a leader (<leader-pod>)` → Secondary (+ following name)
  - `I am (<pod>), a primary…` → Leader
- **Comparison:** **yes** — both columns always shown for PostgreSQL instances. Row highlighted when both are set and disagree.
- **If missing:** `—` for that source; no invented role.
- **Caveats:** Dump-time only. Labels can lag after failover; logs can be truncated or mid-window.

---

## Feature: PostgreSQL Patroni CR configuration (`spec.patroni`)

- **UI:** Percona PostgreSQL tab → under each cluster row → **Patroni (`spec.patroni`)** subsection.
- **Status:** shipped (since v0.8.3)
- **Primary source:** PerconaPGCluster list YAML (`perconapgclusters*.yaml`) → `spec.patroni`:
  - `syncPeriodSeconds`, `leaderLeaseDurationSeconds`, `port`
  - `dynamicConfiguration.postgresql.parameters` — short snippet in the cell; **View / Show full parameters** opens a modal with all `key = value` lines (same pattern as PXC MySQL configuration).
- **Secondary / fallback:** none — values come only from the CR in the dump.
- **Comparison:** no
- **If missing:** subsection omitted when `spec.patroni` is absent; parameters cell shows `—` when timing is set but no parameters map.
- **Caveats:** Ad-hoc/operator defaults only as written in the CR; runtime Patroni DCS may differ if changed outside the CR. Snippet shows up to 3 keys (then “+N more”).

---

## Feature: PostgreSQL backup / restore inventory (collapsible)

- **UI:** Percona PostgreSQL tab → **Backup inventory** / **Restore inventory** (`<details>`, collapsed by default) with client-side filter — same pattern as PXC backups.
- **Status:** shipped (since v0.8.3)
- **Primary source:** `PerconaPGBackup` / `PerconaPGRestore` list YAML under the dump.
- **Filter:** name, cluster, repo, destination (backups), status, type (backups), age — not YAML body text.
- **If missing:** empty-state meta line (no collapsible).
- **Caveats:** Clicking a name still opens the resource YAML modal.

---

## Feature: Kubernetes events free-text filter

- **UI:** Kubernetes tab → **Kubernetes events** (section `dump-events`) filter box.
- **Status:** shipped (since v0.8.3)
- **Primary source:** namespace `events.yaml` (core `v1` and `events.k8s.io/v1`), merged newest-first.
- **Filter behavior:** client-side; matches **any column text** on the row (type, namespace, object, reason, message, count, timestamps). Space-separated tokens are **AND**ed (every token must appear somewhere in the row). Case-insensitive.
- **Discoverability:** Percona PostgreSQL cluster section notes that events live under the Kubernetes tab and can be filtered by cluster/pod name.
- **If missing:** empty events section / no filter when no events.yaml.
- **Caveats:** Filter is substring match only (not field-scoped operators).

---

## Feature: PXC CR size ↔ StatefulSet replicas cross-check

- **UI:** PXC tab → HAProxy / ProxySQL / PXC subsections — columns **Size (CR)**, **STS replicas**, **STS ready**, **CR ↔ STS**.
- **Status:** shipped (since v0.8.3)
- **Primary source (CR size):** PerconaXtraDBCluster → `spec.haproxy.size` / `spec.proxysql.size` / `spec.pxc.size`.
- **Secondary source (STS):** `statefulsets.yaml` entry named `<cluster>-haproxy` / `<cluster>-proxysql` / `<cluster>-pxc` in the same namespace (`spec.replicas`, `status.readyReplicas`).
- **Comparison:** **yes** — mismatch when CR size ≠ STS `spec.replicas`; row highlighted. Note `STS not in dump` when the StatefulSet is missing from the dump.
- **If missing:** STS columns show `—` with note `STS not in dump`.
- **Caveats:** Only compares desired replica counts, not pod readiness vs CR status fields.

---

## Existing notes (fill in over time)

Older features may not yet have full entries. Prefer adding an entry when you next touch that code path rather than backfilling everything at once.
