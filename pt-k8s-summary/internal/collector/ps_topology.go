package collector

import (
	"fmt"
	"html"
	"strings"
	"time"

	"pt-k8s-summary/internal/jpreport"
)

func renderPSTopologyHTML(dumpRoot string, pods *jpreport.PodLoader, k8s map[string]jpreport.PodK8sMeta) (string, error) {
	clusters, err := loadPSClusters(dumpRoot)
	if err != nil {
		return "", err
	}
	if len(clusters) == 0 {
		return "", nil
	}
	if pods == nil {
		pods, err = jpreport.LoadPodLoader(dumpRoot)
		if err != nil {
			pods = &jpreport.PodLoader{}
		}
	}
	if k8s == nil && pods != nil {
		k8s = pods.K8sMetaByPod(dumpRoot, time.Now())
	}
	esc := html.EscapeString
	var b strings.Builder
	b.WriteString(`<style>
.ps-topo-wrap { margin: 0 0 1rem 0; }
.ps-topo-cluster { margin: 0 0 1.1rem 0; padding: 0.85rem 1rem; border: 1px solid #e2e8f0; border-radius: 12px; background: linear-gradient(165deg,#f8fafc 0%,#fff 55%); }
.ps-topo-h { margin: 0 0 0.35rem 0; font-size: 0.92rem; font-weight: 700; color: #0f172a; font-family: ui-monospace, Menlo, monospace; }
.ps-topo-meta { margin: 0 0 0.75rem 0; font-size: 0.72rem; color: #64748b; line-height: 1.45; }
.ps-topo-meta code { font-size: 0.68rem; }
.ps-topo-flow { display: flex; flex-direction: column; align-items: stretch; gap: 0.45rem; }
.ps-topo-layer { border: 1px solid #cbd5e1; border-radius: 10px; background: #fff; overflow: hidden; }
.ps-topo-layer-h { padding: 0.35rem 0.6rem; font-size: 0.7rem; font-weight: 650; color: #334155; background: #f1f5f9; border-bottom: 1px solid #e2e8f0; }
.ps-topo-nodes { display: flex; flex-wrap: wrap; gap: 0.45rem; padding: 0.55rem 0.6rem; }
.ps-topo-node { flex: 1 1 8rem; min-width: 7rem; max-width: 14rem; padding: 0.45rem 0.5rem; border-radius: 8px; border: 1px solid #e2e8f0; background: #fafbfc; font-size: 0.68rem; line-height: 1.35; }
.ps-topo-node-name { font-family: ui-monospace, Menlo, monospace; font-weight: 650; font-size: 0.7rem; color: #0f172a; word-break: break-all; }
.ps-topo-node.ready { border-color: #86efac; background: linear-gradient(180deg,#f0fdf4,#fff); }
.ps-topo-node.notready { border-color: #fdba74; background: linear-gradient(180deg,#fff7ed,#fff); }
.ps-topo-node-st { color: #64748b; margin-top: 0.15rem; }
.ps-topo-arrow { text-align: center; color: #94a3b8; font-size: 0.85rem; line-height: 1; user-select: none; }
.ps-topo-pill { display: inline-block; font-size: 0.62rem; font-weight: 700; padding: 0.1rem 0.35rem; border-radius: 999px; margin-left: 0.25rem; vertical-align: middle; }
.ps-topo-pill.gr { background: #dbeafe; color: #1d4ed8; }
.ps-topo-pill.async { background: #fef3c7; color: #b45309; }
</style>`)
	b.WriteString(`<div class="ps-topo-wrap">`)
	b.WriteString(`<p class="meta">Logical layout from the CR (<code>clusterType</code>, enabled components) and workload pods in the dump (<code>pods.yaml</code> when present). Arrows show typical client traffic flow.</p>`)
	for _, c := range clusters {
		ct := strings.TrimSpace(c.Spec.MySQL.ClusterType)
		if ct == "" {
			ct = "group-replication"
		}
		pillClass := "gr"
		if ct == "async" {
			pillClass = "async"
		}
		var metaParts []string
		metaParts = append(metaParts, fmt.Sprintf(`<span class="ps-topo-pill %s">%s</span>`, pillClass, esc(ct)))
		if psProxyEnabled(c.Spec.Proxy.HAProxy) {
			metaParts = append(metaParts, "HAProxy on")
		} else if psProxyEnabled(c.Spec.Proxy.Router) {
			metaParts = append(metaParts, "MySQL Router on")
		}
		if psOrchEnabled(c.Spec.Orchestrator) {
			metaParts = append(metaParts, "Orchestrator on")
		}
		if c.Spec.Backup != nil && c.Spec.Backup.PITR.Enabled {
			metaParts = append(metaParts, "PITR on")
		}
		b.WriteString(`<div class="ps-topo-cluster"><h4 class="ps-topo-h">`)
		b.WriteString(esc(c.Namespace + "/" + c.Name))
		b.WriteString(`</h4><p class="ps-topo-meta">`)
		b.WriteString(strings.Join(metaParts, " · "))
		if h := strings.TrimSpace(c.Status.Host); h != "" {
			b.WriteString(` · endpoint <code>` + esc(h) + `</code>`)
		}
		b.WriteString(`</p><div class="ps-topo-flow">`)

		layers := psTopologyLayers(c, pods, k8s)
		for i, layer := range layers {
			b.WriteString(`<div class="ps-topo-layer"><div class="ps-topo-layer-h">`)
			b.WriteString(esc(layer.title))
			b.WriteString(`</div><div class="ps-topo-nodes">`)
			for _, n := range layer.nodes {
				cls := "ps-topo-node"
				if n.ready {
					cls += " ready"
				} else if n.hasStatus {
					cls += " notready"
				}
				b.WriteString(`<div class="` + cls + `"><div class="ps-topo-node-name">` + esc(n.name))
				b.WriteString(`</div><div class="ps-topo-node-st">`)
				if n.detail != "" {
					b.WriteString(esc(n.detail))
				} else {
					b.WriteString("—")
				}
				b.WriteString(`</div></div>`)
			}
			if len(layer.nodes) == 0 {
				b.WriteString(`<span class="ps-topo-node-st">No matching pods in dump</span>`)
			}
			b.WriteString(`</div></div>`)
			if i < len(layers)-1 {
				b.WriteString(`<div class="ps-topo-arrow" aria-hidden="true">↓</div>`)
			}
		}
		b.WriteString(`</div></div>`)
	}
	b.WriteString(`</div>`)
	return b.String(), nil
}

