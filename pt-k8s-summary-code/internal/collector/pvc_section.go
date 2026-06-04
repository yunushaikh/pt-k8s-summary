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
	"pt-k8s-summary/internal/k8sfmt"

	"gopkg.in/yaml.v3"
)

const (
	pvcFileName  = "persistentvolumeclaims.yaml"
	podsFileName = "pods.yaml"
	pvFileName   = "persistentvolumes.yaml"
)

// pvcInventorySection renders PersistentVolumeClaim usage from namespace dumps.
type pvcInventorySection struct{}

func (pvcInventorySection) ID() string    { return "pvc-storage-atlas" }
func (pvcInventorySection) Title() string { return "Persistent volume claims" }

func (pvcInventorySection) Collect(ctx dumpctx.Context) (Section, error) {
	rows, err := gatherPVCRows(ctx.Root())
	if err != nil {
		return Section{}, err
	}
	if len(rows) == 0 {
		return Section{}, nil
	}
	return Section{HTML: template.HTML(renderPVCAtlas(rows))}, nil
}

type pvcRow struct {
	Namespace     string
	Name          string
	Phase         string
	StorageClass  string
	RequestRaw    string
	CapacityRaw   string
	RequestHuman  string
	CapacityHuman string
	PodNames      []string
	PVName        string
}

