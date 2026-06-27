package collector

import (
	"fmt"
	"html"
	"html/template"
	"sort"
	"strings"

	"pt-k8s-summary/internal/dumpctx"
	"pt-k8s-summary/internal/k8sfmt"
)

// --- Backup schedules ---

type psBackupScheduleSection struct{}

func (psBackupScheduleSection) ID() string    { return "ps-backup-schedules" }
func (psBackupScheduleSection) Title() string { return "Percona Server · backup schedules" }
func (psBackupScheduleSection) Group() SectionGroup { return GroupPS }

func (psBackupScheduleSection) Collect(ctx dumpctx.Context) (Section, error) {
	clusters, err := loadPSClusters(ctx.Root())
	if err != nil {
		return Section{}, err
	}
	rows := gatherPSBackupScheduleRows(clusters)
	if len(rows) == 0 {
		return Section{}, nil
	}
	return Section{HTML: template.HTML(renderPSBackupSchedules(rows))}, nil
}

type psScheduleRow struct {
	Cluster, Namespace, Name, Cron, StorageName, Type string
	Keep                                              string
}

func gatherPSBackupScheduleRows(clusters []psClusterRecord) []psScheduleRow {
	var rows []psScheduleRow
	for _, c := range clusters {
		if c.Spec.Backup == nil || len(c.Spec.Backup.Schedule) == 0 {
			continue
		}
		for _, sch := range c.Spec.Backup.Schedule {
			name := strings.TrimSpace(sch.Name)
			if name == "" {
				continue
			}
			typ := strings.TrimSpace(sch.Type)
			if typ == "" {
				typ = "full"
			}
			rows = append(rows, psScheduleRow{
				Cluster: c.Name, Namespace: c.Namespace, Name: name,
				Cron: strings.TrimSpace(sch.Schedule), Keep: fmt.Sprintf("%d", sch.Keep),
				StorageName: strings.TrimSpace(sch.StorageName), Type: typ,
			})
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Namespace != rows[j].Namespace {
			return rows[i].Namespace < rows[j].Namespace
		}
		if rows[i].Cluster != rows[j].Cluster {
			return rows[i].Cluster < rows[j].Cluster
		}
		return rows[i].Name < rows[j].Name
	})
	return rows
}

func renderPSBackupSchedules(rows []psScheduleRow) string {
	esc := html.EscapeString
	var tb strings.Builder
	tb.WriteString(psTableOpen("pxc-cert-table"))
	tb.WriteString(`<tr><th>Cluster</th><th>Namespace</th><th>Schedule name</th><th>Cron</th><th>keep</th><th>storageName</th><th>type</th></tr></thead><tbody class="ps-sched-tbody">`)
	for _, r := range rows {
		tb.WriteString(`<tr><td class="pxc-cert-mono">` + esc(r.Cluster) + `</td><td>` + esc(r.Namespace))
		tb.WriteString(`</td><td><code>` + esc(r.Name) + `</code></td><td><code>` + esc(r.Cron))
		tb.WriteString(`</code></td><td>` + esc(r.Keep) + `</td><td><code>` + esc(r.StorageName))
		tb.WriteString(`</code></td><td>` + esc(r.Type) + `</td></tr>`)
	}
	tb.WriteString(`</tbody></table>`)
	meta := fmt.Sprintf("%d schedule(s) · collapsed by default · filterable", len(rows))
	inner := psCollapsibleTable("ps-backup-schedules-coll", "Backup cron schedules", meta, tb.String(), "ps-sched-tbody", "cluster, cron, storage…")
	return `<p class="meta">From <code>spec.backup.schedule</code> on each <code>PerconaServerMySQL</code> CR.</p>` + inner
}

// --- Storage backends ---

type psStorageSection struct{}

func (psStorageSection) ID() string    { return "ps-backup-storages" }
func (psStorageSection) Title() string { return "Percona Server · backup storages" }
func (psStorageSection) Group() SectionGroup { return GroupPS }

func (psStorageSection) Collect(ctx dumpctx.Context) (Section, error) {
	clusters, err := loadPSClusters(ctx.Root())
	if err != nil {
		return Section{}, err
	}
	rows := gatherPSStorageRows(clusters)
	if len(rows) == 0 {
		return Section{}, nil
	}
	return Section{HTML: template.HTML(renderPSStorageBackends(rows))}, nil
}

