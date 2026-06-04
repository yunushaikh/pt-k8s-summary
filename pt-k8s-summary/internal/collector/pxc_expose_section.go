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

// pxcExposeSection reports spec.pxc.expose (Helm: pxc.expose): whether PXC is exposed
// and whether type is LoadBalancer.
type pxcExposeSection struct{}

func (pxcExposeSection) ID() string    { return "pxc-expose" }
func (pxcExposeSection) Title() string { return "PXC · expose" }

func (pxcExposeSection) Collect(ctx dumpctx.Context) (Section, error) {
	rows, err := gatherPXCExposeRows(ctx.Root())
	if err != nil {
		return Section{}, err
	}
	if len(rows) == 0 {
		return Section{}, nil
	}
	return Section{HTML: template.HTML(renderPXCExpose(rows))}, nil
}

type pxcExposeYAML struct {
	Enabled                  *bool             `yaml:"enabled"`
	Type                     string            `yaml:"type"`
	LoadBalancerClass        string            `yaml:"loadBalancerClass"`
	ExternalTrafficPolicy    string            `yaml:"externalTrafficPolicy"`
	InternalTrafficPolicy    string            `yaml:"internalTrafficPolicy"`
	LoadBalancerSourceRanges []string          `yaml:"loadBalancerSourceRanges"`
	Annotations              map[string]string `yaml:"annotations"`
	Labels                   map[string]string `yaml:"labels"`
}

type pxcExposeRow struct {
	Name, Namespace string
	HasExposeBlock    bool
	EnabledDisplay    string
	EnabledTrue       bool // *Enabled == true
	Type              string
	LoadBalancerClass string
	ExtPolicy         string
	IntPolicy         string
	RangesDisplay     string
	AnnotationsDisp   string
	LabelsDisp        string
}

type pxcExposeListDoc struct {
	Items []struct {
		Metadata struct {
			Name      string `yaml:"name"`
			Namespace string `yaml:"namespace"`
		} `yaml:"metadata"`
		Spec struct {
			PXC struct {
				Expose *pxcExposeYAML `yaml:"expose"`
			} `yaml:"pxc"`
		} `yaml:"spec"`
	} `yaml:"items"`
}