func gatherPVCRows(dumpRoot string) ([]pvcRow, error) {
	paths, err := findYAMLFiles(dumpRoot, pvcFileName)
	if err != nil {
		return nil, err
	}
	globalPVCap, err := loadAllPVCapacityByClaim(dumpRoot)
	if err != nil {
		return nil, err
	}
	var rows []pvcRow
	for _, pvcPath := range paths {
		nsDir := filepath.Dir(pvcPath)
		nsHint := filepath.Base(nsDir)

		data, err := os.ReadFile(pvcPath)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", pvcPath, err)
		}
		var list pvcListDoc
		if err := yaml.Unmarshal(data, &list); err != nil {
			return nil, fmt.Errorf("parse %s: %w", pvcPath, err)
		}

		claimToPods, err := loadClaimToPods(filepath.Join(nsDir, podsFileName), nsHint)
		if err != nil {
			return nil, err
		}
		localPVCap := loadPVCapacityByClaim(filepath.Join(nsDir, pvFileName))

		for i := range list.Items {
			it := &list.Items[i]
			name := strings.TrimSpace(it.Metadata.Name)
			if name == "" {
				continue
			}
			ns := strings.TrimSpace(it.Metadata.Namespace)
			if ns == "" {
				ns = nsHint
			}
			sc := strings.TrimSpace(it.Spec.StorageClassName)
			req := ""
			if it.Spec.Resources.Requests != nil {
				req = strings.TrimSpace(it.Spec.Resources.Requests["storage"])
			}
			cap := ""
			if it.Status.Capacity != nil {
				cap = strings.TrimSpace(it.Status.Capacity["storage"])
			}
			if cap == "" {
				key := claimKey(ns, name)
				if v, ok := localPVCap[key]; ok {
					cap = v
				} else if v, ok := globalPVCap[key]; ok {
					cap = v
				}
			}
			phase := strings.TrimSpace(it.Status.Phase)
			if phase == "" {
				phase = "Unknown"
			}
			pods := claimToPods[claimKey(ns, name)]
			sort.Strings(pods)

			rows = append(rows, pvcRow{
				Namespace:     ns,
				Name:          name,
				Phase:         phase,
				StorageClass:  sc,
				RequestRaw:    req,
				CapacityRaw:   cap,
				RequestHuman:  k8sfmt.HumanQuantity(req),
				CapacityHuman: k8sfmt.HumanQuantity(cap),
				PodNames:      pods,
				PVName:        strings.TrimSpace(it.Spec.VolumeName),
			})
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

func claimKey(ns, claim string) string { return ns + "\x00" + claim }

func findYAMLFiles(root, wantBase string) ([]string, error) {
	root = filepath.Clean(root)
	var out []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Base(path), wantBase) {
			out = append(out, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

type pvcListDoc struct {
	Items []pvcItemYAML `yaml:"items"`
}

type pvcItemYAML struct {
	Metadata struct {
		Name      string `yaml:"name"`
		Namespace string `yaml:"namespace"`
	} `yaml:"metadata"`
	Spec struct {
		StorageClassName string           `yaml:"storageClassName"`
		Resources        pvcResourcesYAML `yaml:"resources"`
		VolumeName       string           `yaml:"volumeName"`
	} `yaml:"spec"`
	Status struct {
		Phase    string            `yaml:"phase"`
		Capacity map[string]string `yaml:"capacity"`
	} `yaml:"status"`
}

type pvcResourcesYAML struct {
	Requests map[string]string `yaml:"requests"`
}

func loadClaimToPods(podsPath, defaultNS string) (map[string][]string, error) {
	out := make(map[string][]string)
	data, err := os.ReadFile(podsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, fmt.Errorf("pods %s: %w", podsPath, err)
	}
	var list podListDoc
	if err := yaml.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("pods yaml %s: %w", podsPath, err)
	}
	for i := range list.Items {
		pod := &list.Items[i]
		podName := strings.TrimSpace(pod.Metadata.Name)
		ns := strings.TrimSpace(pod.Metadata.Namespace)
		if ns == "" {
			ns = defaultNS
		}
		if podName == "" {
			continue
		}
		for _, v := range pod.Spec.Volumes {
			if v.PersistentVolumeClaim == nil {
				continue
			}
			cn := strings.TrimSpace(v.PersistentVolumeClaim.ClaimName)
			if cn == "" {
				continue
			}
			key := claimKey(ns, cn)
			out[key] = appendUnique(out[key], podName)
		}
	}
	return out, nil
}

func appendUnique(slice []string, v string) []string {
	for _, x := range slice {
		if x == v {
			return slice
		}
	}
	return append(slice, v)
}

type podListDoc struct {
	Items []podItemYAML `yaml:"items"`
}

type podItemYAML struct {
	Metadata struct {
		Name      string `yaml:"name"`
		Namespace string `yaml:"namespace"`
	} `yaml:"metadata"`
	Spec struct {
		Volumes []volumeYAML `yaml:"volumes"`
	} `yaml:"spec"`
}

type volumeYAML struct {
	Name                  string `yaml:"name"`
	PersistentVolumeClaim *struct {
		ClaimName string `yaml:"claimName"`
	} `yaml:"persistentVolumeClaim"`
}

type pvListDoc struct {
	Items []pvItemYAML `yaml:"items"`
}

type pvItemYAML struct {
	Spec struct {
		Capacity map[string]string `yaml:"capacity"`
		ClaimRef *struct {
			Name      string `yaml:"name"`
			Namespace string `yaml:"namespace"`
		} `yaml:"claimRef"`
	} `yaml:"spec"`
}

func loadAllPVCapacityByClaim(dumpRoot string) (map[string]string, error) {
	paths, err := findYAMLFiles(dumpRoot, pvFileName)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string)
	for _, p := range paths {
		for k, v := range loadPVCapacityByClaim(p) {
			out[k] = v
		}
	}
	return out, nil
}

func loadPVCapacityByClaim(pvPath string) map[string]string {
	out := make(map[string]string)
	data, err := os.ReadFile(pvPath)
	if err != nil {
		return out
	}
	var list pvListDoc
	if err := yaml.Unmarshal(data, &list); err != nil {
		return out
	}
	for i := range list.Items {
		ref := list.Items[i].Spec.ClaimRef
		if ref == nil {
			continue
		}
		name := strings.TrimSpace(ref.Name)
		ns := strings.TrimSpace(ref.Namespace)
		if name == "" {
			continue
		}
		st := ""
		if list.Items[i].Spec.Capacity != nil {
			st = strings.TrimSpace(list.Items[i].Spec.Capacity["storage"])
		}
		if st != "" {
			out[claimKey(ns, name)] = st
		}
	}
	return out
}

func renderPVCAtlas(rows []pvcRow) string {
	var b strings.Builder
	b.WriteString(`<style>
.pvcviz-wrap { --pvc-bg: #0f1419; --pvc-card: #1a2332; --pvc-line: rgba(148,163,184,.18); --pvc-accent: #38bdf8; --pvc-mint: #34d399; --pvc-amber: #fbbf24; --pvc-rose: #fb7185; --pvc-muted: #94a3b8; font-family: ui-sans-serif, system-ui, sans-serif; color: #e2e8f0; border-radius: 16px; padding: 1.25rem 1.35rem; background: linear-gradient(145deg, #0c1220 0%, var(--pvc-bg) 40%, #111827 100%); border: 1px solid var(--pvc-line); box-shadow: 0 20px 50px rgba(0,0,0,.35); }
.pvcviz-head { display: flex; flex-wrap: wrap; align-items: baseline; justify-content: space-between; gap: 0.75rem; margin-bottom: 1.1rem; }
.pvcviz-head h4 { margin: 0; font-size: 1.05rem; font-weight: 650; letter-spacing: -0.02em; background: linear-gradient(90deg, #f8fafc, var(--pvc-accent)); -webkit-background-clip: text; background-clip: text; color: transparent; }
.pvcviz-head span { font-size: 0.78rem; color: var(--pvc-muted); max-width: 36rem; line-height: 1.45; }
.pvcviz-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(300px, 1fr)); gap: 1rem; }
.pvcviz-card { background: linear-gradient(180deg, rgba(30,41,59,.9), rgba(15,23,42,.95)); border: 1px solid var(--pvc-line); border-radius: 14px; padding: 1rem 1.05rem; position: relative; overflow: hidden; }
.pvcviz-card::before { content: ""; position: absolute; inset: 0 0 auto 0; height: 3px; background: linear-gradient(90deg, var(--pvc-accent), var(--pvc-mint)); opacity: 0.85; }
.pvcviz-card-head { display: flex; justify-content: space-between; align-items: flex-start; gap: 0.5rem; margin-bottom: 0.85rem; }
.pvcviz-name { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 0.82rem; font-weight: 600; color: #f1f5f9; word-break: break-all; }
.pvcviz-ns { font-size: 0.72rem; color: var(--pvc-muted); text-transform: uppercase; letter-spacing: 0.06em; margin-top: 0.2rem; }
.pvcviz-badge { font-size: 0.68rem; font-weight: 700; text-transform: uppercase; letter-spacing: 0.04em; padding: 0.22rem 0.55rem; border-radius: 999px; white-space: nowrap; }
.pvcviz-badge.bound { background: rgba(52,211,153,.15); color: var(--pvc-mint); border: 1px solid rgba(52,211,153,.35); }
.pvcviz-badge.pending { background: rgba(251,191,36,.12); color: var(--pvc-amber); border: 1px solid rgba(251,191,36,.35); }
.pvcviz-badge.lost, .pvcviz-badge.failed { background: rgba(251,113,133,.12); color: var(--pvc-rose); border: 1px solid rgba(251,113,133,.35); }
.pvcviz-badge.other { background: rgba(148,163,184,.12); color: #cbd5e1; border: 1px solid rgba(148,163,184,.25); }
.pvcviz-dl { display: grid; grid-template-columns: 1fr 1fr; gap: 0.45rem 0.75rem; font-size: 0.78rem; margin-bottom: 0.85rem; }
.pvcviz-dt { color: var(--pvc-muted); }
.pvcviz-dd { font-family: ui-monospace, Menlo, monospace; color: #e2e8f0; text-align: right; }
.pvcviz-pods { font-size: 0.76rem; color: #cbd5e1; margin-bottom: 0; line-height: 1.5; }
.pvcviz-pods strong { color: #94a3b8; font-weight: 600; }
.pvcviz-chip { display: inline-block; margin: 0.12rem 0.2rem 0 0; padding: 0.12rem 0.45rem; border-radius: 6px; background: rgba(56,189,248,.1); border: 1px solid rgba(56,189,248,.25); font-family: ui-monospace, monospace; font-size: 0.72rem; color: #bae6fd; }
</style>`)
	b.WriteString(`<div class="pvcviz-wrap">`)
	b.WriteString(`<div class="pvcviz-head"><h4>Storage atlas</h4>`)
	b.WriteString(`<span>Claims from <code style="color:#7dd3fc;">persistentvolumeclaims.yaml</code> per namespace · pod links from sibling <code style="color:#7dd3fc;">pods.yaml</code> · capacity from claim status or <code style="color:#7dd3fc;">persistentvolumes.yaml</code>.</span></div>`)
	b.WriteString(`<div class="pvcviz-grid">`)
	for _, r := range rows {
		renderPVCCard(&b, r)
	}
	b.WriteString(`</div></div>`)
	return b.String()
}

func renderPVCCard(b *strings.Builder, r pvcRow) {
	b.WriteString(`<article class="pvcviz-card"><div class="pvcviz-card-head"><div><div class="pvcviz-name">`)
	b.WriteString(html.EscapeString(r.Name))
	b.WriteString(`</div><div class="pvcviz-ns">`)
	b.WriteString(html.EscapeString(r.Namespace))
	b.WriteString(`</div></div>`)
	b.WriteString(phaseBadge(r.Phase))
	b.WriteString(`</div><dl class="pvcviz-dl">`)
	dlRow(b, "Storage class", displayOrDash(r.StorageClass))
	dlRow(b, "Requested", r.RequestHuman)
	dlRow(b, "Capacity", r.CapacityHuman)
	dlRow(b, "PV", displayOrDash(r.PVName))
	b.WriteString(`</dl>`)
	b.WriteString(`<div class="pvcviz-pods"><strong>Attached pods</strong> · `)
	if len(r.PodNames) == 0 {
		b.WriteString(`<span style="color:#64748b;">None referenced in pods.yaml (sidecars / other ns / not mounted yet)</span>`)
	} else {
		for _, p := range r.PodNames {
			b.WriteString(`<span class="pvcviz-chip">`)
			b.WriteString(html.EscapeString(p))
			b.WriteString(`</span>`)
		}
	}
	b.WriteString(`</div></article>`)
}

func dlRow(b *strings.Builder, label, value string) {
	b.WriteString(`<dt class="pvcviz-dt">`)
	b.WriteString(html.EscapeString(label))
	b.WriteString(`</dt><dd class="pvcviz-dd">`)
	b.WriteString(html.EscapeString(value))
	b.WriteString(`</dd>`)
}

func displayOrDash(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "—"
	}
	return s
}

func phaseBadge(phase string) string {
	p := strings.ToLower(strings.TrimSpace(phase))
	cls := "other"
	switch p {
	case "bound":
		cls = "bound"
	case "pending":
		cls = "pending"
	case "lost", "failed":
		cls = "lost"
	}
	return fmt.Sprintf(`<span class="pvcviz-badge %s">%s</span>`, cls, html.EscapeString(phase))
}