type psStorageRow struct {
	Cluster, Namespace, StorageName, Type, Target, Region, CredentialsSecret, VerifyTLS string
}

func gatherPSStorageRows(clusters []psClusterRecord) []psStorageRow {
	var rows []psStorageRow
	for _, c := range clusters {
		if c.Spec.Backup == nil || len(c.Spec.Backup.Storages) == 0 {
			continue
		}
		names := make([]string, 0, len(c.Spec.Backup.Storages))
		for k := range c.Spec.Backup.Storages {
			names = append(names, k)
		}
		sort.Strings(names)
		for _, sn := range names {
			st := c.Spec.Backup.Storages[sn]
			if st == nil {
				continue
			}
			row := psStorageRow{
				Cluster: c.Name, Namespace: c.Namespace, StorageName: sn,
				Type: strings.TrimSpace(st.Type),
			}
			if st.VerifyTLS != nil {
				if *st.VerifyTLS {
					row.VerifyTLS = "true"
				} else {
					row.VerifyTLS = "false"
				}
			}
			switch row.Type {
			case "s3":
				if st.S3 != nil {
					row.Target = strings.TrimSpace(st.S3.Bucket)
					if p := strings.TrimSpace(st.S3.Prefix); p != "" {
						row.Target += "/" + p
					}
					row.Region = strings.TrimSpace(st.S3.Region)
					row.CredentialsSecret = strings.TrimSpace(st.S3.CredentialsSecret)
				}
			case "gcs":
				if st.GCS != nil {
					row.Target = strings.TrimSpace(st.GCS.Bucket)
					if p := strings.TrimSpace(st.GCS.Prefix); p != "" {
						row.Target += "/" + p
					}
					row.CredentialsSecret = strings.TrimSpace(st.GCS.CredentialsSecret)
				}
			case "azure":
				if st.Azure != nil {
					row.Target = strings.TrimSpace(st.Azure.ContainerName)
					if p := strings.TrimSpace(st.Azure.Prefix); p != "" {
						row.Target += "/" + p
					}
					row.CredentialsSecret = strings.TrimSpace(st.Azure.CredentialsSecret)
				}
			}
			rows = append(rows, row)
		}
	}
	return rows
}

func renderPSStorageBackends(rows []psStorageRow) string {
	esc := html.EscapeString
	dash := "—"
	var tb strings.Builder
	tb.WriteString(psTableOpen("pxc-cert-table"))
	tb.WriteString(`<tr><th>Cluster</th><th>Namespace</th><th>storage</th><th>type</th><th>bucket/container</th><th>region</th><th>credentialsSecret</th><th>verifyTLS</th></tr></thead><tbody>`)
	for _, r := range rows {
		tgt, reg, cred, vt := r.Target, r.Region, r.CredentialsSecret, r.VerifyTLS
		if tgt == "" {
			tgt = dash
		}
		if reg == "" {
			reg = dash
		}
		if cred == "" {
			cred = dash
		}
		if vt == "" {
			vt = dash
		}
		tb.WriteString(`<tr><td class="pxc-cert-mono">` + esc(r.Cluster) + `</td><td>` + esc(r.Namespace))
		tb.WriteString(`</td><td><code>` + esc(r.StorageName) + `</code></td><td>` + esc(r.Type))
		tb.WriteString(`</td><td><code>` + esc(tgt) + `</code></td><td>` + esc(reg))
		tb.WriteString(`</td><td><code>` + esc(cred) + `</code></td><td>` + esc(vt) + `</td></tr>`)
	}
	tb.WriteString(`</tbody></table>`)
	return `<p class="meta">Named backup destinations from <code>spec.backup.storages</code> (secret names only; no credential values).</p>` + tb.String()
}

// --- Upgrade options ---

type psUpgradeSection struct{}

func (psUpgradeSection) ID() string    { return "ps-upgrade-options" }
func (psUpgradeSection) Title() string { return "Percona Server · upgradeOptions" }
func (psUpgradeSection) Group() SectionGroup { return GroupPS }

func (psUpgradeSection) Collect(ctx dumpctx.Context) (Section, error) {
	clusters, err := loadPSClusters(ctx.Root())
	if err != nil {
		return Section{}, err
	}
	if len(clusters) == 0 {
		return Section{}, nil
	}
	return Section{HTML: template.HTML(renderPSUpgradeOptions(clusters))}, nil
}

