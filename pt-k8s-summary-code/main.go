package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"

	"pt-k8s-summary/internal/collector"
	"pt-k8s-summary/internal/dumpctx"

	"gopkg.in/yaml.v3"
)

// YAML mirrors the subset of v1 Node / List needed for the report.

type nodeListDoc struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Items      []node `yaml:"items"`
}

type node struct {
	Metadata nodeMetadata `yaml:"metadata"`
	Status   nodeStatus   `yaml:"status"`
}

type nodeMetadata struct {
	Name        string            `yaml:"name"`
	Labels      map[string]string `yaml:"labels"`
	Annotations map[string]string `yaml:"annotations"`
}

type nodeStatus struct {
	Addresses  []nodeAddress     `yaml:"addresses"`
	Capacity   map[string]string `yaml:"capacity"`
	Conditions []nodeCondition   `yaml:"conditions"`
	NodeInfo   nodeInfo          `yaml:"nodeInfo"`
}

type nodeAddress struct {
	Type    string `yaml:"type"`
	Address string `yaml:"address"`
}

type nodeCondition struct {
	Type               string `yaml:"type"`
	Status             string `yaml:"status"`
	LastTransitionTime string `yaml:"lastTransitionTime"`
}

type nodeInfo struct {
	OperatingSystem string `yaml:"operatingSystem"`
	OSImage         string `yaml:"osImage"`
	KernelVersion   string `yaml:"kernelVersion"`
	KubeletVersion  string `yaml:"kubeletVersion"`
}

type conditionRow struct {
	Status            string
	StatusTrue        bool
	SinceTransition   string
	HighlightPressure bool
}

type nodeRow struct {
	Hostname         string
	OS               string
	Kernel           string
	KubeletVersion   string
	Role             string
	InstanceType     string
	IP               string
	CPUCap           string
	EphemeralGiB     string
	Memory           string
	Pods             string
	Ready            conditionRow
	PIDPressure      conditionRow
	DiskPressure     conditionRow
	MemoryPressure   conditionRow
	RowPressureClass string
}

type condCellTmpl struct {
	Status          string
	SinceTransition string
	StatusClass     string
	DurationClass   string
}

type nodeRowTmpl struct {
	Hostname         string
	Role             string
	IP               string
	InstanceType     string
	OS               string
	Kernel           string
	KubeletVersion   string
	CPUCap           string
	EphemeralGiB     string
	Memory           string
	Pods             string
	Ready            condCellTmpl
	PIDPressure      condCellTmpl
	DiskPressure     condCellTmpl
	MemoryPressure   condCellTmpl
	RowPressureClass string
}

// processingReport captures inputs and artifacts used to build the HTML report.
type processingReport struct {
	SourceArchive    string
	DumpRootDisplay  string
	NodesPathDisplay string
	PXCYAMLRelPaths  []string
	CollectorErrors  string
	HasErrorsFile    bool
	// GaleraSince is the pt-galera-log-explainer --since= value (RFC3339), or "".
	GaleraSince string
}

