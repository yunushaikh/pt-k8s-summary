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

## Feature: Newest lines in the pod log viewer

- **UI:** Above every pod-log table (PXC, Percona Server, PostgreSQL, MongoDB), and again in the viewer, above the log text, when the file is larger than the embed limit.
- **Status:** shipped (since v0.9.0)
- **Primary source:** the pod directory’s `*.log`, `logs.txt`, and `summary.txt`. Formatted and Full both embed the end of the file.
- **Secondary / fallback:** the complete file is copied next to the report under `<report>_logs/` and linked as **Open in a new window**.
- **Comparison:** no. The window is the newest part of the same file, not a second source.
- **If missing:** a file under the size limit is shown in full, with no newest-lines note in the viewer.
- **Caveats:** the cut starts on a newline. The first line of a very long last line can still be partial. Lines stay in file order, newest at the bottom of the window.

## Feature: Percona Server for MongoDB topology and issues

- **UI:** Percona MongoDB tab. Cluster table (state, ready, version, sharding, PITR, PMM, host). Under each cluster: a **Check these first** list, a topology table (config server, shards, mongos), and status conditions. Backups and restores are separate tables. Cluster, backup, and restore names open the CR YAML. Workload and operator pod logs sit at the top of the tab when the dump has them.
- **Status:** shipped (since v0.9.0)
- **Primary source:** `perconaservermongodbs.yaml` (`status.state`, `status.ready`/`size`, `status.message`, `status.host`, `status.replsets.*.members.stateStr`, `status.mongos`, `status.conditions`, `spec.replsets`, `spec.sharding`, `spec.backup.pitr`, `spec.unsafeFlags`). Backups: `perconaservermongodbbackups.yaml`. Restores: `perconaservermongodbrestores.yaml`.
- **Secondary / fallback:** StatefulSet ready vs spec size from `statefulsets.yaml` (`<cluster>-<replset>` and `<cluster>-mongos`). Pod container readiness from `pods.yaml` labels `app.kubernetes.io/instance`, `component`, and `replset`.
- **Comparison:** yes — CR ready/size vs spec size, CR size vs StatefulSet ready replicas, CR mongos ready vs pod container ready. Rows turn red when state is not ready, a condition type is `error` with status True (even if cluster state is ready), a replset has no members or no PRIMARY, a shard is not added, or no matching pods exist.
- **If missing:** empty section text. A replset with size greater than zero and an empty members map shows “no member roles in status”. A size-1 replset with one PRIMARY is shown as PRIMARY; SECONDARY is shown only when `stateStr` says so.
- **Caveats:** these roles come from the operator status, not a live `rs.status()`. Private keys are not inlined; certificate expiry is the Certificates section.

## Feature: Percona Server for MongoDB certificates

- **UI:** Percona MongoDB tab → **Certificates** (under backups and restores). Columns: namespace, cluster, dump file, cert, issuer, Not Before, Not After, note.
- **Status:** shipped (since v0.9.0)
- **Primary source:** OpenSSL `x509 -text` files `<namespace>/<cluster>-ca-cert`, `<cluster>-ssl`, and `<cluster>-ssl-internal`. Each `--- Decoded tls.crt ---` / `ca.crt` block is one row. `tls.key` is a note only.
- **Secondary / fallback:** cert-manager `certificates.yaml` (`status.notBefore`, `status.notAfter`, Ready condition, `spec.issuerRef.name`) when none of those OpenSSL files match a PerconaServerMongoDB cluster.
- **Comparison:** no. The two sources are not shown together. OpenSSL text wins when it is present.
- **If missing:** the Certificates block is omitted.
- **Caveats:** a file is included only when its name matches a PerconaServerMongoDB in the same namespace, so a MySQL or PXC cert in the same dump is left on that technology’s tab. Expiry is the Not After value from the dump, not a live check.

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

## Feature: Dump inventory + soft-fail + YAML browser

