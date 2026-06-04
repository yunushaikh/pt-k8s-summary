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

// pxcReplicationChannelsSection reports spec.pxc.replicationChannels (cross-site /
// external async replication). When isSource is false, sourcesList defines upstream
// source hosts/ports/weights the replica should use.
type pxcReplicationChannelsSection struct{}

func (pxcReplicationChannelsSection) ID() string    { return "pxc-repl-channels" }
func (pxcReplicationChannelsSection) Title() string { return "PXC · replication channels" }

func (pxcReplicationChannelsSection) Collect(ctx dumpctx.Context) (Section, error) {
	rows, err := gatherPXCReplicationRows(ctx.Root())
	if err != nil {
		return Section{}, err
	}
	if len(rows) == 0 {
		return Section{}, nil
	}
	return Section{HTML: template.HTML(renderPXCReplicationHTML(rows))}, nil
}

type pxcRepSource struct {
	Host   string `yaml:"host"`
	Port   *int   `yaml:"port"`
	Weight *int   `yaml:"weight"`
}

type pxcRepChannel struct {
	Name        string   `yaml:"name"`
	IsSource    *bool    `yaml:"isSource"`
	SourcesList []pxcRepSource `yaml:"sourcesList"`
}

type pxcRepCluster struct {
	Name, Namespace string
	Channels        []pxcRepChannel
}

type pxcRepListDoc struct {
	Items []struct {
		Metadata struct {
			Name      string `yaml:"name"`
			Namespace string `yaml:"namespace"`
		} `yaml:"metadata"`
		Spec struct {
			PXC struct {
				ReplicationChannels []pxcRepChannel `yaml:"replicationChannels"`
			} `yaml:"pxc"`
		} `yaml:"spec"`
	} `yaml:"items"`
}

