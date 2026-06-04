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

// pxcExtraPVCsSection reports spec.pxc.extraPVCs from each PerconaXtraDBCluster CR
// (same shape as Helm values key pxc.extraPVCs).
type pxcExtraPVCsSection struct{}

func (pxcExtraPVCsSection) ID() string    { return "pxc-extra-pvcs" }
func (pxcExtraPVCsSection) Title() string { return "PXC · extraPVCs" }

func (pxcExtraPVCsSection) Collect(ctx dumpctx.Context) (Section, error) {
	rows, err := gatherPXCExtraPVCRows(ctx.Root())
	if err != nil {
		return Section{}, err
	}
	if len(rows) == 0 {
		return Section{}, nil
	}
	return Section{HTML: template.HTML(renderPXCExtraPVCs(rows))}, nil
}

type pxcExtraPVCYAML struct {
	Name      string `yaml:"name"`
	ClaimName string `yaml:"claimName"`
	MountPath string `yaml:"mountPath"`
	SubPath   string `yaml:"subPath"`
	ReadOnly  bool   `yaml:"readOnly"`
}

type pxcExtraPVCClusterRow struct {
	Name, Namespace string
	Entries         []pxcExtraPVCYAML
}

type pxcExtraPVCListDoc struct {
	Items []struct {
		Metadata struct {
			Name      string `yaml:"name"`
			Namespace string `yaml:"namespace"`
		} `yaml:"metadata"`
		Spec struct {
			PXC struct {
				ExtraPVCs []pxcExtraPVCYAML `yaml:"extraPVCs"`
			} `yaml:"pxc"`
		} `yaml:"spec"`
	} `yaml:"items"`
}

func gatherPXCExtraPVCRows(dumpRoot string) ([]pxcExtraPVCClusterRow, error) {
	paths, err := findYAMLFiles(dumpRoot, pxcCRListFile)
	if err != nil {
		return nil, err
	}
	var out []pxcExtraPVCClusterRow
	for _, p := range paths {
		nsHint := filepath.Base(filepath.Dir(p))
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", p, err)
		}
		var list pxcExtraPVCListDoc
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
			entries := list.Items[i].Spec.PXC.ExtraPVCs
			if entries == nil {
				entries = []pxcExtraPVCYAML{}
			}
			out = append(out, pxcExtraPVCClusterRow{
				Name:      name,
				Namespace: ns,
				Entries:   entries,
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

func renderPXCExtraPVCs(rows []pxcExtraPVCClusterRow) string {
	var b strings.Builder
	b.WriteString(`<style>
#pxc-extra-pvcs .pxc-xpvc-note { font-size: 0.72rem; color: #64748b; margin: 0 0 0.85rem 0; line-height: 1.45; }
#pxc-extra-pvcs .pxc-xpvc-table { width: 100%; border-collapse: collapse; font-size: 0.78rem; }
#pxc-extra-pvcs .pxc-xpvc-table th { text-align: left; padding: 0.4rem 0.55rem; background: #f1f5f9; border: 1px solid #e2e8f0; font-weight: 650; color: #334155; white-space: nowrap; }
#pxc-extra-pvcs .pxc-xpvc-table td { padding: 0.4rem 0.55rem; border: 1px solid #e2e8f0; vertical-align: middle; }
#pxc-extra-pvcs .pxc-xpvc-table td.pxc-xpvc-cr { font-family: ui-monospace, Menlo, monospace; font-weight: 700; font-size: 0.76rem; color: #0f172a; }
#pxc-extra-pvcs .pxc-xpvc-table td.pxc-xpvc-ns { font-size: 0.65rem; color: #64748b; text-transform: uppercase; letter-spacing: 0.05em; }
#pxc-extra-pvcs .pxc-xpvc-table td.pxc-xpvc-mono { font-family: ui-monospace, Menlo, monospace; font-size: 0.72rem; word-break: break-word; }
#pxc-extra-pvcs .pxc-xpvc-table td.pxc-xpvc-ro { font-family: ui-sans-serif, system-ui, sans-serif; font-weight: 600; white-space: nowrap; }
#pxc-extra-pvcs .pxc-xpvc-table td.pxc-xpvc-none { color: #64748b; font-size: 0.76rem; font-style: italic; }
</style>`)
	b.WriteString(`<p class="pxc-xpvc-note">External PVC mounts from <code>spec.pxc.extraPVCs</code> in each <code>PerconaXtraDBCluster</code> CR (Helm: <code>pxc.extraPVCs</code>). The operator mounts existing claims in PXC pods; it does not create these PVCs.</p>`)
	b.WriteString(`<table class="pxc-xpvc-table"><thead><tr>`)
	b.WriteString(`<th scope="col">CR name</th><th scope="col">Namespace</th><th scope="col">name</th><th scope="col">claimName</th><th scope="col">mountPath</th><th scope="col">subPath</th><th scope="col">readOnly</th>`)
	b.WriteString(`</tr></thead><tbody>`)

	for _, row := range rows {
		escName := html.EscapeString(row.Name)
		escNS := html.EscapeString(row.Namespace)
		if len(row.Entries) == 0 {
			b.WriteString(`<tr><td class="pxc-xpvc-cr">`)
			b.WriteString(escName)
			b.WriteString(`</td><td class="pxc-xpvc-ns">`)
			b.WriteString(escNS)
			b.WriteString(`</td><td colspan="5" class="pxc-xpvc-none">No <code>extraPVCs</code> defined under <code>spec.pxc</code>.</td></tr>`)
			continue
		}
		for _, e := range row.Entries {
			b.WriteString(`<tr><td class="pxc-xpvc-cr">`)
			b.WriteString(escName)
			b.WriteString(`</td><td class="pxc-xpvc-ns">`)
			b.WriteString(escNS)
			b.WriteString(`</td><td class="pxc-xpvc-mono">`)
			b.WriteString(html.EscapeString(strings.TrimSpace(e.Name)))
			b.WriteString(`</td><td class="pxc-xpvc-mono">`)
			b.WriteString(html.EscapeString(strings.TrimSpace(e.ClaimName)))
			b.WriteString(`</td><td class="pxc-xpvc-mono">`)
			b.WriteString(html.EscapeString(strings.TrimSpace(e.MountPath)))
			b.WriteString(`</td><td class="pxc-xpvc-mono">`)
			sub := strings.TrimSpace(e.SubPath)
			if sub == "" {
				b.WriteString(`—`)
			} else {
				b.WriteString(html.EscapeString(sub))
			}
			b.WriteString(`</td><td class="pxc-xpvc-ro">`)
			if e.ReadOnly {
				b.WriteString(`true`)
			} else {
				b.WriteString(`false`)
			}
			b.WriteString(`</td></tr>`)
		}
	}
	b.WriteString(`</tbody></table>`)
	return b.String()
}