- **UI:** Horizontal buttons under the report title, left aligned: **Processing details** and **Dump inventory**. Each opens a side panel (closed by default). The inventory button shows a gap count when files are missing. **Browse dump YAML** stays inside the inventory panel (modal: folder tree, file pane, in-file search with highlight).
- **Status:** shipped (since v0.8.5). Side menu since v0.8.6.
- **Primary source:** walk of the dump root for expected collector exports (`nodes.yaml`, `events.yaml`, `pods.yaml`, `statefulsets.yaml`, `errors.txt`, PXC/PS/PG list YAML via `dumpfiles.FindListYAMLFiles`).
- **Soft-fail:** missing or unreadable `nodes.yaml` (and operator list load/parse errors) no longer abort HTML generation; the report is written with empty affected sections.
- **Generation warnings:** every inventory row with status `missing` or `unreadable` prints to stderr as `warning: dump file missing: …` or `warning: dump file unreadable: …` while generating the report.
- **YAML browser embed caps:** per-file 256 KiB, up to 80 files, ~6 MiB total; oversize files appear in the tree as omitted with a note.
- **If missing:** inventory shows `missing` / `unreadable`; browser button disabled when nothing was embedded.
- **Caveats:** Operator CRDs absent from a single-product dump (e.g. no PS YAML on a PXC-only dump) are listed as missing and warned — expected for incomplete multi-operator coverage, not necessarily a collector failure.

---

## Feature: PXC previous status transitions

- **UI:** PXC tab → under each cluster row → **Previous transitions** (time, type, status, reason, message), oldest first. `error` and `ErrorReconcile` are highlighted. Every unique condition is listed (no row cap). Six or fewer rows start expanded; more than six use the same collapsed summary as backups (**expand to view**, with the error count in the summary).
- **Status:** shipped (since v0.8.4)
- **Primary source:** PerconaXtraDBCluster `status.conditions` (`lastTransitionTime`, `type`, `status`, `reason`, `message`) from the cluster list YAML.
- **Secondary / fallback:** none. The full document is still the CR YAML modal.
- **Comparison:** no
- **Duplication:** exact copies (same time, type, status, reason, and message) are dropped. The current `ready` condition is omitted here because the Ready column already shows its status and time.
- **If missing:** subsection omitted when no conditions remain after those drops.
- **Caveats:** This is the operator’s stored condition list, not a reconstructed event history. Older `error` entries can remain while Ready is still True.

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

## Feature: Certified images vs `pods.yaml` (PXC / PS)

- **UI:** PXC or Percona Server tab → under each cluster → **Container images vs certified images** table (**Matches certified list**: yes / no / not checked).
- **Status:** shipped (improved in v0.8.3 for unpublished lists)
- **Primary source (cluster images):** `pods.yaml` container images for the cluster (labels `app.kubernetes.io/instance` + component).
- **Secondary source (certified list):** HTTP fetch of that operator’s release notes for `spec.crVersion`:
  - PXC: `…/Kubernetes-Operator-for-PXC-RN{crVersion}.html#percona-certified-images`
  - PS: `…/Kubernetes-Operator-for-PS-RN{crVersion}.html#percona-certified-images`
  - Section must be a real heading/`id="percona-certified-images"` (sidebar nav link alone does **not** count).
- **When a list exists (typical):** PXC **1.17.0+**, PS **0.10.0+** — compare normalized `percona/…:tag` refs → **yes** / **no**.
- **When no list is published:** PXC **≤ 1.16.x**, PS **≤ 0.9.x** (page may 200 OK but has no section) → muted “does not publish a certified images list…” note; all rows **not checked**.
- **Other “not checked” cases:** `-certified-images=false`; missing `spec.crVersion`; HTTP failure (e.g. future version RN **404**); section present but no `percona/…` refs parsed (flagged as a docs/layout error, not as unpublished).
- **Comparison:** yes when a list was loaded; otherwise no comparison is invented.
- **Caveats:** Needs network unless disabled. Registry prefixes like `docker.io/` are normalized away. Tag must match the docs table exactly.

---

## Existing notes (fill in over time)

Older features may not yet have full entries. Prefer adding an entry when you next touch that code path rather than backfilling everything at once.
