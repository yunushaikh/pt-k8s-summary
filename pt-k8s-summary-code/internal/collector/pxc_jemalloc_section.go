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

// pxcJemallocSection reports spec.pxc.mysqlAllocator for each PXC CR (Helm: pxc.mysqlAllocator).
// The section is shown whenever any PerconaXtraDBCluster exists in the dump, with enabled / not enabled per row.
type pxcJemallocSection struct{}

func (pxcJemallocSection) ID() string    { return "pxc-jemalloc" }
func (pxcJemallocSection) Title() string { return "PXC · jemalloc" }

func (pxcJemallocSection) Collect(ctx dumpctx.Context) (Section, error) {
	rows, err := gatherPXCJemallocRows(ctx.Root())
	if err != nil {
		return Section{}, err
	}
	if len(rows) == 0 {
		return Section{}, nil
	}
	return Section{HTML: template.HTML(renderPXCJemalloc(rows))}, nil
}

type pxcJemallocRow struct {
	Name, Namespace string
	MySQLAllocator   string // trimmed from CR; empty if unset
}

type pxcJemallocListDoc struct {
	Items []struct {
		Metadata struct {
			Name      string `yaml:"name"`
			Namespace string `yaml:"namespace"`
		} `yaml:"metadata"`
		Spec struct {
			PXC struct {
				MySQLAllocator string `yaml:"mysqlAllocator"`
			} `yaml:"pxc"`
		} `yaml:"spec"`
	} `yaml:"items"`
}

func gatherPXCJemallocRows(dumpRoot string) ([]pxcJemallocRow, error) {
	paths, err := findYAMLFiles(dumpRoot, pxcCRListFile)
	if err != nil {
		return nil, err
	}
	var out []pxcJemallocRow
	for _, p := range paths {
		nsHint := filepath.Base(filepath.Dir(p))
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", p, err)
		}
		var list pxcJemallocListDoc
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
			raw := strings.TrimSpace(list.Items[i].Spec.PXC.MySQLAllocator)
			out = append(out, pxcJemallocRow{
				Name:           name,
				Namespace:      ns,
				MySQLAllocator: raw,
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

func jemallocEnabled(raw string) bool {
	return strings.EqualFold(strings.TrimSpace(raw), "jemalloc")
}

func renderPXCJemalloc(rows []pxcJemallocRow) string {
	var b strings.Builder
	b.WriteString(`<style>
#pxc-jemalloc .pxc-jem-note { font-size: 0.72rem; color: #64748b; margin: 0 0 0.75rem 0; line-height: 1.45; }
#pxc-jemalloc .pxc-jem-table { width: 100%; border-collapse: collapse; font-size: 0.78rem; }
#pxc-jemalloc .pxc-jem-table th { text-align: left; padding: 0.4rem 0.55rem; background: #f1f5f9; border: 1px solid #e2e8f0; font-weight: 650; color: #334155; }
#pxc-jemalloc .pxc-jem-table td { padding: 0.4rem 0.55rem; border: 1px solid #e2e8f0; vertical-align: middle; }
#pxc-jemalloc .pxc-jem-table td.pxc-jem-cr { font-family: ui-monospace, Menlo, monospace; font-weight: 700; font-size: 0.76rem; color: #0f172a; }
#pxc-jemalloc .pxc-jem-table td.pxc-jem-ns { font-size: 0.65rem; color: #64748b; text-transform: uppercase; letter-spacing: 0.05em; }
#pxc-jemalloc .pxc-jem-table td.pxc-jem-val { font-family: ui-monospace, Menlo, monospace; font-size: 0.72rem; word-break: break-word; }
#pxc-jemalloc .pxc-jem-pill { display: inline-block; font-size: 0.72rem; font-weight: 800; letter-spacing: 0.03em; padding: 0.25rem 0.5rem; border-radius: 8px; }
#pxc-jemalloc .pxc-jem-pill.on { color: #166534; background: linear-gradient(180deg,#f0fdf4,#dcfce7); border: 1px solid #bbf7d0; }
#pxc-jemalloc .pxc-jem-pill.off { color: #64748b; background: #f8fafc; border: 1px solid #e2e8f0; font-weight: 700; }
#pxc-jemalloc .pxc-jem-pill.other { color: #9a3412; background: linear-gradient(180deg,#fff7ed,#ffedd5); border: 1px solid #fdba74; }
#pxc-jemalloc .pxc-jem-unset { color: #94a3b8; font-style: italic; font-family: ui-sans-serif, system-ui, sans-serif; }
</style>`)
	b.WriteString(`<p class="pxc-jem-note"><code>spec.pxc.mysqlAllocator</code> selects the memory allocator for MySQL in PXC pods (Helm: <code>pxc.mysqlAllocator</code>). When set to <code>jemalloc</code>, jemalloc is enabled. If omitted, the image default allocator applies.</p>`)
	b.WriteString(`<table class="pxc-jem-table"><thead><tr>`)
	b.WriteString(`<th scope="col">CR name</th><th scope="col">Namespace</th><th scope="col">mysqlAllocator (CR)</th><th scope="col">jemalloc</th>`)
	b.WriteString(`</tr></thead><tbody>`)
	for _, r := range rows {
		raw := strings.TrimSpace(r.MySQLAllocator)
		b.WriteString(`<tr><td class="pxc-jem-cr">`)
		b.WriteString(html.EscapeString(r.Name))
		b.WriteString(`</td><td class="pxc-jem-ns">`)
		b.WriteString(html.EscapeString(r.Namespace))
		b.WriteString(`</td><td class="pxc-jem-val">`)
		if raw == "" {
			b.WriteString(`<span class="pxc-jem-unset">not set</span>`)
		} else {
			b.WriteString(html.EscapeString(raw))
		}
		b.WriteString(`</td><td>`)
		switch {
		case jemallocEnabled(raw):
			b.WriteString(`<span class="pxc-jem-pill on">enabled</span>`)
		case raw == "":
			b.WriteString(`<span class="pxc-jem-pill off">not enabled</span>`)
		default:
			b.WriteString(`<span class="pxc-jem-pill other">not enabled</span>`)
		}
		b.WriteString(`</td></tr>`)
	}
	b.WriteString(`</tbody></table>`)
	return b.String()
}