func gatherPXCReplicationRows(dumpRoot string) ([]pxcRepCluster, error) {
	paths, err := findYAMLFiles(dumpRoot, pxcCRListFile)
	if err != nil {
		return nil, err
	}
	var out []pxcRepCluster
	for _, p := range paths {
		nsHint := filepath.Base(filepath.Dir(p))
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", p, err)
		}
		var list pxcRepListDoc
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
			ch := list.Items[i].Spec.PXC.ReplicationChannels
			if ch == nil {
				ch = []pxcRepChannel{}
			}
			out = append(out, pxcRepCluster{
				Name:        name,
				Namespace:   ns,
				Channels:    ch,
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

func isSourceString(b *bool) string {
	if b == nil {
		return "not set"
	}
	if *b {
		return "true (this site is the source)"
	}
	return "false (this site is a replica; uses sourcesList when set)"
}

// formatOneSourceText returns a plain line for a single sourcesList entry
// (escaped later when embedded in HTML).
func formatOneSourceText(s pxcRepSource) string {
	h := strings.TrimSpace(s.Host)
	if h == "" {
		h = "(host missing in CR)"
	}
	port := 3306
	portNote := ""
	if s.Port != nil {
		port = *s.Port
	} else {
		portNote = "; port omitted in CR (default 3306 in operator)"
	}
	wt := "weight not set in CR"
	if s.Weight != nil {
		wt = fmt.Sprintf("weight %d", *s.Weight)
	}
	return fmt.Sprintf("%s:%d — %s%s", h, port, wt, portNote)
}

func clusterReplicaWithSources(c pxcRepCluster) bool {
	for _, ch := range c.Channels {
		if ch.IsSource != nil && !*ch.IsSource && len(ch.SourcesList) > 0 {
			return true
		}
	}
	return false
}

func clusterHasReplicaChannel(c pxcRepCluster) bool {
	for _, ch := range c.Channels {
		if ch.IsSource != nil && !*ch.IsSource {
			return true
		}
	}
	return false
}

func renderPXCReplicationHTML(rows []pxcRepCluster) string {
	var b strings.Builder
	b.WriteString(`<style>
#pxc-repl-channels .pxc-repl-note { font-size: 0.72rem; color: #64748b; margin: 0 0 0.75rem 0; line-height: 1.45; }
#pxc-repl-channels .pxc-repl-card { border: 1px solid #e2e8f0; border-radius: 10px; padding: 0.65rem 0.85rem; margin: 0.75rem 0; background: #fafafa; }
#pxc-repl-channels .pxc-repl-card h4 { margin: 0 0 0.4rem 0; font-size: 0.95rem; }
#pxc-repl-channels .pxc-repl-card h4 .pxc-repl-ns { font-size: 0.65rem; color: #64748b; font-weight: 500; text-transform: uppercase; letter-spacing: 0.04em; margin-left: 0.35rem; }
#pxc-repl-channels .pxc-repl-summary { font-size: 0.78rem; margin: 0.25rem 0 0.5rem; }
#pxc-repl-channels .pxc-repl-ok { color: #166534; }
#pxc-repl-channels .pxc-repl-warn { color: #9a3412; }
#pxc-repl-channels .pxc-repl-mute { color: #64748b; }
#pxc-repl-channels .pxc-repl-chan { margin: 0.5rem 0 0.25rem; padding-left: 1.1rem; }
#pxc-repl-channels .pxc-repl-chan > li { margin: 0.5rem 0; }
#pxc-repl-channels .pxc-repl-chan code { font-size: 0.8rem; }
#pxc-repl-channels .pxc-repl-chan .pxc-repl-meta { display: block; font-size: 0.72rem; color: #475569; margin: 0.2rem 0 0.15rem; }
#pxc-repl-channels .pxc-repl-sl { margin: 0.2rem 0 0.15rem; padding-left: 1.2rem; }
#pxc-repl-channels .pxc-repl-sl li { font-family: ui-monospace, Menlo, monospace; font-size: 0.7rem; margin: 0.2rem 0; }
#pxc-repl-channels .pxc-repl-empty { font-size: 0.75rem; color: #64748b; font-style: italic; margin: 0.15rem 0; }
</style>`)
	b.WriteString(`<p class="pxc-repl-note">From <code>spec.pxc.replicationChannels</code> (Percona cross-site / external replication). A channel with <code>isSource: false</code> indicates this cluster is a <strong>replica</strong> for that channel; <code>sourcesList</code> names upstream host(s), port(s), and weight(s). Source-side channels typically use <code>isSource: true</code> and omit <code>sourcesList</code>.</p>`)

	for _, c := range rows {
		b.WriteString(`<div class="pxc-repl-card">`)
		b.WriteString(`<h4><code>`)
		b.WriteString(html.EscapeString(c.Name))
		b.WriteString(`</code><span class="pxc-repl-ns">`)
		b.WriteString(html.EscapeString(c.Namespace))
		b.WriteString(`</span></h4>`)
		if len(c.Channels) == 0 {
			b.WriteString(`<p class="pxc-repl-empty">No <code>replicationChannels</code> block in this custom resource (or the list is empty).</p></div>`)
			continue
		}
		switch {
		case clusterReplicaWithSources(c):
			b.WriteString(`<p class="pxc-repl-summary pxc-repl-ok">At least one channel is a <strong>replica</strong> with a non-empty <code>sourcesList</code> (this site is configured to pull from the listed upstream address(es)).</p>`)
		case clusterHasReplicaChannel(c):
			b.WriteString(`<p class="pxc-repl-summary pxc-repl-warn">A channel is marked <code>isSource: false</code> (replica), but <code>sourcesList</code> is empty in the CR for that channel—upstream source host(s) are not defined here (fix if this site should replicate from an external master).</p>`)
		default:
			b.WriteString(`<p class="pxc-repl-summary pxc-repl-mute">This CR has no channel with <code>isSource: false</code> and a non-empty <code>sourcesList</code> (single-site PXC, or this cluster is only a source, or off-site details are in another file).</p>`)
		}
		b.WriteString(`<ol class="pxc-repl-chan">`)
		for _, ch := range c.Channels {
			chanName := strings.TrimSpace(ch.Name)
			if chanName == "" {
				chanName = "(unnamed channel)"
			}
			b.WriteString(`<li><code>`)
			b.WriteString(html.EscapeString(chanName))
			b.WriteString(`</code>`)
			b.WriteString(`<span class="pxc-repl-meta">`)
			b.WriteString(html.EscapeString("isSource: " + isSourceString(ch.IsSource)))
			b.WriteString(`</span>`)
			if len(ch.SourcesList) == 0 {
				b.WriteString(`<p class="pxc-repl-empty">No <code>sourcesList</code> entries in the CR for this channel.</p>`)
			} else {
				b.WriteString(`<p class="pxc-repl-meta" style="margin-top:0.15rem">sourcesList:</p><ul class="pxc-repl-sl">`)
				for _, src := range ch.SourcesList {
					b.WriteString(`<li>`)
					b.WriteString(html.EscapeString(formatOneSourceText(src)))
					b.WriteString(`</li>`)
				}
				b.WriteString(`</ul>`)
			}
			b.WriteString(`</li>`)
		}
		b.WriteString(`</ol></div>`)
	}
	return b.String()
}