func renderPSUpgradeOptions(clusters []psClusterRecord) string {
	esc := html.EscapeString
	dash := "—"
	var b strings.Builder
	b.WriteString(`<table class="pxc-cert-table"><thead><tr><th>Cluster</th><th>Namespace</th><th>apply</th><th>versionServiceEndpoint</th></tr></thead><tbody>`)
	for _, c := range clusters {
		apply := strings.TrimSpace(c.Spec.UpgradeOptions.Apply)
		if apply == "" {
			apply = dash
		}
		ep := strings.TrimSpace(c.Spec.UpgradeOptions.VersionServiceEndpoint)
		if ep == "" {
			ep = dash
		}
		b.WriteString(`<tr><td class="pxc-cert-mono">` + esc(c.Name) + `</td><td>` + esc(c.Namespace))
		b.WriteString(`</td><td><code>` + esc(apply) + `</code></td><td><code>` + esc(ep) + `</code></td></tr>`)
	}
	b.WriteString(`</tbody></table>`)
	b.WriteString(`<p class="meta"><code>upgradeOptions.apply</code> controls automatic image upgrades (<code>disabled</code>, <code>never</code>, <code>recommended</code>).</p>`)
	return b.String()
}

// --- Sidecar PVCs & toolkit ---

type psSidecarToolkitSection struct{}

func (psSidecarToolkitSection) ID() string    { return "ps-sidecar-toolkit" }
func (psSidecarToolkitSection) Title() string { return "Percona Server · sidecars, sidecar PVCs & toolkit" }
func (psSidecarToolkitSection) Group() SectionGroup { return GroupPS }

func (psSidecarToolkitSection) Collect(ctx dumpctx.Context) (Section, error) {
	clusters, err := loadPSClusters(ctx.Root())
	if err != nil {
		return Section{}, err
	}
	htmlStr := renderPSSidecarToolkit(clusters)
	if htmlStr == "" {
		return Section{}, nil
	}
	return Section{HTML: template.HTML(htmlStr)}, nil
}

func renderPSSidecarToolkit(clusters []psClusterRecord) string {
	esc := html.EscapeString
	var sidecarRows []struct{ cluster, ns, name, image string }
	var pvcRows []struct{ cluster, ns, name, size, sc string }
	var toolkitRows []struct{ cluster, ns, image string }
	for _, c := range clusters {
		for _, sc := range c.Spec.MySQL.Sidecars {
			n := strings.TrimSpace(sc.Name)
			if n == "" {
				continue
			}
			sidecarRows = append(sidecarRows, struct{ cluster, ns, name, image string }{
				c.Name, c.Namespace, n, strings.TrimSpace(sc.Image),
			})
		}
		for _, pvc := range c.Spec.MySQL.SidecarPVCs {
			n := strings.TrimSpace(pvc.Name)
			if n == "" {
				continue
			}
			size := strings.TrimSpace(pvc.Spec.Resources.Requests.Storage)
			sc := "—"
			if pvc.Spec.StorageClassName != nil {
				sc = strings.TrimSpace(*pvc.Spec.StorageClassName)
				if sc == "" {
					sc = "—"
				}
			}
			pvcRows = append(pvcRows, struct{ cluster, ns, name, size, sc string }{
				c.Name, c.Namespace, n, k8sfmt.HumanQuantity(size), sc,
			})
		}
		if c.Spec.Toolkit != nil {
			img := strings.TrimSpace(c.Spec.Toolkit.Image)
			if img != "" {
				toolkitRows = append(toolkitRows, struct{ cluster, ns, image string }{c.Name, c.Namespace, img})
			}
		}
	}
	if len(sidecarRows) == 0 && len(pvcRows) == 0 && len(toolkitRows) == 0 {
		return ""
	}
	var b strings.Builder
	if len(toolkitRows) > 0 {
		b.WriteString(`<h4 class="pxc-subsection-title" style="margin-top:0">Toolkit</h4><table class="pxc-cert-table"><thead><tr><th>Cluster</th><th>Namespace</th><th>image</th></tr></thead><tbody>`)
		for _, r := range toolkitRows {
			b.WriteString(`<tr><td class="pxc-cert-mono">` + esc(r.cluster) + `</td><td>` + esc(r.ns) + `</td><td><code>` + esc(r.image) + `</code></td></tr>`)
		}
		b.WriteString(`</tbody></table>`)
	}
	if len(sidecarRows) > 0 {
		b.WriteString(`<h4 class="pxc-subsection-title">MySQL sidecars</h4><table class="pxc-cert-table"><thead><tr><th>Cluster</th><th>Namespace</th><th>name</th><th>image</th></tr></thead><tbody>`)
		for _, r := range sidecarRows {
			b.WriteString(`<tr><td class="pxc-cert-mono">` + esc(r.cluster) + `</td><td>` + esc(r.ns) + `</td><td><code>` + esc(r.name) + `</td><td><code>` + esc(r.image) + `</code></td></tr>`)
		}
		b.WriteString(`</tbody></table>`)
	}
	if len(pvcRows) > 0 {
		b.WriteString(`<h4 class="pxc-subsection-title">Sidecar PVCs</h4><table class="pxc-cert-table"><thead><tr><th>Cluster</th><th>Namespace</th><th>PVC name</th><th>size</th><th>storageClass</th></tr></thead><tbody>`)
		for _, r := range pvcRows {
			b.WriteString(`<tr><td class="pxc-cert-mono">` + esc(r.cluster) + `</td><td>` + esc(r.ns) + `</td><td><code>` + esc(r.name) + `</code></td><td>` + esc(r.size) + `</td><td><code>` + esc(r.sc) + `</code></td></tr>`)
		}
		b.WriteString(`</tbody></table>`)
	}
	return b.String()
}