const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Kubernetes &amp; Percona XtraDB Cluster</title>
<style>
:root { font-family: system-ui, sans-serif; color: #1a1a1a; }
body { margin: 2rem; line-height: 1.45; }
h1 { font-size: 1.35rem; }
h2 { font-size: 1.1rem; margin-top: 2rem; }
table { border-collapse: collapse; width: 100%; font-size: 0.9rem; }
th, td { border: 1px solid #ccc; padding: 0.45rem 0.55rem; text-align: left; vertical-align: top; }
th { background: #f4f4f4; }
tr.pressure-alert td { background: #ffe5e5; }
.status-true { color: #0a6b0a; font-weight: 600; }
.status-false { color: #555; }
.status-bad { color: #a40000; font-weight: 600; }
.status-muted { color: #666; }
.meta { color: #444; font-size: 0.85rem; margin-bottom: 1.5rem; }
.sub { font-size: 0.85rem; color: #333; margin-top: 0.2rem; }
.sub-ok { color: #0a6b0a; }
.sub-bad { color: #a40000; }
tr.pxc-comp-title td { background: #eaeaea; font-size: 0.9rem; vertical-align: middle; }
tr.pxc-sub td { background: #fafafa; font-size: 0.88rem; vertical-align: top; }
tr.pxc-sub strong { color: #333; }
.proc-list { margin: 0.25rem 0 0 1.1rem; }
.proc-list li { margin: 0.2rem 0; }
.collector-err-teaser { margin: 0.75rem 0 1rem 0; padding: 0.65rem 0.85rem; background: #fafafa; border: 1px solid #e5e5e5; border-radius: 8px; font-size: 0.88rem; color: #444; }
.collector-err-teaser-txt { color: #555; }
.collector-err-btn { margin-right: 0.35rem; font-size: 0.82rem; font-weight: 650; padding: 0.32rem 0.65rem; border-radius: 8px; border: 1px solid #b91c1c; background: #fef2f2; color: #991b1b; cursor: pointer; vertical-align: middle; }
.collector-err-btn:hover { background: #fee2e2; }
.collector-err-stash { display: none !important; }
#collector-err-modal { position: fixed; inset: 0; z-index: 10000; display: flex; align-items: center; justify-content: center; background: rgba(15, 23, 42, 0.45); opacity: 0; pointer-events: none; transition: opacity 0.12s; }
#collector-err-modal[aria-hidden="false"] { opacity: 1; pointer-events: auto; }
#collector-err-modal .collector-err-dlg { background: #fff; color: #1a1a1a; max-width: min(96vw, 52rem); max-height: 88vh; width: 100%; display: flex; flex-direction: column; border-radius: 12px; box-shadow: 0 20px 50px rgba(0,0,0,0.22); overflow: hidden; border: 1px solid #e5e5e5; }
#collector-err-modal .collector-err-dlg-h { display: flex; align-items: center; justify-content: space-between; gap: 0.75rem; padding: 0.55rem 0.85rem; background: #fef2f2; border-bottom: 1px solid #fecaca; }
#collector-err-modal .collector-err-dlg-t { font-size: 1rem; font-weight: 650; margin: 0; line-height: 1.3; }
#collector-err-modal .collector-err-dlg-x { display: block; width: 2.25rem; height: 2.25rem; line-height: 1; border: none; background: transparent; color: #64748b; font-size: 1.5rem; cursor: pointer; border-radius: 8px; }
#collector-err-modal .collector-err-dlg-x:hover { background: #fee2e2; color: #991b1b; }
#collector-err-modal .collector-err-dlg-body { margin: 0; padding: 0.85rem; overflow: auto; max-height: calc(88vh - 3.2rem); font-size: 0.8rem; line-height: 1.45; font-family: ui-monospace, Menlo, Consolas, monospace; white-space: pre-wrap; word-break: break-word; background: #fff5f5; border: none; color: #1a1a1a; }
.report-section { margin-top: 2rem; padding-top: 1rem; border-top: 1px solid #ddd; }
.report-section h3 { font-size: 1.05rem; margin-bottom: 0.5rem; }
.nodes-coll { margin: 0.5rem 0 1.5rem 0; border: 1px solid #d4cfc4; border-radius: 12px; overflow: hidden; background: linear-gradient(180deg, #faf9f7 0%, #fff 48%); box-shadow: 0 2px 14px rgba(30, 27, 75, 0.07); }
.nodes-coll[open] { box-shadow: 0 6px 24px rgba(30, 27, 75, 0.1); border-color: #c9c2b4; }
.nodes-coll-sum { display: flex; align-items: center; gap: 0.85rem; padding: 0.7rem 1rem 0.75rem; cursor: pointer; user-select: none; list-style: none; background: linear-gradient(105deg, rgba(49, 46, 129, 0.06) 0%, rgba(255,255,255,0) 55%); transition: background 0.15s; }
.nodes-coll-sum::-webkit-details-marker { display: none; }
.nodes-coll-sum:hover { background: linear-gradient(105deg, rgba(49, 46, 129, 0.1) 0%, rgba(255,255,255,0) 55%); }
.nodes-coll[open] > .nodes-coll-sum { border-bottom: 1px solid #e8e4dc; }
.nodes-coll-exp { flex: 0 0 auto; width: 2.05rem; height: 2.05rem; display: inline-flex; align-items: center; justify-content: center; border-radius: 9px; border: 2px solid #c9a227; background: linear-gradient(155deg, #1e1b4b 0%, #312e81 42%, #4338ca 100%); color: #fef3c7; font-size: 1.35rem; font-weight: 200; line-height: 1; box-shadow: 0 3px 10px rgba(30, 27, 75, 0.35), inset 0 1px 0 rgba(255,255,255,0.15); transition: border-color 0.15s, box-shadow 0.15s, transform 0.12s; }
.nodes-coll-exp::before { content: "+"; display: block; margin-top: -0.08rem; }
.nodes-coll[open] .nodes-coll-exp::before { content: "\2212"; font-size: 1.2rem; margin-top: -0.12rem; }
.nodes-coll-sum:hover .nodes-coll-exp { border-color: #e8c547; box-shadow: 0 4px 16px rgba(30, 27, 75, 0.4), inset 0 1px 0 rgba(255,255,255,0.18); }
.nodes-coll-sum:active .nodes-coll-exp { transform: scale(0.96); }
.nodes-coll-sum-body { display: flex; flex-direction: column; align-items: flex-start; gap: 0.12rem; min-width: 0; }
.nodes-coll-sum-h { font-size: 1rem; font-weight: 650; color: #1e1b4b; letter-spacing: 0.02em; }
.nodes-coll-sum-meta { font-size: 0.78rem; color: #64748b; font-weight: 500; }
.nodes-coll-inner { padding: 0.85rem 1rem 1.1rem; overflow-x: auto; }
</style>
</head>
<body>
<h1>Cluster summary</h1>
<h2>Processing details</h2>
<p class="meta">Everything below was derived from the following inputs (typical layout from <code>pt-k8s-debug-collector</code>).</p>
<ul class="proc-list">
<li><strong>Source</strong>: {{ .Proc.SourceArchive }}</li>
<li><strong>Dump root</strong>: <code>{{ .Proc.DumpRootDisplay }}</code></li>
<li><strong>Nodes manifest</strong>: <code>{{ .Proc.NodesPathDisplay }}</code></li>
<li><strong><code>perconaxtradbclusters.pxc.percona.com.yaml</code> files</strong> ({{ len .Proc.PXCYAMLRelPaths }}):{{ if .Proc.PXCYAMLRelPaths }}
<ul class="proc-list">{{ range .Proc.PXCYAMLRelPaths }}<li><code>{{ . }}</code></li>{{ end }}</ul>{{ else }} <em>none found under the dump root</em>{{ end }}</li>
{{ if .Proc.HasErrorsFile }}
<li><strong>Collector <code>errors.txt</code></strong>: present{{ if .Proc.CollectorErrors }} — use <strong>View errors</strong> under Processing details to read the excerpt{{ end }}</li>
{{ else }}
<li><strong>Collector <code>errors.txt</code></strong>: not present at dump root</li>
{{ end }}
{{ if .Proc.GaleraSince }}
<li><strong><code>pt-galera-log-explainer</code> <code>--since</code></strong>: <code>{{ .Proc.GaleraSince }}</code> (only log lines on or after this time in the member <code>mysqld-error.log</code> files)</li>
{{ end }}
</ul>
{{ if .Proc.CollectorErrors }}
<p class="collector-err-teaser"><button type="button" class="collector-err-btn" id="collector-err-open">View errors</button> <span class="collector-err-teaser-txt">Collector <code>errors.txt</code> excerpt (may be truncated for this report).</span></p>
<pre id="collector-err-stash" class="collector-err-stash">{{ .Proc.CollectorErrors }}</pre>
<div id="collector-err-modal" aria-hidden="true" role="dialog" aria-modal="true" aria-labelledby="collector-err-title">
<div class="collector-err-dlg" role="document" tabindex="-1">
<div class="collector-err-dlg-h"><h3 class="collector-err-dlg-t" id="collector-err-title">Collector <code>errors.txt</code></h3><button type="button" class="collector-err-dlg-x" data-collector-err-x="" aria-label="Close">×</button></div>
<pre class="collector-err-dlg-body" id="collector-err-body" tabindex="0" aria-live="polite" aria-labelledby="collector-err-title"></pre>
</div></div>
<script>(function(){
  var mod=document.getElementById("collector-err-modal");
  var btn=document.getElementById("collector-err-open");
  var stash=document.getElementById("collector-err-stash");
  var body=document.getElementById("collector-err-body");
  if(!mod||!btn||!stash||!body)return;
  function openE(){
    body.textContent=stash.textContent;
    mod.setAttribute("aria-hidden","false");
    body.focus();
  }
  function closeE(){
    mod.setAttribute("aria-hidden","true");
    body.textContent="";
  }
  btn.addEventListener("click", openE);
  mod.querySelectorAll("[data-collector-err-x]").forEach(function(el){ el.addEventListener("click", closeE); });
  mod.addEventListener("click", function(ev){ if(ev.target===mod) closeE(); });
  document.addEventListener("keydown", function(ev){
    if(ev.key==="Escape" && mod.getAttribute("aria-hidden")==="false") closeE();
  });
})();</script>
{{ end }}

<h1>Kubernetes cluster — nodes</h1>
<p class="meta">Report generated at <strong>{{ .GeneratedAt }}</strong> · Node count: <strong>{{ .NodeCount }}</strong></p>

<h2>Nodes</h2>
<details class="nodes-coll">
<summary class="nodes-coll-sum" aria-label="Expand or collapse the Kubernetes nodes table"><span class="nodes-coll-exp" aria-hidden="true"></span><span class="nodes-coll-sum-body"><strong class="nodes-coll-sum-h">Node inventory</strong><span class="nodes-coll-sum-meta">{{ .NodeCount }} {{ if eq .NodeCount 1 }}node{{ else }}nodes{{ end }} · roles, capacity &amp; pressure</span></span></summary>
<div class="nodes-coll-inner">
<table>
<thead>
<tr>
<th>Hostname</th>
<th>Role</th>
<th>IP</th>
<th>Instance type</th>
<th>OS</th>
<th>Kernel</th>
<th>Kubelet</th>
<th>CPU (capacity)</th>
<th>Ephemeral storage (GiB)</th>
<th>Memory (capacity)</th>
<th>Pods (capacity)</th>
<th>Ready</th>
<th>PID pressure</th>
<th>Disk pressure</th>
<th>Memory pressure</th>
</tr>
</thead>
<tbody>
{{ range .Nodes }}
<tr class="{{ .RowPressureClass }}">
<td>{{ .Hostname }}</td>
<td>{{ .Role }}</td>
<td>{{ .IP }}</td>
<td>{{ .InstanceType }}</td>
<td>{{ .OS }}</td>
<td>{{ .Kernel }}</td>
<td>{{ .KubeletVersion }}</td>
<td>{{ .CPUCap }}</td>
<td>{{ .EphemeralGiB }}</td>
<td>{{ .Memory }}</td>
<td>{{ .Pods }}</td>
<td>{{ template "condCell" .Ready }}</td>
<td>{{ template "condCell" .PIDPressure }}</td>
<td>{{ template "condCell" .DiskPressure }}</td>
<td>{{ template "condCell" .MemoryPressure }}</td>
</tr>
{{ end }}
</tbody>
</table>
</div>
</details>

<h2>Percona XtraDB clusters (operator CR)</h2>
{{ if .PXCEmpty }}
<p class="meta">No <code>PerconaXtraDBCluster</code> resources under <code>{{ .Proc.DumpRootDisplay }}</code> (searched recursively for <code>perconaxtradbclusters.pxc.percona.com.yaml</code>).</p>
{{ else }}
<p class="meta">{{ .PXCMeta }} Dump root: <code>{{ .Proc.DumpRootDisplay }}</code></p>
<table>
<thead>
<tr>
<th>Cluster</th>
<th>Namespace</th>
<th>CR version</th>
<th>Created</th>
<th>Latest condition</th>
<th>PMM enabled</th>
</tr>
</thead>
<tbody>
{{ range .PXCRows }}
<tr>
<td>{{ .Name }}</td>
<td>{{ .Namespace }}</td>
<td>{{ .CRVersion }}</td>
<td>{{ .Created }}</td>
<td><strong>{{ .CondType }}</strong> / {{ .CondStatus }}<br><span class="sub sub-ok">Since {{ .CondSince }}</span></td>
<td>{{ .PMMEnabled }}</td>
</tr>
{{ if $.ShowHAProxyCol }}
<tr class="pxc-comp-title"><td colspan="6"><strong>HAProxy</strong></td></tr>
<tr class="pxc-sub"><td colspan="6">{{ .HAProxyCell }}</td></tr>
{{ end }}
{{ if $.ShowProxySQLCol }}
<tr class="pxc-comp-title"><td colspan="6"><strong>ProxySQL</strong></td></tr>
<tr class="pxc-sub"><td colspan="6">{{ .ProxySQLCell }}</td></tr>
{{ end }}
<tr class="pxc-comp-title"><td colspan="6"><strong>PXC</strong></td></tr>
<tr class="pxc-sub"><td colspan="6">{{ .PXCCell }}</td></tr>
{{ end }}
</tbody>
</table>
{{ end }}
{{ if .ExtraSections }}
<h2>Additional analysis</h2>
<p class="meta">Sections below are contributed by pluggable collectors in <code>internal/collector/</code> (see <code>contrib_owner.go</code> / <code>contrib_partner.go</code>).</p>
{{ range .ExtraSections }}
<section class="report-section" id="{{ .ID }}">
{{ if .Title }}<h3>{{ .Title }}</h3>{{ end }}
{{ .HTML }}
</section>
{{ end }}
{{ end }}
</body>
</html>

{{ define "condCell" }}
<div><span class="{{ .StatusClass }}">{{ .Status }}</span></div>
<div class="sub {{ .DurationClass }}">Since {{ .SinceTransition }}</div>
{{ end }}
`

var quantityRe = regexp.MustCompile(`^([0-9]*\.?[0-9]+)([A-Za-z]*)$`)

const collectorErrorsMax = 256 * 1024

// pullKnownFlags moves -dump, -nodes, and -out (with their values) before any
// positional archive path so flag.Parse accepts either:
//
//	pt-k8s-summary -out report.html dump.tar.gz
//	pt-k8s-summary dump.tar.gz -out report.html
func pullKnownFlags(argv []string) []string {
	known := map[string]struct{}{
		"-dump":         {},
		"-nodes":        {},
		"-out":          {},
		"-galera-since": {},
	}
	var pulled, rest []string
	for i := 0; i < len(argv); {
		a := argv[i]
		if _, ok := known[a]; ok && i+1 < len(argv) {
			pulled = append(pulled, a, argv[i+1])
			i += 2
			continue
		}
		rest = append(rest, a)
		i++
	}
	return append(pulled, rest...)
}

// normalizeGaleraSince returns "" for empty input, or a UTC RFC3339Nano time string for pt-galera-log-explainer --since=.
func normalizeGaleraSince(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC().Format(time.RFC3339Nano), nil
		}
	}
	return "", fmt.Errorf("galera-since: invalid time %q (use RFC3339, e.g. 2023-01-05T03:24:26Z or 2023-01-05T03:24:26.000000Z)", s)
}

func usage() {
	fmt.Fprintf(os.Stderr, `Usage:
  pt-k8s-summary [flags] <cluster-dump.tar.gz|.tgz>
  pt-k8s-summary [flags] -dump <extracted-cluster-dump-dir>
  e.g. pt-galera timeline: -galera-since 2023-01-05T03:24:26.000000Z

Reads a tarball produced by pt-k8s-debug-collector (or an extracted dump tree),
parses nodes.yaml and Percona XtraDB Cluster API lists, and writes an HTML report.

Flags:
`)
	flag.PrintDefaults()
}

func main() {
	if len(os.Args) > 1 {
		os.Args = append([]string{os.Args[0]}, pullKnownFlags(os.Args[1:])...)
	}
	dumpPath := flag.String("dump", "", "path to extracted cluster dump root (recursive PXC search; default nodes: <dump>/nodes.yaml)")
	nodesPath := flag.String("nodes", "", "path to nodes.yaml (default: <dump>/nodes.yaml when -dump is set)")
	outPath := flag.String("out", "", "output HTML path (default: reports/<archive-stem>-summary.html for archives, else reports/report.html)")
	galeraSince := flag.String("galera-since", "", "if set, pass to pt-galera-log-explainer --since= (RFC3339 / RFC3339Nano, e.g. 2023-01-05T03:24:26.000000Z) to only include events on or after that instant")
	flag.Usage = usage
	flag.Parse()

	args := flag.Args()
	if len(args) != 1 || !looksLikeClusterArchive(args[0]) {
		if *dumpPath == "" {
			usage()
			os.Exit(2)
		}
	}

	if err := runMain(args, *dumpPath, *nodesPath, *outPath, *galeraSince); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func runMain(args []string, dumpFlag, nodesFlag, outFlag, galeraSince string) error {
	gs, err := normalizeGaleraSince(galeraSince)
	if err != nil {
		return err
	}
	galeraSince = gs
	var extractTmp string
	defer func() {
		if extractTmp != "" {
			_ = os.RemoveAll(extractTmp)
		}
	}()

	var (
		dumpAbs  string
		nodesAbs string
		out      string
		proc     processingReport
	)

	switch {
	case len(args) == 1 && looksLikeClusterArchive(args[0]):
		archiveAbs, err := filepath.Abs(args[0])
		if err != nil {
			return fmt.Errorf("archive path: %w", err)
		}
		tmpDir, err := os.MkdirTemp("", "pt-k8s-summary-*")
		if err != nil {
			return fmt.Errorf("temp dir: %w", err)
		}
		extractTmp = tmpDir
		root, err := extractClusterArchive(archiveAbs, tmpDir)
		if err != nil {
			return fmt.Errorf("extract archive: %w", err)
		}
		dumpAbs = root
		proc.SourceArchive = archiveAbs
		base := filepath.Base(dumpAbs)
		proc.DumpRootDisplay = base + " (top-level directory from the archive)"
		proc.NodesPathDisplay = filepath.ToSlash(filepath.Join(base, "nodes.yaml"))
		if outFlag != "" {
			out = outFlag
		} else {
			out = defaultReportNameFromArchive(archiveAbs)
		}
	case dumpFlag != "":
		var err error
		dumpAbs, err = filepath.Abs(dumpFlag)
		if err != nil {
			return fmt.Errorf("dump path: %w", err)
		}
		proc.SourceArchive = "(directory mode: " + dumpAbs + ")"
		proc.DumpRootDisplay = dumpAbs
		proc.NodesPathDisplay = ""
		if outFlag != "" {
			out = outFlag
		} else {
			out = filepath.Join("reports", "report.html")
		}
	default:
		return fmt.Errorf("invalid arguments (expected an archive or -dump)")
	}

	if nodesFlag != "" {
		var err error
		nodesAbs, err = filepath.Abs(nodesFlag)
		if err != nil {
			return fmt.Errorf("nodes path: %w", err)
		}
	} else {
		nodesAbs = filepath.Join(dumpAbs, "nodes.yaml")
	}
	if proc.NodesPathDisplay == "" {
		proc.NodesPathDisplay = nodesAbs
	}

	data, err := os.ReadFile(nodesAbs)
	if err != nil {
		return fmt.Errorf("read nodes file %q: %w", nodesAbs, err)
	}

	var list nodeListDoc
	if err := yaml.Unmarshal(data, &list); err != nil {
		return fmt.Errorf("parse nodes yaml: %w", err)
	}

	now := time.Now()
	proc.GaleraSince = galeraSince
	dumpCtx := dumpctx.New(dumpAbs, now).WithGaleraSince(galeraSince)

	nodeRows := make([]nodeRow, 0, len(list.Items))
	for i := range list.Items {
		nodeRows = append(nodeRows, buildNodeRow(&list.Items[i], now))
	}

	if b, err := dumpCtx.ReadRel("errors.txt"); err == nil {
		proc.HasErrorsFile = true
		proc.CollectorErrors = string(truncateRunes(b, collectorErrorsMax))
	}

	pxcRows, pxcAbsPaths, err := loadPXCRowsFromDump(dumpAbs, now)
	if err != nil {
		return fmt.Errorf("pxc resources: %w", err)
	}
	pxcFileCount := len(pxcAbsPaths)
	proc.PXCYAMLRelPaths = make([]string, 0, len(pxcAbsPaths))
	for _, p := range pxcAbsPaths {
		rel, err := filepath.Rel(dumpAbs, p)
		if err != nil {
			rel = p
		}
		proc.PXCYAMLRelPaths = append(proc.PXCYAMLRelPaths, filepath.ToSlash(rel))
	}
	pxcMeta := fmt.Sprintf("Found %d cluster(s) in %d YAML file(s).", len(pxcRows), pxcFileCount)
	showHAProxyCol := false
	showProxySQLCol := false
	for _, r := range pxcRows {
		if r.HAProxyEnabled {
			showHAProxyCol = true
		}
		if r.ProxySQLEnabled {
			showProxySQLCol = true
		}
	}

	extraSections := collector.GatherSections(dumpCtx)

	tmpl, err := template.New("report").Parse(htmlTemplate)
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	if dir := filepath.Dir(out); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create output directory: %w", err)
		}
	}

	outF, err := os.Create(out)
	if err != nil {
		return fmt.Errorf("create output: %w", err)
	}
	defer outF.Close()

	execData := struct {
		Proc            processingReport
		GeneratedAt     string
		NodeCount       int
		Nodes           []nodeRowTmpl
		PXCRows         []pxcRowTmpl
		PXCEmpty        bool
		PXCMeta         string
		ShowHAProxyCol  bool
		ShowProxySQLCol bool
		ExtraSections   []collector.Section
	}{
		Proc:            proc,
		GeneratedAt:     now.UTC().Format(time.RFC3339),
		NodeCount:       len(nodeRows),
		Nodes:           toTmplNodes(nodeRows),
		PXCRows:         pxcRows,
		PXCEmpty:        len(pxcRows) == 0,
		PXCMeta:         pxcMeta,
		ShowHAProxyCol:  showHAProxyCol,
		ShowProxySQLCol: showProxySQLCol,
		ExtraSections:   extraSections,
	}

	if err := tmpl.Execute(outF, execData); err != nil {
		return fmt.Errorf("render: %w", err)
	}

	fmt.Printf("Wrote %s (%d nodes, %d PXC cluster(s) from %d file(s))\n", out, len(nodeRows), len(pxcRows), pxcFileCount)
	return nil
}

func defaultReportNameFromArchive(archiveAbs string) string {
	base := filepath.Base(archiveAbs)
	lower := strings.ToLower(base)
	stem := base
	switch {
	case strings.HasSuffix(lower, ".tar.gz"):
		stem = base[:len(base)-len(".tar.gz")]
	case strings.HasSuffix(lower, ".tgz"):
		stem = base[:len(base)-len(".tgz")]
	}
	stem = strings.TrimSpace(stem)
	if stem == "" || stem == "." {
		stem = "cluster-dump"
	}
	return filepath.Join("reports", stem+"-summary.html")
}

func truncateRunes(b []byte, max int) []byte {
	if max <= 0 || len(b) <= max {
		return b
	}
	s := string(b)
	if len(s) <= max {
		return b
	}
	// Trim by bytes is OK for an excerpt box; avoid importing unicode/utf8 for a soft cap.
	return []byte(s[:max] + "\n… (truncated)\n")
}

func toTmplNodes(rows []nodeRow) []nodeRowTmpl {
	out := make([]nodeRowTmpl, 0, len(rows))
	for _, w := range rows {
		out = append(out, nodeRowTmpl{
			Hostname:         w.Hostname,
			Role:             w.Role,
			IP:               w.IP,
			InstanceType:     w.InstanceType,
			OS:               w.OS,
			Kernel:           w.Kernel,
			KubeletVersion:   w.KubeletVersion,
			CPUCap:           w.CPUCap,
			EphemeralGiB:     w.EphemeralGiB,
			Memory:           w.Memory,
			Pods:             w.Pods,
			Ready:            condToTmpl(w.Ready, false),
			PIDPressure:      condToTmpl(w.PIDPressure, true),
			DiskPressure:     condToTmpl(w.DiskPressure, true),
			MemoryPressure:   condToTmpl(w.MemoryPressure, true),
			RowPressureClass: w.RowPressureClass,
		})
	}
	return out
}

func condToTmpl(c conditionRow, isPressure bool) condCellTmpl {
	var statusClass, durationClass string
	if isPressure {
		if c.StatusTrue {
			statusClass = "status-bad"
			durationClass = "sub-bad"
		} else if c.Status == "False" {
			statusClass = "status-true"
			durationClass = "sub-ok"
		} else {
			statusClass = "status-muted"
			durationClass = "sub"
		}
	} else {
		if c.Status == "True" {
			statusClass = "status-true"
			durationClass = "sub-ok"
		} else {
			statusClass = "status-bad"
			durationClass = "sub-bad"
		}
	}
	return condCellTmpl{
		Status:          c.Status,
		SinceTransition: c.SinceTransition,
		StatusClass:     statusClass,
		DurationClass:   durationClass,
	}
}

func k8sRole(n *node) string {
	if n == nil {
		return ""
	}
	l := n.Metadata.Labels
	if l == nil {
		return "worker"
	}
	if _, ok := l["node-role.kubernetes.io/control-plane"]; ok {
		return "control-plane"
	}
	if _, ok := l["node-role.kubernetes.io/master"]; ok {
		return "control-plane"
	}
	return "worker"
}

func nodeIP(n *node) string {
	var internal, external, hostname string
	for _, a := range n.Status.Addresses {
		switch a.Type {
		case "InternalIP":
			internal = a.Address
		case "ExternalIP":
			external = a.Address
		case "Hostname":
			hostname = a.Address
		}
	}
	if internal != "" {
		return internal
	}
	if external != "" {
		return external
	}
	if n.Metadata.Annotations != nil {
		if v := n.Metadata.Annotations["alpha.kubernetes.io/provided-node-ip"]; v != "" {
			return v
		}
	}
	return hostname
}

func instanceType(n *node) string {
	if n.Metadata.Labels == nil {
		return ""
	}
	if v := n.Metadata.Labels["node.kubernetes.io/instance-type"]; v != "" {
		return v
	}
	return n.Metadata.Labels["beta.kubernetes.io/instance-type"]
}

// quantityToBytes parses a Kubernetes-style quantity (e.g. "48", "3300421268Ki") into bytes as float64.
func quantityToBytes(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	m := quantityRe.FindStringSubmatch(s)
	if m == nil {
		return 0, false
	}
	val, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, false
	}
	suf := m[2]
	if suf == "" {
		return val, true
	}
	mult := 1.0
	switch suf {
	case "n":
		mult = 1e-9
	case "u":
		mult = 1e-6
	case "m":
		mult = 1e-3
	case "":
		mult = 1
	case "k":
		mult = 1e3
	case "M":
		mult = 1e6
	case "G":
		mult = 1e9
	case "T":
		mult = 1e12
	case "P":
		mult = 1e15
	case "E":
		mult = 1e18
	case "Ki":
		mult = 1024
	case "Mi":
		mult = 1024 * 1024
	case "Gi":
		mult = 1024 * 1024 * 1024
	case "Ti":
		mult = 1024 * 1024 * 1024 * 1024
	case "Pi":
		mult = 1024 * 1024 * 1024 * 1024 * 1024
	case "Ei":
		mult = 1024 * 1024 * 1024 * 1024 * 1024 * 1024
	default:
		return 0, false
	}
	return val * mult, true
}

func quantityToGiBString(s string) string {
	b, ok := quantityToBytes(s)
	if !ok || b <= 0 {
		return s
	}
	return fmt.Sprintf("%.2f", b/(1024*1024*1024))
}

func quantityMemoryDisplay(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	b, ok := quantityToBytes(s)
	if !ok {
		return s
	}
	if b >= 1024*1024*1024 {
		return fmt.Sprintf("%.2f GiB", b/(1024*1024*1024))
	}
	if b >= 1024*1024 {
		return fmt.Sprintf("%.2f MiB", b/(1024*1024))
	}
	if b >= 1024 {
		return fmt.Sprintf("%.2f KiB", b/1024)
	}
	return fmt.Sprintf("%.0f B", b)
}

func findCondition(conditions []nodeCondition, typ string) *nodeCondition {
	for i := range conditions {
		if conditions[i].Type == typ {
			return &conditions[i]
		}
	}
	return nil
}

func conditionRowFrom(cond *nodeCondition, now time.Time, pressure bool) conditionRow {
	if cond == nil {
		return conditionRow{
			Status:          "Unknown",
			SinceTransition: "n/a",
		}
	}
	st := strings.TrimSpace(cond.Status)
	since := "n/a"
	if cond.LastTransitionTime != "" {
		if t, err := time.Parse(time.RFC3339, cond.LastTransitionTime); err == nil {
			since = humanizeDurationInState(t, now)
		}
	}
	isTrue := st == "True"
	return conditionRow{
		Status:            st,
		StatusTrue:        isTrue,
		SinceTransition:   since,
		HighlightPressure: pressure && isTrue,
	}
}

// humanizeDurationInState returns how long the node has been in the current condition
// (wall time since lastTransitionTime), without a trailing "ago" (used as a duration phrase).
func humanizeDurationInState(t, now time.Time) string {
	if !now.After(t) {
		return "0s"
	}
	d := now.Sub(t)
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.0fm", d.Minutes())
	}
	if d < 24*time.Hour {
		h := int(d.Hours())
		m := int(d.Minutes()) % 60
		return fmt.Sprintf("%dh %dm", h, m)
	}
	days := int(d.Hours() / 24)
	h := int(d.Hours()) % 24
	return fmt.Sprintf("%dd %dh", days, h)
}

func buildNodeRow(n *node, now time.Time) nodeRow {
	ni := n.Status.NodeInfo
	osStr := ni.OperatingSystem
	if ni.OSImage != "" {
		if osStr != "" {
			osStr = ni.OSImage + " (" + osStr + ")"
		} else {
			osStr = ni.OSImage
		}
	}

	ready := findCondition(n.Status.Conditions, "Ready")
	pid := findCondition(n.Status.Conditions, "PIDPressure")
	disk := findCondition(n.Status.Conditions, "DiskPressure")
	mem := findCondition(n.Status.Conditions, "MemoryPressure")

	rReady := conditionRowFrom(ready, now, false)
	rPID := conditionRowFrom(pid, now, true)
	rDisk := conditionRowFrom(disk, now, true)
	rMem := conditionRowFrom(mem, now, true)

	rowClass := ""
	if rPID.HighlightPressure || rDisk.HighlightPressure || rMem.HighlightPressure {
		rowClass = "pressure-alert"
	}

	capacity := n.Status.Capacity
	cpu := strings.TrimSpace(capacity["cpu"])
	ephem := strings.TrimSpace(capacity["ephemeral-storage"])
	ephemGiB := quantityToGiBString(ephem)
	memQty := quantityMemoryDisplay(capacity["memory"])
	pods := strings.TrimSpace(capacity["pods"])

	host := n.Metadata.Name
	if n.Metadata.Labels != nil {
		if h := n.Metadata.Labels["kubernetes.io/hostname"]; h != "" {
			host = h
		}
	}

	return nodeRow{
		Hostname:         host,
		OS:               osStr,
		Kernel:           ni.KernelVersion,
		KubeletVersion:   ni.KubeletVersion,
		Role:             k8sRole(n),
		InstanceType:     instanceType(n),
		IP:               nodeIP(n),
		CPUCap:           cpu,
		EphemeralGiB:     ephemGiB,
		Memory:           memQty,
		Pods:             pods,
		Ready:            rReady,
		PIDPressure:      rPID,
		DiskPressure:     rDisk,
		MemoryPressure:   rMem,
		RowPressureClass: rowClass,
	}
}
