package collector

import (
	"fmt"
	"html"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"pt-k8s-summary/internal/dumpctx"
	"pt-k8s-summary/internal/dumpfiles"

	"gopkg.in/yaml.v3"
)

const psCRListFile = "perconaservermysqls.ps.percona.com.yaml"

func findPSCRListYAMLs(dumpRoot string) ([]string, error) {
	return dumpfiles.FindListYAMLFiles(dumpRoot, dumpfiles.PSClusterList)
}

type psHelmPMMSection struct{}

func (psHelmPMMSection) ID() string    { return "ps-helm-pmm" }
func (psHelmPMMSection) Title() string { return "Percona Server · Helm & PMM" }

func (psHelmPMMSection) Collect(ctx dumpctx.Context) (Section, error) {
	pairs, err := gatherPSHelmPMMPairs(ctx.Root())
	if err != nil {
		return Section{}, err
	}
	if len(pairs) == 0 {
		return Section{}, nil
	}
	return Section{HTML: template.HTML(renderPXCHelmPMMPairs(pairs))}, nil
}

func gatherPSHelmPMMPairs(dumpRoot string) ([]pxcHelmPMMPair, error) {
	paths, err := findPSCRListYAMLs(dumpRoot)
	if err != nil {
		return nil, err
	}
	var out []pxcHelmPMMPair
	for _, p := range paths {
		nsHint := filepath.Base(filepath.Dir(p))
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", p, err)
		}
		var list pxcUnifiedListDoc
		if err := yaml.Unmarshal(data, &list); err != nil {
			return nil, fmt.Errorf("yaml %s: %w", p, err)
		}
		for i := range list.Items {
			md := list.Items[i].Metadata
			name := strings.TrimSpace(md.Name)
			if name == "" {
				continue
			}
			ns := strings.TrimSpace(md.Namespace)
			if ns == "" {
				ns = nsHint
			}
			lbl := md.Labels
			if lbl == nil {
				lbl = map[string]string{}
			}
			ann := md.Annotations
			if ann == nil {
				ann = map[string]string{}
			}
			pm := list.Items[i].Spec.PMM
			img := strings.TrimSpace(pm.Image)
			out = append(out, pxcHelmPMMPair{
				Name: name, Namespace: ns,
				Helm: helmPXCRow{
					Name: name, Namespace: ns,
					IsHelm: strings.EqualFold(strings.TrimSpace(lbl["app.kubernetes.io/managed-by"]), "Helm") || strings.TrimSpace(ann["meta.helm.sh/release-name"]) != "",
					ManagedBy: strings.TrimSpace(lbl["app.kubernetes.io/managed-by"]),
					ReleaseName: strings.TrimSpace(ann["meta.helm.sh/release-name"]),
					Chart: strings.TrimSpace(lbl["helm.sh/chart"]),
				},
				PMM: pmmRow{
					Name: name, Namespace: ns, Enabled: pm.Enabled,
					ClientImage: img, ClientTag: imageTagFromRef(img),
					ServerHost: strings.TrimSpace(pm.ServerHost),
				},
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Namespace != out[j].Namespace {
			return out[i].Namespace < out[j].Namespace
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

type psPITRSection struct{}

func (psPITRSection) ID() string    { return "ps-pitr" }
func (psPITRSection) Title() string { return "Percona Server · PITR & backup" }

func (psPITRSection) Collect(ctx dumpctx.Context) (Section, error) {
	clusters, err := loadPSClusters(ctx.Root())
	if err != nil {
		return Section{}, err
	}
	if len(clusters) == 0 {
		return Section{}, nil
	}
	return Section{HTML: template.HTML(renderPSPITRDetail(clusters))}, nil
}

func renderPSPITRDetail(clusters []psClusterRecord) string {
	esc := html.EscapeString
	var b strings.Builder
	b.WriteString(`<p class="meta">Backup and point-in-time recovery settings from <code>spec.backup</code>. When PITR is enabled, the binlog server block is required.</p>`)
	b.WriteString(`<table class="pxc-cert-table"><thead><tr><th>Cluster</th><th>Namespace</th><th>Backup</th><th>PITR</th><th>Backup image</th><th>sourcePod</th><th>backoffLimit</th></tr></thead><tbody>`)
	for _, c := range clusters {
		if c.Spec.Backup == nil {
			continue
		}
		bu := `<span class="pitr-pill off">off</span>`
		if c.Spec.Backup.Enabled {
			bu = `<span class="pitr-pill on">on</span>`
		}
		pi := `<span class="pitr-pill off">off</span>`
		if c.Spec.Backup.PITR.Enabled {
			pi = `<span class="pitr-pill on">on</span>`
		}
		img := strings.TrimSpace(c.Spec.Backup.Image)
		if img == "" {
			img = "—"
		}
		src := strings.TrimSpace(c.Spec.Backup.SourcePod)
		if src == "" {
			src = "—"
		}
		bl := "—"
		if c.Spec.Backup.BackoffLimit != nil {
			bl = fmt.Sprintf("%d", *c.Spec.Backup.BackoffLimit)
		}
		b.WriteString(`<tr><td class="pxc-cert-mono">` + esc(c.Name) + `</td><td>` + esc(c.Namespace))
		b.WriteString(`</td><td>` + bu + `</td><td>` + pi + `</td><td><code>` + esc(img))
		b.WriteString(`</code></td><td><code>` + esc(src) + `</code></td><td>` + esc(bl) + `</td></tr>`)
	}
	b.WriteString(`</tbody></table>`)

	var binlogRows []struct {
		cluster, ns, image, bucket, region, cred, ckpt, ssl string
		size                                                string
	}
	for _, c := range clusters {
		if c.Spec.Backup == nil || !c.Spec.Backup.PITR.Enabled || c.Spec.Backup.PITR.BinlogServer == nil {
			continue
		}
		bs := c.Spec.Backup.PITR.BinlogServer
		row := struct {
			cluster, ns, image, bucket, region, cred, ckpt, ssl string
			size                                                string
		}{c.Name, c.Namespace, strings.TrimSpace(bs.Image), "—", "—", "—", "—", "—", fmt.Sprintf("%d", bs.Size)}
		if bs.Storage.S3 != nil {
			row.bucket = strings.TrimSpace(bs.Storage.S3.Bucket)
			if p := strings.TrimSpace(bs.Storage.S3.Prefix); p != "" {
				row.bucket += "/" + p
			}
			row.region = strings.TrimSpace(bs.Storage.S3.Region)
			row.cred = strings.TrimSpace(bs.Storage.S3.CredentialsSecret)
		}
		parts := []string{}
		if bs.CheckpointInterval != "" {
			parts = append(parts, "interval "+bs.CheckpointInterval)
		}
		if bs.CheckpointSize != "" {
			parts = append(parts, "size "+bs.CheckpointSize)
		}
		if len(parts) > 0 {
			row.ckpt = strings.Join(parts, ", ")
		}
		if bs.SSLMode != "" {
			row.ssl = bs.SSLMode
		}
		binlogRows = append(binlogRows, row)
	}
	if len(binlogRows) > 0 {
		var tb strings.Builder
		tb.WriteString(psTableOpen("pxc-cert-table"))
		tb.WriteString(`<tr><th>Cluster</th><th>Namespace</th><th>size</th><th>image</th><th>S3 bucket</th><th>region</th><th>credentialsSecret</th><th>checkpoint</th><th>sslMode</th></tr></thead><tbody>`)
		for _, r := range binlogRows {
			tb.WriteString(`<tr><td class="pxc-cert-mono">` + esc(r.cluster) + `</td><td>` + esc(r.ns))
			tb.WriteString(`</td><td>` + esc(r.size) + `</td><td><code>` + esc(r.image))
			tb.WriteString(`</code></td><td><code>` + esc(r.bucket) + `</code></td><td>` + esc(r.region))
			tb.WriteString(`</td><td><code>` + esc(r.cred) + `</code></td><td>` + esc(r.ckpt))
			tb.WriteString(`</td><td><code>` + esc(r.ssl) + `</code></td></tr>`)
		}
		tb.WriteString(`</tbody></table>`)
		b.WriteString(`<h4 class="pxc-subsection-title">Binlog server (PITR)</h4>`)
		b.WriteString(tb.String())
	}
	return b.String()
}

// removed legacy gatherPSPITRRows — use loadPSClusters + renderPSPITRDetail

func _legacyGatherPSPITRRows_removed() {
	_ = psPITRRow{}
}

type psPITRRow struct {
	Name, Namespace string
	BackupEnabled   bool
	PITREnabled     bool
	BackupImage     string
	StorageNames    string
}

func gatherPSPITRRows(dumpRoot string) ([]psPITRRow, error) {
	paths, err := findPSCRListYAMLs(dumpRoot)
	if err != nil {
		return nil, err
	}
	var rows []psPITRRow
	for _, p := range paths {
		nsHint := filepath.Base(filepath.Dir(p))
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		var list struct {
			Items []struct {
				Metadata struct {
					Name      string `yaml:"name"`
					Namespace string `yaml:"namespace"`
				} `yaml:"metadata"`
				Spec struct {
					Backup struct {
						Enabled  bool   `yaml:"enabled"`
						Image    string `yaml:"image"`
						PITR     struct{ Enabled bool `yaml:"enabled"` } `yaml:"pitr"`
						Storages map[string]interface{} `yaml:"storages"`
					} `yaml:"backup"`
				} `yaml:"spec"`
			} `yaml:"items"`
		}
		if err := yaml.Unmarshal(data, &list); err != nil {
			return nil, err
		}
		for _, item := range list.Items {
			name := strings.TrimSpace(item.Metadata.Name)
			if name == "" {
				continue
			}
			ns := strings.TrimSpace(item.Metadata.Namespace)
			if ns == "" {
				ns = nsHint
			}
			storages := "—"
			if len(item.Spec.Backup.Storages) > 0 {
				names := make([]string, 0, len(item.Spec.Backup.Storages))
				for k := range item.Spec.Backup.Storages {
					names = append(names, k)
				}
				sort.Strings(names)
				storages = strings.Join(names, ", ")
			}
			img := strings.TrimSpace(item.Spec.Backup.Image)
			if img == "" {
				img = "—"
			}
			rows = append(rows, psPITRRow{
				Name: name, Namespace: ns,
				BackupEnabled: item.Spec.Backup.Enabled,
				PITREnabled:   item.Spec.Backup.PITR.Enabled,
				BackupImage:   img,
				StorageNames:  storages,
			})
		}
	}
	return rows, nil
}

func renderPSPITRTable(rows []psPITRRow) string {
	var b strings.Builder
	b.WriteString(`<div class="pitr-wrap"><table class="pitr-table"><thead><tr>`)
	b.WriteString(`<th>Cluster</th><th>Namespace</th><th>Backup</th><th>PITR</th><th>Storages</th><th>Backup image</th>`)
	b.WriteString(`</tr></thead><tbody>`)
	for _, r := range rows {
		bu := `<span class="pitr-pill off">off</span>`
		if r.BackupEnabled {
			bu = `<span class="pitr-pill on">on</span>`
		}
		pi := `<span class="pitr-pill off">off</span>`
		if r.PITREnabled {
			pi = `<span class="pitr-pill on">on</span>`
		}
		b.WriteString(`<tr><td><code>` + html.EscapeString(r.Name) + `</code></td><td><code>`)
		b.WriteString(html.EscapeString(r.Namespace) + `</code></td><td>` + bu + `</td><td>` + pi)
		b.WriteString(`</td><td><code>` + html.EscapeString(r.StorageNames) + `</code></td><td><code>`)
		b.WriteString(html.EscapeString(r.BackupImage) + `</code></td></tr>`)
	}
	b.WriteString(`</tbody></table></div>`)
	return b.String()
}

type psUpdateStrategySection struct{}

func (psUpdateStrategySection) ID() string    { return "ps-update-strategy" }
func (psUpdateStrategySection) Title() string { return "Percona Server · updateStrategy" }

func (psUpdateStrategySection) Collect(ctx dumpctx.Context) (Section, error) {
	rows, err := gatherPSUpdateStrategyRows(ctx.Root())
	if err != nil {
		return Section{}, err
	}
	if len(rows) == 0 {
		return Section{}, nil
	}
	return Section{HTML: template.HTML(renderPSUpdateStrategy(rows))}, nil
}

func gatherPSUpdateStrategyRows(dumpRoot string) ([]pxcUpdateStrategyRow, error) {
	paths, err := findPSCRListYAMLs(dumpRoot)
	if err != nil {
		return nil, err
	}
	var rows []pxcUpdateStrategyRow
	for _, p := range paths {
		nsHint := filepath.Base(filepath.Dir(p))
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		var list struct {
			Items []struct {
				Metadata struct {
					Name      string `yaml:"name"`
					Namespace string `yaml:"namespace"`
				} `yaml:"metadata"`
				Spec     struct{ UpdateStrategy string `yaml:"updateStrategy"` } `yaml:"spec"`
			} `yaml:"items"`
		}
		if err := yaml.Unmarshal(data, &list); err != nil {
			return nil, err
		}
		for _, item := range list.Items {
			name := strings.TrimSpace(item.Metadata.Name)
			if name == "" {
				continue
			}
			ns := strings.TrimSpace(item.Metadata.Namespace)
			if ns == "" {
				ns = nsHint
			}
			val := strings.TrimSpace(item.Spec.UpdateStrategy)
			src := "spec.updateStrategy"
			if val == "" {
				val = "—"
				src = ""
			}
			rows = append(rows, pxcUpdateStrategyRow{Name: name, Namespace: ns, Value: val, SourceField: src})
		}
	}
	return rows, nil
}

func renderPSUpdateStrategy(rows []pxcUpdateStrategyRow) string {
	return renderPXCUpdateStrategy(rows)
}

func gatherPSUnsafeFlagRows(dumpRoot string) ([]unsafeFlagCluster, error) {
	paths, err := findPSCRListYAMLs(dumpRoot)
	if err != nil {
		return nil, err
	}
	var out []unsafeFlagCluster
	for _, p := range paths {
		nsHint := filepath.Base(filepath.Dir(p))
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", p, err)
		}
		var list unsafePXCListDoc
		if err := yaml.Unmarshal(data, &list); err != nil {
			return nil, fmt.Errorf("yaml %s: %w", p, err)
		}
		for i := range list.Items {
			md := list.Items[i].Metadata
			name := strings.TrimSpace(md.Name)
			if name == "" {
				continue
			}
			ns := strings.TrimSpace(md.Namespace)
			if ns == "" {
				ns = nsHint
			}
			raw := list.Items[i].Spec.UnsafeFlags
			flags := make(map[string]unsafeFlagTri)
			for _, key := range psUnsafeFlagKeys {
				if raw == nil {
					flags[key] = unsafeFlagTri{Present: false}
					continue
				}
				v, ok := raw[key]
				if !ok {
					flags[key] = unsafeFlagTri{Present: false}
					continue
				}
				b, parsed := parseBoolish(v)
				if !parsed {
					flags[key] = unsafeFlagTri{Present: false}
					continue
				}
				flags[key] = unsafeFlagTri{Present: true, Value: b}
			}
			out = append(out, unsafeFlagCluster{Name: name, Namespace: ns, Flags: flags})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Namespace != out[j].Namespace {
			return out[i].Namespace < out[j].Namespace
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func gatherPSPauseRows(dumpRoot string) ([]pauseRow, error) {
	paths, err := findPSCRListYAMLs(dumpRoot)
	if err != nil {
		return nil, err
	}
	var rows []pauseRow
	for _, p := range paths {
		nsHint := filepath.Base(filepath.Dir(p))
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", p, err)
		}
		var list pausePXCListDoc
		if err := yaml.Unmarshal(data, &list); err != nil {
			return nil, fmt.Errorf("yaml %s: %w", p, err)
		}
		for i := range list.Items {
			md := list.Items[i].Metadata
			name := strings.TrimSpace(md.Name)
			if name == "" {
				continue
			}
			ns := strings.TrimSpace(md.Namespace)
			if ns == "" {
				ns = nsHint
			}
			pp := list.Items[i].Spec.Pause
			pr := pauseRow{Name: name, Namespace: ns}
			if pp != nil {
				pr.Present = true
				pr.Paused = *pp
			}
			rows = append(rows, pr)
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Namespace != rows[j].Namespace {
			return rows[i].Namespace < rows[j].Namespace
		}
		return rows[i].Name < rows[j].Name
	})
	return rows, nil
}

type psUnsafePauseSection struct{}

func (psUnsafePauseSection) ID() string    { return "ps-unsafe-flags-pause" }
func (psUnsafePauseSection) Title() string { return "" }

func (psUnsafePauseSection) Collect(ctx dumpctx.Context) (Section, error) {
	unsafeRows, err := gatherPSUnsafeFlagRows(ctx.Root())
	if err != nil {
		return Section{}, err
	}
	pauseRows, err := gatherPSPauseRows(ctx.Root())
	if err != nil {
		return Section{}, err
	}
	if len(unsafeRows) == 0 && len(pauseRows) == 0 {
		return Section{}, nil
	}
	return Section{HTML: template.HTML(renderPSUnsafePause(unsafeRows, pauseRows))}, nil
}

var psUnsafeFlagKeys = []string{"backupNonReadyCluster", "mysqlSize", "orchestrator", "orchestratorSize", "proxy", "proxySize"}

func renderPSUnsafePause(unsafeClusters []unsafeFlagCluster, pauseRows []pauseRow) string {
	htmlStr := renderUnsafePauseRowWithKeys(unsafeClusters, pauseRows, psUnsafeFlagKeys, "ps-unsafe-flags-pause", "Percona Server · unsafeFlags", "Percona Server · spec.pause", psCRListFile, "PerconaServerMySQL")
	return htmlStr
}

type psExposeSection struct{}

func (psExposeSection) ID() string    { return "ps-expose" }
func (psExposeSection) Title() string { return "Percona Server · expose" }

func (psExposeSection) Collect(ctx dumpctx.Context) (Section, error) {
	rows, err := gatherPSExposeRows(ctx.Root())
	if err != nil {
		return Section{}, err
	}
	if len(rows) == 0 {
		return Section{}, nil
	}
	return Section{HTML: template.HTML(renderPSExpose(rows))}, nil
}

type psExposeRow struct {
	Name, Namespace, Target string
	EnabledDisplay          string
	EnabledTrue             bool
	Type                    string
}

func gatherPSExposeRows(dumpRoot string) ([]psExposeRow, error) {
	paths, err := findPSCRListYAMLs(dumpRoot)
	if err != nil {
		return nil, err
	}
	var out []psExposeRow
	for _, p := range paths {
		nsHint := filepath.Base(filepath.Dir(p))
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		var list struct {
			Items []struct {
				Metadata struct {
					Name      string `yaml:"name"`
					Namespace string `yaml:"namespace"`
				} `yaml:"metadata"`
				Spec struct {
					MySQL struct {
						ExposePrimary *struct{ Enabled *bool `yaml:"enabled"`; Type string `yaml:"type"` } `yaml:"exposePrimary"`
						Expose        *struct{ Enabled *bool `yaml:"enabled"`; Type string `yaml:"type"` } `yaml:"expose"`
					} `yaml:"mysql"`
					Proxy struct {
						HAProxy *struct {
							Expose *struct{ Enabled *bool `yaml:"enabled"`; Type string `yaml:"type"` } `yaml:"expose"`
						} `yaml:"haproxy"`
					} `yaml:"proxy"`
				} `yaml:"spec"`
			} `yaml:"items"`
		}
		if err := yaml.Unmarshal(data, &list); err != nil {
			return nil, err
		}
		for _, item := range list.Items {
			name := strings.TrimSpace(item.Metadata.Name)
			if name == "" {
				continue
			}
			ns := strings.TrimSpace(item.Metadata.Namespace)
			if ns == "" {
				ns = nsHint
			}
			appendExpose := func(target string, block *struct{ Enabled *bool `yaml:"enabled"`; Type string `yaml:"type"` }) {
				if block == nil {
					return
				}
				row := psExposeRow{Name: name, Namespace: ns, Target: target}
				switch {
				case block.Enabled == nil:
					row.EnabledDisplay = "not set"
				case *block.Enabled:
					row.EnabledDisplay = "true"
					row.EnabledTrue = true
				default:
					row.EnabledDisplay = "false"
				}
				row.Type = strings.TrimSpace(block.Type)
				if row.Type == "" {
					row.Type = "—"
				}
				out = append(out, row)
			}
			if item.Spec.MySQL.ExposePrimary != nil {
				appendExpose("mysql.exposePrimary", item.Spec.MySQL.ExposePrimary)
			}
			if item.Spec.MySQL.Expose != nil {
				appendExpose("mysql.expose", item.Spec.MySQL.Expose)
			}
			if item.Spec.Proxy.HAProxy != nil && item.Spec.Proxy.HAProxy.Expose != nil {
				appendExpose("proxy.haproxy.expose", item.Spec.Proxy.HAProxy.Expose)
			}
		}
	}
	return out, nil
}

func renderPSExpose(rows []psExposeRow) string {
	var b strings.Builder
	b.WriteString(`<table class="pxc-cert-table"><thead><tr><th>Cluster</th><th>Namespace</th><th>Target</th><th>enabled</th><th>type</th></tr></thead><tbody>`)
	for _, r := range rows {
		b.WriteString(`<tr><td class="pxc-cert-mono">` + html.EscapeString(r.Name) + `</td><td>`)
		b.WriteString(html.EscapeString(r.Namespace) + `</td><td class="pxc-cert-mono">` + html.EscapeString(r.Target))
		b.WriteString(`</td><td>` + html.EscapeString(r.EnabledDisplay) + `</td><td><code>`)
		b.WriteString(html.EscapeString(r.Type) + `</code></td></tr>`)
	}
	b.WriteString(`</tbody></table>`)
	return b.String()
}