// --- PVC sizing (MySQL data volume) ---

type psPVCSizingSection struct{}

func (psPVCSizingSection) ID() string    { return "ps-pvc-sizing" }
func (psPVCSizingSection) Title() string { return "Percona Server · MySQL volume sizing" }
func (psPVCSizingSection) Group() SectionGroup { return GroupPS }

func (psPVCSizingSection) Collect(ctx dumpctx.Context) (Section, error) {
	clusters, err := loadPSClusters(ctx.Root())
	if err != nil {
		return Section{}, err
	}
	rows := gatherPSPVCSizingRows(clusters)
	if len(rows) == 0 {
		return Section{}, nil
	}
	return Section{HTML: template.HTML(renderPSPVCSizing(rows))}, nil
}

type psPVCSizingRow struct {
	Cluster, Namespace, Size, StorageClass, VolumeExpansion string
}

func gatherPSPVCSizingRows(clusters []psClusterRecord) []psPVCSizingRow {
	var rows []psPVCSizingRow
	for _, c := range clusters {
		if c.Spec.MySQL.VolumeSpec == nil || c.Spec.MySQL.VolumeSpec.PersistentVolumeClaim == nil {
			continue
		}
		pvc := c.Spec.MySQL.VolumeSpec.PersistentVolumeClaim
		size := strings.TrimSpace(pvc.Resources.Requests.Storage)
		if size == "" {
			continue
		}
		sc := "—"
		if pvc.StorageClassName != nil {
			sc = strings.TrimSpace(*pvc.StorageClassName)
			if sc == "" {
				sc = "—"
			}
		}
		rows = append(rows, psPVCSizingRow{
			Cluster: c.Name, Namespace: c.Namespace,
			Size: k8sfmt.HumanQuantity(size), StorageClass: sc,
			VolumeExpansion: psBoolish(c.Spec.EnableVolumeExpansion),
		})
	}
	return rows
}

func renderPSPVCSizing(rows []psPVCSizingRow) string {
	esc := html.EscapeString
	var b strings.Builder
	b.WriteString(`<table class="pxc-cert-table"><thead><tr><th>Cluster</th><th>Namespace</th><th>mysql.volumeSpec size</th><th>storageClass</th><th>enableVolumeExpansion</th></tr></thead><tbody>`)
	for _, r := range rows {
		b.WriteString(`<tr><td class="pxc-cert-mono">` + esc(r.Cluster) + `</td><td>` + esc(r.Namespace))
		b.WriteString(`</td><td>` + esc(r.Size) + `</td><td><code>` + esc(r.StorageClass))
		b.WriteString(`</code></td><td>` + esc(r.VolumeExpansion) + `</td></tr>`)
	}
	b.WriteString(`</tbody></table>`)
	return b.String()
}

// --- Status extras ---

type psStatusSection struct{}

func (psStatusSection) ID() string    { return "ps-status" }
func (psStatusSection) Title() string { return "Percona Server · cluster status" }
func (psStatusSection) Group() SectionGroup { return GroupPS }