func gatherPXCExposeRows(dumpRoot string) ([]pxcExposeRow, error) {
	paths, err := findYAMLFiles(dumpRoot, pxcCRListFile)
	if err != nil {
		return nil, err
	}
	var out []pxcExposeRow
	for _, p := range paths {
		nsHint := filepath.Base(filepath.Dir(p))
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", p, err)
		}
		var list pxcExposeListDoc
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
			ex := list.Items[i].Spec.PXC.Expose
			row := pxcExposeRow{Name: name, Namespace: ns, HasExposeBlock: ex != nil}
			if ex == nil {
				row.EnabledDisplay = "—"
				out = append(out, row)
				continue
			}
			switch {
			case ex.Enabled == nil:
				row.EnabledDisplay = "not set"
			case *ex.Enabled:
				row.EnabledDisplay = "true"
				row.EnabledTrue = true
			default:
				row.EnabledDisplay = "false"
			}
			row.Type = strings.TrimSpace(ex.Type)
			row.LoadBalancerClass = strings.TrimSpace(ex.LoadBalancerClass)
			row.ExtPolicy = strings.TrimSpace(ex.ExternalTrafficPolicy)
			row.IntPolicy = strings.TrimSpace(ex.InternalTrafficPolicy)
			var ranges []string
			for _, r := range ex.LoadBalancerSourceRanges {
				s := strings.TrimSpace(r)
				if s != "" {
					ranges = append(ranges, s)
				}
			}
			row.RangesDisplay = strings.Join(ranges, ", ")
			row.AnnotationsDisp = formatExposeMap(ex.Annotations)
			row.LabelsDisp = formatExposeMap(ex.Labels)
			out = append(out, row)
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

func formatExposeMap(m map[string]string) string {
	if len(m) == 0 {
		return ""
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		parts = append(parts, k+"="+m[k])
	}
	return strings.Join(parts, "; ")
}

func truncateRunesDisplay(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + "…"
}

func isLoadBalancerType(t string) bool {
	return strings.EqualFold(strings.TrimSpace(t), "LoadBalancer")
}

func renderPXCExpose(rows []pxcExposeRow) string {
	var b strings.Builder
	b.WriteString(`<style>
#pxc-expose .pxc-exp-note { font-size: 0.72rem; color: #64748b; margin: 0 0 0.75rem 0; line-height: 1.45; }
#pxc-expose .pxc-exp-table { width: 100%; border-collapse: collapse; font-size: 0.72rem; table-layout: fixed; }
#pxc-expose .pxc-exp-table th { text-align: left; padding: 0.35rem 0.45rem; background: #f1f5f9; border: 1px solid #e2e8f0; font-weight: 650; color: #334155; vertical-align: bottom; word-break: break-word; }
#pxc-expose .pxc-exp-table td { padding: 0.35rem 0.45rem; border: 1px solid #e2e8f0; vertical-align: top; word-break: break-word; }
#pxc-expose .pxc-exp-table td.pxc-exp-cr { font-family: ui-monospace, Menlo, monospace; font-weight: 700; font-size: 0.74rem; color: #0f172a; }
#pxc-expose .pxc-exp-table td.pxc-exp-ns { font-size: 0.62rem; color: #64748b; text-transform: uppercase; letter-spacing: 0.05em; }
#pxc-expose .pxc-exp-table td.pxc-exp-mono { font-family: ui-monospace, Menlo, monospace; font-size: 0.68rem; }
#pxc-expose .pxc-exp-pill { display: inline-block; font-size: 0.68rem; font-weight: 800; letter-spacing: 0.02em; padding: 0.2rem 0.45rem; border-radius: 8px; white-space: nowrap; }
#pxc-expose .pxc-exp-pill.yes { color: #166534; background: linear-gradient(180deg,#f0fdf4,#dcfce7); border: 1px solid #bbf7d0; }
#pxc-expose .pxc-exp-pill.no { color: #64748b; background: #f8fafc; border: 1px solid #e2e8f0; }
#pxc-expose .pxc-exp-pill.warn { color: #9a3412; background: linear-gradient(180deg,#fff7ed,#ffedd5); border: 1px solid #fdba74; }
#pxc-expose .pxc-exp-pill.na { color: #94a3b8; background: #f1f5f9; border: 1px solid #e2e8f0; font-weight: 600; }
#pxc-expose .pxc-exp-unset { color: #94a3b8; font-style: italic; font-family: ui-sans-serif, system-ui, sans-serif; }
</style>`)
	b.WriteString(`<p class="pxc-exp-note"><code>spec.pxc.expose</code> controls a Service that exposes PXC pods (Helm: <code>pxc.expose</code>). When <code>enabled: true</code> and <code>type: LoadBalancer</code>, a cloud load balancer is typically provisioned. <code>loadBalancerClass</code>, traffic policies, source ranges, and service metadata are shown below when present.</p>`)
	b.WriteString(`<table class="pxc-exp-table"><thead><tr>`)
	b.WriteString(`<th scope="col">CR name</th><th scope="col">Namespace</th><th scope="col">expose in CR</th><th scope="col">enabled</th><th scope="col">type</th><th scope="col">LoadBalancer</th>`)
	b.WriteString(`<th scope="col">loadBalancerClass</th><th scope="col">externalTrafficPolicy</th><th scope="col">internalTrafficPolicy</th>`)
	b.WriteString(`<th scope="col">loadBalancerSourceRanges</th><th scope="col">annotations</th><th scope="col">labels</th>`)
	b.WriteString(`</tr></thead><tbody>`)

	for _, r := range rows {
		b.WriteString(`<tr><td class="pxc-exp-cr">`)
		b.WriteString(html.EscapeString(r.Name))
		b.WriteString(`</td><td class="pxc-exp-ns">`)
		b.WriteString(html.EscapeString(r.Namespace))
		b.WriteString(`</td><td>`)
		if r.HasExposeBlock {
			b.WriteString(`<span class="pxc-exp-pill yes">yes</span>`)
		} else {
			b.WriteString(`<span class="pxc-exp-pill no">no</span>`)
		}
		b.WriteString(`</td><td>`)
		if !r.HasExposeBlock {
			b.WriteString(`<span class="pxc-exp-unset">—</span>`)
		} else if r.EnabledDisplay == "not set" {
			b.WriteString(`<span class="pxc-exp-unset">not set</span>`)
		} else {
			b.WriteString(html.EscapeString(r.EnabledDisplay))
		}
		b.WriteString(`</td><td class="pxc-exp-mono">`)
		if !r.HasExposeBlock || r.Type == "" {
			b.WriteString(`<span class="pxc-exp-unset">—</span>`)
		} else {
			b.WriteString(html.EscapeString(r.Type))
		}
		b.WriteString(`</td><td>`)
		b.WriteString(lbPillHTML(r))
		b.WriteString(`</td><td class="pxc-exp-mono">`)
		if r.LoadBalancerClass == "" {
			b.WriteString(`<span class="pxc-exp-unset">—</span>`)
		} else {
			b.WriteString(html.EscapeString(truncateRunesDisplay(r.LoadBalancerClass, 48)))
		}
		b.WriteString(`</td><td class="pxc-exp-mono">`)
		if r.ExtPolicy == "" {
			b.WriteString(`<span class="pxc-exp-unset">—</span>`)
		} else {
			b.WriteString(html.EscapeString(r.ExtPolicy))
		}
		b.WriteString(`</td><td class="pxc-exp-mono">`)
		if r.IntPolicy == "" {
			b.WriteString(`<span class="pxc-exp-unset">—</span>`)
		} else {
			b.WriteString(html.EscapeString(r.IntPolicy))
		}
		b.WriteString(`</td><td class="pxc-exp-mono">`)
		if r.RangesDisplay == "" {
			b.WriteString(`<span class="pxc-exp-unset">—</span>`)
		} else {
			b.WriteString(html.EscapeString(truncateRunesDisplay(r.RangesDisplay, 64)))
		}
		b.WriteString(`</td><td class="pxc-exp-mono">`)
		if r.AnnotationsDisp == "" {
			b.WriteString(`<span class="pxc-exp-unset">—</span>`)
		} else {
			b.WriteString(html.EscapeString(truncateRunesDisplay(r.AnnotationsDisp, 56)))
		}
		b.WriteString(`</td><td class="pxc-exp-mono">`)
		if r.LabelsDisp == "" {
			b.WriteString(`<span class="pxc-exp-unset">—</span>`)
		} else {
			b.WriteString(html.EscapeString(truncateRunesDisplay(r.LabelsDisp, 56)))
		}
		b.WriteString(`</td></tr>`)
	}
	b.WriteString(`</tbody></table>`)
	return b.String()
}

func lbPillHTML(r pxcExposeRow) string {
	if !r.HasExposeBlock {
		return `<span class="pxc-exp-pill na">n/a</span>`
	}
	if !r.EnabledTrue {
		return `<span class="pxc-exp-pill no">no</span>`
	}
	if isLoadBalancerType(r.Type) {
		return `<span class="pxc-exp-pill yes">yes</span>`
	}
	if strings.TrimSpace(r.Type) == "" {
		return `<span class="pxc-exp-pill warn">no (type unset)</span>`
	}
	return fmt.Sprintf(`<span class="pxc-exp-pill warn">no (%s)</span>`, html.EscapeString(r.Type))
}
