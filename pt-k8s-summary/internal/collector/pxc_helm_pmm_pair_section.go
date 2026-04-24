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

	"gopkg.in/yaml.v3"
)

// pxcCRListFile is the kubectl list export name for PerconaXtraDBCluster CRs.
const pxcCRListFile = "perconaxtradbclusters.pxc.percona.com.yaml"

type helmPXCRow struct {
	Name, Namespace string
	IsHelm          bool
	ManagedBy       string
	ReleaseName     string
	Chart           string
}

type pmmRow struct {
	Name        string
	Namespace   string
	Enabled     bool
	ClientImage string
	ClientTag   string
	ServerHost  string
}

// pxcHelmPMMPairSection shows Helm install origin and PMM settings side-by-side
// for each PerconaXtraDBCluster (one read of perconaxtradbclusters YAML per file).
type pxcHelmPMMPairSection struct{}

func (pxcHelmPMMPairSection) ID() string    { return "pxc-helm-pmm" }
func (pxcHelmPMMPairSection) Title() string { return "PXC · Helm & PMM" }

func (pxcHelmPMMPairSection) Collect(ctx dumpctx.Context) (Section, error) {
	rows, err := gatherPXCHelmPMMPairs(ctx.Root())
	if err != nil {
		return Section{}, err
	}
	if len(rows) == 0 {
		return Section{}, nil
	}
	return Section{HTML: template.HTML(renderPXCHelmPMMPairs(rows))}, nil
}

type pxcHelmPMMPair struct {
	Name, Namespace string
	Helm            helmPXCRow
	PMM             pmmRow
}

type pxcUnifiedListDoc struct {
	Items []struct {
		Metadata struct {
			Name        string            `yaml:"name"`
			Namespace   string            `yaml:"namespace"`
			Labels      map[string]string `yaml:"labels"`
			Annotations map[string]string `yaml:"annotations"`
		} `yaml:"metadata"`
		Spec struct {
			PMM struct {
				Enabled    bool   `yaml:"enabled"`
				Image      string `yaml:"image"`
				ServerHost string `yaml:"serverHost"`
			} `yaml:"pmm"`
		} `yaml:"spec"`
	} `yaml:"items"`
}