func (psStatusSection) Collect(ctx dumpctx.Context) (Section, error) {
	clusters, err := loadPSClusters(ctx.Root())
	if err != nil {
		return Section{}, err
	}
	if len(clusters) == 0 {
		return Section{}, nil
	}
	return Section{HTML: template.HTML(renderPSStatusExtras(clusters))}, nil
}

func renderPSStatusExtras(clusters []psClusterRecord) string {
	esc := html.EscapeString
	dash := "—"
	var b strings.Builder
	b.WriteString(`<p class="meta">Observed state from <code>status</code> on each CR: endpoint, overall state, component versions, and notable conditions.</p>`)
	b.WriteString(`<table class="pxc-cert-table"><thead><tr><th>Cluster</th><th>Namespace</th><th>host</th><th>state</th><th>MySQL</th><th>Proxy</th><th>Orchestrator</th><th>Binlog srv</th><th>Conditions</th></tr></thead><tbody>`)
	for _, c := range clusters {
		host := strings.TrimSpace(c.Status.Host)
		if host == "" {
			host = dash
		}
		state := strings.TrimSpace(c.Status.State)
		if state == "" {
			state = dash
		}
		mysql := psCompStatusShort(c.Status.MySQL)
		proxy := dash
		if psProxyEnabled(c.Spec.Proxy.HAProxy) {
			proxy = "HAProxy " + psCompStatusShort(c.Status.HAProxy)
		} else if psProxyEnabled(c.Spec.Proxy.Router) {
			proxy = "Router " + psCompStatusShort(c.Status.Router)
		}
		orch := dash
		if psOrchEnabled(c.Spec.Orchestrator) {
			orch = psCompStatusShort(c.Status.Orchestrator)
		}
		binlog := dash
		if c.Spec.Backup != nil && c.Spec.Backup.PITR.Enabled {
			binlog = psCompStatusShort(c.Status.BinlogServer)
		}
		conds := psNotableConditions(c.Status.Conditions)
		if conds == "" {
			conds = dash
		}
		b.WriteString(`<tr><td class="pxc-cert-mono">` + esc(c.Name) + `</td><td>` + esc(c.Namespace))
		b.WriteString(`</td><td><code>` + esc(host) + `</code></td><td>` + esc(state))
		b.WriteString(`</td><td>` + esc(mysql) + `</td><td>` + esc(proxy) + `</td><td>` + esc(orch))
		b.WriteString(`</td><td>` + esc(binlog) + `</td><td class="pxc-cert-mono" style="font-size:0.72rem">` + esc(conds) + `</td></tr>`)
	}
	b.WriteString(`</tbody></table>`)
	return b.String()
}

func psCompStatusShort(s psComponentStatusYAML) string {
	if s.Ready == 0 && s.Size == 0 && strings.TrimSpace(s.State) == "" {
		return "—"
	}
	st := strings.TrimSpace(s.State)
	if st == "" {
		st = "—"
	}
	ver := strings.TrimSpace(s.Version)
	out := fmt.Sprintf("%d/%d ready · %s", s.Ready, s.Size, st)
	if ver != "" {
		out += " · " + ver
	}
	return out
}

func psNotableConditions(conds []psCRCondition) string {
	want := map[string]bool{"Ready": true, "Initializing": true, "InnoDBClusterBootstrapped": true}
	var parts []string
	for _, c := range conds {
		if !want[c.Type] {
			continue
		}
		p := c.Type + "=" + strings.TrimSpace(c.Status)
		if r := strings.TrimSpace(c.Reason); r != "" {
			p += " (" + r + ")"
		}
		parts = append(parts, p)
	}
	return strings.Join(parts, "; ")
}

// --- Topology summary (also used from pod logs) ---

type psTopologySection struct{}

func (psTopologySection) ID() string    { return "ps-topology" }
func (psTopologySection) Title() string { return "Percona Server · topology" }
func (psTopologySection) Group() SectionGroup { return GroupPS }

func (psTopologySection) Collect(ctx dumpctx.Context) (Section, error) {
	h, err := renderPSTopologyHTML(ctx.Root(), nil, nil)
	if err != nil {
		return Section{}, err
	}
	if h == "" {
		return Section{}, nil
	}
	return Section{HTML: template.HTML(h)}, nil
}