type psTopoNode struct {
	name, detail string
	ready, hasStatus bool
}

type psTopoLayer struct {
	title string
	nodes []psTopoNode
}

func psTopologyLayers(c psClusterRecord, pods *jpreport.PodLoader, k8s map[string]jpreport.PodK8sMeta) []psTopoLayer {
	inst := c.Name
	ns := c.Namespace
	var layers []psTopoLayer

	if c.Spec.Backup != nil && c.Spec.Backup.PITR.Enabled {
		title := "Binlog server (PITR)"
		if c.Status.BinlogServer.Size > 0 {
			title += fmt.Sprintf(" · %d/%d ready", c.Status.BinlogServer.Ready, c.Status.BinlogServer.Size)
		}
		layers = append(layers, psTopoLayer{title: title, nodes: psPodsForComponent(pods, k8s, ns, inst, "binlog", "-binlog-")})
	}

	if psOrchEnabled(c.Spec.Orchestrator) {
		title := fmt.Sprintf("Orchestrator · spec size %d", c.Spec.Orchestrator.Size)
		if c.Status.Orchestrator.Size > 0 {
			title += fmt.Sprintf(" · %d/%d ready", c.Status.Orchestrator.Ready, c.Status.Orchestrator.Size)
		}
		layers = append(layers, psTopoLayer{title: title, nodes: psPodsForComponent(pods, k8s, ns, inst, "orchestrator", "-orc-")})
	}

	if psProxyEnabled(c.Spec.Proxy.HAProxy) {
		title := fmt.Sprintf("HAProxy · spec size %d", c.Spec.Proxy.HAProxy.Size)
		if c.Status.HAProxy.Size > 0 {
			title += fmt.Sprintf(" · %d/%d ready", c.Status.HAProxy.Ready, c.Status.HAProxy.Size)
		}
		layers = append(layers, psTopoLayer{title: title, nodes: psPodsForComponent(pods, k8s, ns, inst, "proxy", "-haproxy-")})
	} else if psProxyEnabled(c.Spec.Proxy.Router) {
		title := fmt.Sprintf("MySQL Router · spec size %d", c.Spec.Proxy.Router.Size)
		if c.Status.Router.Size > 0 {
			title += fmt.Sprintf(" · %d/%d ready", c.Status.Router.Ready, c.Status.Router.Size)
		}
		layers = append(layers, psTopoLayer{title: title, nodes: psPodsForComponent(pods, k8s, ns, inst, "router", "-router-")})
	}

	mysqlTitle := fmt.Sprintf("MySQL · %s · spec size %d", strings.TrimSpace(c.Spec.MySQL.ClusterType), c.Spec.MySQL.Size)
	if c.Status.MySQL.Size > 0 {
		mysqlTitle += fmt.Sprintf(" · %d/%d ready", c.Status.MySQL.Ready, c.Status.MySQL.Size)
	}
	layers = append(layers, psTopoLayer{title: mysqlTitle, nodes: psPodsForComponent(pods, k8s, ns, inst, "database", "-mysql-")})

	return layers
}

func psPodsForComponent(pods *jpreport.PodLoader, k8s map[string]jpreport.PodK8sMeta, ns, instance, component, nameFrag string) []psTopoNode {
	if pods == nil {
		return nil
	}
	names := pods.PSPodNamesForInstance(ns, instance, component, nameFrag)
	var out []psTopoNode
	for _, name := range names {
		n := psTopoNode{name: name}
		key := ns + "\x00" + name
		if k8s != nil {
			if m, ok := k8s[key]; ok {
				n.hasStatus = true
				n.ready = strings.HasPrefix(m.Ready, "1/") || m.Ready == "True" || strings.EqualFold(m.Status, "Running")
				parts := []string{m.Status}
				if m.Ready != "" && m.Ready != "—" {
					parts = append(parts, "ready "+m.Ready)
				}
				if m.IP != "" && m.IP != "—" {
					parts = append(parts, m.IP)
				}
				if m.Node != "" && m.Node != "—" {
					parts = append(parts, "node "+m.Node)
				}
				n.detail = strings.Join(parts, " · ")
			}
		}
		out = append(out, n)
	}
	return out
}