func gatherPXCHelmPMMPairs(dumpRoot string) ([]pxcHelmPMMPair, error) {
	paths, err := findYAMLFiles(dumpRoot, pxcCRListFile)
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
			managedBy := strings.TrimSpace(lbl["app.kubernetes.io/managed-by"])
			rel := strings.TrimSpace(ann["meta.helm.sh/release-name"])
			chart := strings.TrimSpace(lbl["helm.sh/chart"])
			isHelm := strings.EqualFold(managedBy, "Helm") || rel != ""

			pm := list.Items[i].Spec.PMM
			img := strings.TrimSpace(pm.Image)

			out = append(out, pxcHelmPMMPair{
				Name:      name,
				Namespace: ns,
				Helm: helmPXCRow{
					Name:        name,
					Namespace:   ns,
					IsHelm:      isHelm,
					ManagedBy:   managedBy,
					ReleaseName: rel,
					Chart:       chart,
				},
				PMM: pmmRow{
					Name:        name,
					Namespace:   ns,
					Enabled:     pm.Enabled,
					ClientImage: img,
					ClientTag:   imageTagFromRef(img),
					ServerHost:  strings.TrimSpace(pm.ServerHost),
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

func renderPXCHelmPMMPairs(pairs []pxcHelmPMMPair) string {
	var b strings.Builder
	b.WriteString(`<style>
.pxc-pair-wrap { font-family: ui-sans-serif, system-ui, sans-serif; font-size: 0.78rem; color: #1e293b; }
.pxc-pair-stack { display: flex; flex-direction: column; gap: 0.65rem; }
.pxc-pair-card { border: 1px solid #e2e8f0; border-radius: 12px; padding: 0.45rem 0.55rem 0.55rem; background: linear-gradient(180deg, #fafafa, #f4f4f5); box-shadow: 0 1px 3px rgba(15,23,42,.06); }
.pxc-pair-head { display: flex; align-items: baseline; gap: 0.45rem; margin-bottom: 0.4rem; padding-bottom: 0.35rem; border-bottom: 1px solid #e2e8f0; }
.pxc-pair-name { font-family: ui-monospace, Menlo, monospace; font-weight: 700; font-size: 0.8rem; color: #0f172a; }
.pxc-pair-ns { font-size: 0.65rem; color: #64748b; text-transform: uppercase; letter-spacing: 0.06em; }
.pxc-pair-grid { display: grid; grid-template-columns: minmax(0,1fr) minmax(0,1fr); gap: 0.45rem; align-items: stretch; }
@media (max-width: 720px) { .pxc-pair-grid { grid-template-columns: 1fr; } }
.helmpxc-tile { border-radius: 10px; padding: 0.35rem 0.45rem 0.4rem; border: 1px solid #e2e8f0; background: linear-gradient(165deg, #fafafa 0%, #f1f5f9 100%); display: flex; flex-direction: column; gap: 0.2rem; min-height: 100%; }
.helmpxc-tile.helm-yes { border-color: #6ee7b7; background: linear-gradient(165deg, #ecfdf5 0%, #d1fae5 100%); }
.helmpxc-tile.helm-no { border-color: #e5e7eb; }
.helmpxc-top { display: flex; align-items: center; justify-content: space-between; gap: 0.35rem; }
.helmpxc-lbl { font-size: 0.62rem; font-weight: 700; color: #64748b; text-transform: uppercase; letter-spacing: 0.05em; }
.helmpxc-pill { font-size: 0.62rem; font-weight: 700; letter-spacing: 0.04em; padding: 0.12rem 0.4rem; border-radius: 999px; white-space: nowrap; }
.helmpxc-pill.yes { background: #059669; color: #fff; }
.helmpxc-pill.no { background: #64748b; color: #fff; }
.helmpxc-meta { font-size: 0.62rem; color: #475569; line-height: 1.35; }
.helmpxc-meta code { font-size: 0.6rem; background: rgba(255,255,255,.75); padding: 0.04rem 0.2rem; border-radius: 4px; }
.helmpxc-k { color: #94a3b8; font-weight: 500; }
.pmmviz-tile { border-radius: 10px; padding: 0.35rem 0.45rem 0.4rem; border: 1px solid #e2e8f0; background: linear-gradient(165deg, #fafafa 0%, #f8fafc 100%); display: flex; flex-direction: column; gap: 0.2rem; min-height: 100%; }
.pmmviz-tile.on { border-color: #a78bfa; background: linear-gradient(165deg, #f5f3ff 0%, #ede9fe 100%); }
.pmmviz-tile.off { border-color: #e5e7eb; }
.pmmviz-top { display: flex; align-items: center; justify-content: space-between; gap: 0.35rem; }
.pmmviz-lbl { font-size: 0.62rem; font-weight: 700; color: #64748b; text-transform: uppercase; letter-spacing: 0.05em; }
.pmmviz-pill { font-size: 0.62rem; font-weight: 700; letter-spacing: 0.04em; padding: 0.12rem 0.4rem; border-radius: 999px; white-space: nowrap; }
.pmmviz-pill.on { background: #7c3aed; color: #fff; }
.pmmviz-pill.off { background: #94a3b8; color: #fff; }
.pmmviz-row { font-size: 0.62rem; color: #475569; line-height: 1.35; display: flex; gap: 0.3rem; flex-wrap: wrap; align-items: baseline; }
.pmmviz-k { color: #94a3b8; font-weight: 500; flex: 0 0 auto; }
.pmmviz-row code { font-size: 0.6rem; background: rgba(255,255,255,.85); padding: 0.04rem 0.22rem; border-radius: 4px; color: #0f172a; }
</style>`)
	b.WriteString(`<div class="pxc-pair-wrap"><div class="pxc-pair-stack">`)
	for _, pair := range pairs {
		b.WriteString(`<div class="pxc-pair-card"><div class="pxc-pair-head"><span class="pxc-pair-name">`)
		b.WriteString(html.EscapeString(pair.Name))
		b.WriteString(`</span><span class="pxc-pair-ns">`)
		b.WriteString(html.EscapeString(pair.Namespace))
		b.WriteString(`</span></div><div class="pxc-pair-grid">`)
		writeHelmHalfTile(&b, pair.Helm)
		writePMMHalfTile(&b, pair.PMM)
		b.WriteString(`</div></div>`)
	}
	b.WriteString(`</div></div>`)
	return b.String()
}

func writeHelmHalfTile(b *strings.Builder, r helmPXCRow) {
	tileClass := "helmpxc-tile helm-no"
	pillClass := "helmpxc-pill no"
	pillText := "Not Helm"
	if r.IsHelm {
		tileClass = "helmpxc-tile helm-yes"
		pillClass = "helmpxc-pill yes"
		pillText = "Helm"
	}
	b.WriteString(`<div class="` + tileClass + `"><div class="helmpxc-top"><span class="helmpxc-lbl">Helm</span><span class="` + pillClass + `">`)
	b.WriteString(pillText)
	b.WriteString(`</span></div><div class="helmpxc-meta">`)
	mb := r.ManagedBy
	if mb == "" {
		mb = "—"
	}
	b.WriteString(`<span class="helmpxc-k">managed-by</span> <code>`)
	b.WriteString(html.EscapeString(mb))
	b.WriteString(`</code>`)
	if r.IsHelm && r.ReleaseName != "" {
		b.WriteString(` · <span class="helmpxc-k">release</span> <code>`)
		b.WriteString(html.EscapeString(r.ReleaseName))
		b.WriteString(`</code>`)
	}
	if r.IsHelm && r.Chart != "" {
		b.WriteString(` · <span class="helmpxc-k">chart</span> <code>`)
		b.WriteString(html.EscapeString(r.Chart))
		b.WriteString(`</code>`)
	}
	b.WriteString(`</div></div>`)
}

func imageTagFromRef(img string) string {
	img = strings.TrimSpace(img)
	if img == "" {
		return ""
	}
	if i := strings.LastIndex(img, "@"); i >= 0 {
		img = img[:i]
	}
	if i := strings.LastIndex(img, ":"); i >= 0 {
		return img[i+1:]
	}
	return img
}

func writePMMHalfTile(b *strings.Builder, r pmmRow) {
	tile := "pmmviz-tile off"
	pill := "pmmviz-pill off"
	pillText := "PMM off"
	if r.Enabled {
		tile = "pmmviz-tile on"
		pill = "pmmviz-pill on"
		pillText = "PMM on"
	}
	b.WriteString(`<div class="` + tile + `"><div class="pmmviz-top"><span class="pmmviz-lbl">PMM</span><span class="` + pill + `">`)
	b.WriteString(pillText)
	b.WriteString(`</span></div>`)
	tag := r.ClientTag
	if tag == "" {
		tag = "—"
	}
	host := r.ServerHost
	if host == "" {
		host = "—"
	}
	b.WriteString(`<div class="pmmviz-row"><span class="pmmviz-k">client</span> <code>`)
	b.WriteString(html.EscapeString(tag))
	b.WriteString(`</code></div>`)
	b.WriteString(`<div class="pmmviz-row"><span class="pmmviz-k">serverHost</span> <code>`)
	b.WriteString(html.EscapeString(host))
	b.WriteString(`</code></div>`)
	if r.ClientImage != "" && r.ClientImage != tag {
		b.WriteString(`<div class="pmmviz-row"><span class="pmmviz-k">image</span> <code>`)
		b.WriteString(html.EscapeString(r.ClientImage))
		b.WriteString(`</code></div>`)
	}
	b.WriteString(`</div>`)
}
