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

// pxcUpdateStrategySection reports how PXC upgrades are driven (SmartUpdate vs RollingUpdate, etc.).
// In cluster dumps this is usually spec.updateStrategy; some charts document it as pxc.updateStrategy
// in values.yaml, which maps into the CR.
type pxcUpdateStrategySection struct{}

func (pxcUpdateStrategySection) ID() string    { return "pxc-update-strategy" }
func (pxcUpdateStrategySection) Title() string { return "PXC · updateStrategy" }

func (pxcUpdateStrategySection) Collect(ctx dumpctx.Context) (Section, error) {
	rows, err := gatherPXCUpdateStrategyRows(ctx.Root())
	if err != nil {
		return Section{}, err
	}
	if len(rows) == 0 {
		return Section{}, nil
	}
	return Section{HTML: template.HTML(renderPXCUpdateStrategy(rows))}, nil
}

type pxcUpdateStrategyRow struct {
	Name, Namespace string
	Value            string // empty if not set
	SourceField      string // spec.updateStrategy or spec.pxc.updateStrategy or ""
}

type pxcUpdateStrategyListDoc struct {
	Items []struct {
		Metadata struct {
			Name      string `yaml:"name"`
			Namespace string `yaml:"namespace"`
		} `yaml:"metadata"`
		Spec struct {
			UpdateStrategy string `yaml:"updateStrategy"`
			PXC            struct {
				UpdateStrategy string `yaml:"updateStrategy"`
			} `yaml:"pxc"`
		} `yaml:"spec"`
	} `yaml:"items"`
}

func gatherPXCUpdateStrategyRows(dumpRoot string) ([]pxcUpdateStrategyRow, error) {
	paths, err := findYAMLFiles(dumpRoot, pxcCRListFile)
	if err != nil {
		return nil, err
	}
	var out []pxcUpdateStrategyRow
	for _, p := range paths {
		nsHint := filepath.Base(filepath.Dir(p))
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", p, err)
		}
		var list pxcUpdateStrategyListDoc
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
			spec := list.Items[i].Spec
			root := strings.TrimSpace(spec.UpdateStrategy)
			nested := strings.TrimSpace(spec.PXC.UpdateStrategy)
			var val, src string
			if root != "" {
				val, src = root, "spec.updateStrategy"
			} else if nested != "" {
				val, src = nested, "spec.pxc.updateStrategy"
			}
			out = append(out, pxcUpdateStrategyRow{
				Name:        name,
				Namespace:   ns,
				Value:       val,
				SourceField: src,
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

func updateStrategyHint(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "smartupdate":
		return "Operator-controlled rollout (ordered pods, version service, backup-aware)."
	case "rollingupdate":
		return "Kubernetes-style rolling update of StatefulSet pods."
	case "ondelete":
		return "Pods recreated only when deleted manually."
	default:
		if strings.TrimSpace(v) == "" {
			return "—"
		}
		return "See operator docs for this value."
	}
}

func renderPXCUpdateStrategy(rows []pxcUpdateStrategyRow) string {
	var b strings.Builder
	b.WriteString(`<style>
#pxc-update-strategy .pxc-us-note { font-size: 0.72rem; color: #64748b; margin: 0 0 0.75rem 0; line-height: 1.45; }
#pxc-update-strategy .pxc-us-table { width: 100%; border-collapse: collapse; font-size: 0.78rem; }
#pxc-update-strategy .pxc-us-table th { text-align: left; padding: 0.4rem 0.55rem; background: #f1f5f9; border: 1px solid #e2e8f0; font-weight: 650; color: #334155; }
#pxc-update-strategy .pxc-us-table td { padding: 0.4rem 0.55rem; border: 1px solid #e2e8f0; vertical-align: top; }
#pxc-update-strategy .pxc-us-table td.pxc-us-cr { font-family: ui-monospace, Menlo, monospace; font-weight: 700; font-size: 0.76rem; color: #0f172a; }
#pxc-update-strategy .pxc-us-table td.pxc-us-ns { font-size: 0.65rem; color: #64748b; text-transform: uppercase; letter-spacing: 0.05em; }
#pxc-update-strategy .pxc-us-table td.pxc-us-val { font-family: ui-monospace, Menlo, monospace; font-size: 0.74rem; word-break: break-word; }
#pxc-update-strategy .pxc-us-table td.pxc-us-src { font-size: 0.68rem; color: #64748b; font-family: ui-monospace, Menlo, monospace; }
#pxc-update-strategy .pxc-us-table td.pxc-us-hint { font-size: 0.7rem; color: #475569; line-height: 1.4; }
#pxc-update-strategy .pxc-us-unset { color: #94a3b8; font-style: italic; font-family: ui-sans-serif, system-ui, sans-serif; }
</style>`)
	b.WriteString(`<p class="pxc-us-note"><strong>updateStrategy</strong> controls how the operator applies image and spec changes to PXC pods. In the CR it is usually <code>spec.updateStrategy</code> (Helm values often use <code>updateStrategy</code> or <code>pxc.updateStrategy</code> depending on the chart). Values such as <code>SmartUpdate</code>, <code>RollingUpdate</code>, and <code>OnDelete</code> change upgrade behaviour.</p>`)
	b.WriteString(`<table class="pxc-us-table"><thead><tr>`)
	b.WriteString(`<th scope="col">CR name</th><th scope="col">Namespace</th><th scope="col">updateStrategy</th><th scope="col">YAML path</th><th scope="col">Effect (summary)</th>`)
	b.WriteString(`</tr></thead><tbody>`)
	for _, r := range rows {
		b.WriteString(`<tr><td class="pxc-us-cr">`)
		b.WriteString(html.EscapeString(r.Name))
		b.WriteString(`</td><td class="pxc-us-ns">`)
		b.WriteString(html.EscapeString(r.Namespace))
		b.WriteString(`</td><td class="pxc-us-val">`)
		if r.Value == "" {
			b.WriteString(`<span class="pxc-us-unset">not set</span>`)
		} else {
			b.WriteString(html.EscapeString(r.Value))
		}
		b.WriteString(`</td><td class="pxc-us-src">`)
		if r.SourceField == "" {
			b.WriteString(`<span class="pxc-us-unset">—</span>`)
		} else {
			b.WriteString(html.EscapeString(r.SourceField))
		}
		b.WriteString(`</td><td class="pxc-us-hint">`)
		b.WriteString(html.EscapeString(updateStrategyHint(r.Value)))
		b.WriteString(`</td></tr>`)
	}
	b.WriteString(`</tbody></table>`)
	return b.String()
}
