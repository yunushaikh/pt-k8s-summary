package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"

	"gopkg.in/yaml.v3"
)

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
	Addresses  []nodeAddress `yaml:"addresses"`
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
	OSImage          string `yaml:"osImage"`
	KernelVersion    string `yaml:"kernelVersion"`
	KubeletVersion   string `yaml:"kubeletVersion"`
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

type reportPage struct {
	GeneratedAt      string
	NodeCount        int
	Nodes            []nodeRowTmpl
	PXCEmpty         bool
	PXCMeta          string
	DumpRoot         string
	PXCRows          []pxcRowTmpl
	ShowHAProxyCol   bool
	ShowProxySQLCol  bool
	PXCMainColspan   int
	BackupEmpty      bool
	BackupMeta       string
	BackupRows       []backupRowTmpl
}

const pxcTableColspan = 8

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
.pressure-false { color: #0a6b0a; font-weight: 600; }
.pressure-true { color: #a40000; font-weight: 600; }
.meta { color: #444; font-size: 0.85rem; margin-bottom: 1.5rem; }
.sub { font-size: 0.85rem; color: #333; margin-top: 0.2rem; }
.sub-ok { color: #0a6b0a; }
.sub-bad { color: #a40000; }
.unsafe-flags-ok { color: #0a6b0a; font-weight: 600; }
.unsafe-flags-bad { color: #a40000; font-weight: 600; }
h3.pxc-subsection-title { font-size: 1.1rem; margin: 1.25rem 0 0.35rem 0; font-weight: 600; }
h3.pxc-subsection-title:first-child { margin-top: 0; }
td.pxc-comp-wrap { vertical-align: top; padding: 0.5rem 0.55rem 0.85rem; }
table.pxc-inner-table { border-collapse: collapse; width: 100%; font-size: 0.88rem; margin-bottom: 0.15rem; }
table.pxc-inner-table th, table.pxc-inner-table td { border: 1px solid #ccc; padding: 0.35rem 0.5rem; text-align: left; }
table.pxc-inner-table th { background: #f4f4f4; }
h4.pxc-pod-subtitle { font-size: 0.95rem; margin: 0.75rem 0 0.25rem 0; font-weight: 600; }
.pxc-pod-log-cell { white-space: nowrap; vertical-align: middle; }
.pxc-pod-name-cell { vertical-align: middle; }
.pod-spec-store { display: none; }
.pod-log-store { display: none; }
.pod-log-btn { font-size: 0.82rem; cursor: pointer; }
.pod-name-btn { font: inherit; color: #0645ad; background: none; border: none; padding: 0; margin: 0; cursor: pointer; text-align: left; text-decoration: underline; text-underline-offset: 2px; }
.pod-name-btn:hover { color: #0b0080; }
.pxc-cfg-cell { max-width: 28rem; vertical-align: top; }
.pxc-cfg-snippet { white-space: pre-wrap; margin: 0 0 0.35rem 0; font-size: 0.8rem; max-height: calc(1.35em * 5); overflow: hidden; font-family: ui-monospace, monospace; }
.pxc-cfg-btn { font-size: 0.82rem; cursor: pointer; margin-top: 0.15rem; }
.pxc-cfg-full-store { display: none; }
.pxc-cfg-dialog { max-width: min(92vw, 56rem); width: 100%; border: 1px solid #999; padding: 0; }
.pxc-cfg-dialog::backdrop { background: rgba(0,0,0,0.35); }
.pxc-cfg-dialog-inner { padding: 1rem; }
.pxc-cfg-dialog-title { margin: 0 0 0.35rem 0; font-size: 0.95rem; }
.pxc-cfg-dialog-pre { white-space: pre-wrap; max-height: 70vh; overflow: auto; margin: 0.5rem 0 0.75rem; font-size: 0.82rem; border: 1px solid #ddd; padding: 0.5rem; background: #fafafa; font-family: ui-monospace, monospace; }
</style>
</head>
<body>
<h1>Kubernetes Cluster</h1>
<p class="meta">Report generated at <strong>{{ .GeneratedAt }}</strong> · Node count: <strong>{{ .NodeCount }}</strong></p>

<h2>Nodes</h2>
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
<th>CPU</th>
<th>Ephemeral storage (GiB)</th>
<th>Memory (GiB)</th>
<th>Pods Capacity</th>
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
{{ template "condCell" .Ready }}
{{ template "condCell" .PIDPressure }}
{{ template "condCell" .DiskPressure }}
{{ template "condCell" .MemoryPressure }}
</tr>
{{ end }}
</tbody>
</table>

<h2>Percona XtraDB Cluster</h2>
{{ if .PXCEmpty }}
<p class="meta">No <code>PerconaXtraDBCluster</code> resources under <code>{{ .DumpRoot }}</code> (searched recursively for <code>perconaxtradbclusters.pxc.percona.com.yaml</code>).</p>
{{ else }}
<p class="meta">{{ .PXCMeta }} Dump root: <code>{{ .DumpRoot }}</code></p>
<table>
<thead>
<tr>
<th>Cluster</th>
<th>Namespace</th>
<th>CR version</th>
<th>Created</th>
<th>Ready</th>
<th>PMM enabled</th>
<th>Unsafe Flags</th>
<th>Update Strategy</th>
</tr>
</thead>
<tbody>
{{ range .PXCRows }}
<tr>
<td>{{ .Name }}</td>
<td>{{ .Namespace }}</td>
<td>{{ .CRVersion }}</td>
<td>{{ .Created }}</td>
<td><span class="{{ .ReadyStatusClass }}">{{ .ReadyStatus }}</span><br><span class="sub sub-ok">Since {{ .ReadySince }}</span></td>
<td>{{ .PMMEnabled }}</td>
<td>{{ if .UnsafeFlagsOK }}<span class="unsafe-flags-ok">NO</span>{{ else }}<span class="unsafe-flags-bad">{{ .UnsafeFlagsEscaped }}</span>{{ end }}</td>
<td>{{ .UpdateStrategy }}</td>
</tr>
<tr><td colspan="{{ $.PXCMainColspan }}" class="pxc-comp-wrap">
<h3 class="pxc-subsection-title">Container images vs certified images</h3>
<p class="meta">Compared using <code>spec.crVersion</code> <strong>{{ .CRVersion }}</strong> against the <a href="{{ .CertifiedDocURL }}" target="_blank" rel="noopener noreferrer">Percona certified images</a> list for that operator release.{{ if .CertifiedFetchErrEscaped }}<br><span class="status-bad">{{ .CertifiedFetchErrEscaped }}</span>{{ end }}</p>
<table class="pxc-inner-table">
<thead><tr><th>Image (from <code>pods.yaml</code>)</th><th>Matches certified list</th></tr></thead>
<tbody>
{{ range .ImageCertRows }}<tr><td><code>{{ .ImageEscaped }}</code></td><td>{{ if .IsCertified }}<span class="unsafe-flags-ok">yes</span>{{ else }}<span class="unsafe-flags-bad">no</span>{{ end }}</td></tr>
{{ else }}<tr><td colspan="2" class="status-muted">No container images found for this cluster in <code>pods.yaml</code> (expected labels <code>app.kubernetes.io/instance</code> and component <code>haproxy</code>, <code>proxysql</code>, or <code>pxc</code>).</td></tr>{{ end }}
</tbody>
</table>
</td></tr>
{{ if $.ShowHAProxyCol }}
<tr><td colspan="{{ $.PXCMainColspan }}" class="pxc-comp-wrap">
<h3 class="pxc-subsection-title">HAProxy</h3>
<table class="pxc-inner-table">
<thead><tr><th>Size</th><th>Status</th><th>Version</th></tr></thead>
<tbody><tr><td>{{ .HAProxySize }}</td><td>{{ .HAProxyStatus }}</td><td>{{ .HAProxyVersion }}</td></tr></tbody>
</table>
<h4 class="pxc-pod-subtitle">Pods</h4>
<table class="pxc-inner-table">
<thead><tr><th>Name</th><th>Ready</th><th>Status</th><th>Restarts</th><th>Age</th><th>IP</th><th>Node</th><th>Logs</th></tr></thead>
<tbody>
{{ range .HAProxyPods }}<tr><td class="pxc-pod-name-cell"><textarea readonly hidden class="pod-spec-store" id="{{ .PodSpecModalID }}-full">{{ .PodSpecEscaped }}</textarea><button type="button" class="pod-name-btn" data-target="{{ .PodSpecModalID }}" data-pod-name="{{ .Name }}">{{ .Name }}</button></td><td>{{ .Ready }}</td><td>{{ .Status }}</td><td>{{ .Restarts }}</td><td>{{ .Age }}</td><td>{{ .PodIP }}</td><td>{{ .Node }}</td><td class="pxc-pod-log-cell">{{ if .HasPodLog }}<textarea readonly hidden class="pod-log-store" id="{{ .PodLogModalID }}-full">{{ .PodLogEscaped }}</textarea><button type="button" class="pod-log-btn" data-target="{{ .PodLogModalID }}" data-pod-name="{{ .Name }}">View logs</button>{{ else }}<span class="status-muted">—</span>{{ end }}</td></tr>
{{ else }}<tr><td colspan="8">No matching pods in <code>pods.yaml</code> for this HAProxy workload.</td></tr>{{ end }}
</tbody>
</table>
</td></tr>
{{ end }}
{{ if $.ShowProxySQLCol }}
<tr><td colspan="{{ $.PXCMainColspan }}" class="pxc-comp-wrap">
<h3 class="pxc-subsection-title">ProxySQL</h3>
<table class="pxc-inner-table">
<thead><tr><th>Size</th><th>Status</th><th>Version</th></tr></thead>
<tbody><tr><td>{{ .ProxySQLSize }}</td><td>{{ .ProxySQLStatus }}</td><td>{{ .ProxySQLVersion }}</td></tr></tbody>
</table>
<h4 class="pxc-pod-subtitle">Pods</h4>
<table class="pxc-inner-table">
<thead><tr><th>Name</th><th>Ready</th><th>Status</th><th>Restarts</th><th>Age</th><th>IP</th><th>Node</th><th>Logs</th></tr></thead>
<tbody>
{{ range .ProxySQLPods }}<tr><td class="pxc-pod-name-cell"><textarea readonly hidden class="pod-spec-store" id="{{ .PodSpecModalID }}-full">{{ .PodSpecEscaped }}</textarea><button type="button" class="pod-name-btn" data-target="{{ .PodSpecModalID }}" data-pod-name="{{ .Name }}">{{ .Name }}</button></td><td>{{ .Ready }}</td><td>{{ .Status }}</td><td>{{ .Restarts }}</td><td>{{ .Age }}</td><td>{{ .PodIP }}</td><td>{{ .Node }}</td><td class="pxc-pod-log-cell">{{ if .HasPodLog }}<textarea readonly hidden class="pod-log-store" id="{{ .PodLogModalID }}-full">{{ .PodLogEscaped }}</textarea><button type="button" class="pod-log-btn" data-target="{{ .PodLogModalID }}" data-pod-name="{{ .Name }}">View logs</button>{{ else }}<span class="status-muted">—</span>{{ end }}</td></tr>
{{ else }}<tr><td colspan="8">No matching pods in <code>pods.yaml</code> for this ProxySQL workload.</td></tr>{{ end }}
</tbody>
</table>
</td></tr>
{{ end }}
<tr><td colspan="{{ $.PXCMainColspan }}" class="pxc-comp-wrap">
<h3 class="pxc-subsection-title">PXC</h3>
<table class="pxc-inner-table">
<thead><tr><th>Size</th><th>Status</th><th>Version</th><th>MySQL <code>configuration</code></th></tr></thead>
<tbody><tr><td>{{ .PXCSize }}</td><td>{{ .PXCStatus }}</td><td>{{ .PXCVersion }}</td><td class="pxc-cfg-cell"><pre class="pxc-cfg-snippet">{{ .PXCConfigSnippet }}</pre>{{ if .PXCConfigTruncated }}<textarea readonly hidden class="pxc-cfg-full-store" id="{{ .PXCConfigModalID }}-full">{{ .PXCConfigFullEscaped }}</textarea><div><button type="button" class="pxc-cfg-btn" data-target="{{ .PXCConfigModalID }}">Show full configuration</button></div>{{ end }}</td></tr></tbody>
</table>
<h4 class="pxc-pod-subtitle">Pods</h4>
<table class="pxc-inner-table">
<thead><tr><th>Name</th><th>Ready</th><th>Status</th><th>Restarts</th><th>Age</th><th>IP</th><th>Node</th><th>Logs</th></tr></thead>
<tbody>
{{ range .PXCPods }}<tr><td class="pxc-pod-name-cell"><textarea readonly hidden class="pod-spec-store" id="{{ .PodSpecModalID }}-full">{{ .PodSpecEscaped }}</textarea><button type="button" class="pod-name-btn" data-target="{{ .PodSpecModalID }}" data-pod-name="{{ .Name }}">{{ .Name }}</button></td><td>{{ .Ready }}</td><td>{{ .Status }}</td><td>{{ .Restarts }}</td><td>{{ .Age }}</td><td>{{ .PodIP }}</td><td>{{ .Node }}</td><td class="pxc-pod-log-cell">{{ if .HasPodLog }}<textarea readonly hidden class="pod-log-store" id="{{ .PodLogModalID }}-full">{{ .PodLogEscaped }}</textarea><button type="button" class="pod-log-btn" data-target="{{ .PodLogModalID }}" data-pod-name="{{ .Name }}">View logs</button>{{ else }}<span class="status-muted">—</span>{{ end }}</td></tr>
{{ else }}<tr><td colspan="8">No matching pods in <code>pods.yaml</code> for this PXC workload.</td></tr>{{ end }}
</tbody>
</table>
</td></tr>
{{ end }}
</tbody>
</table>
{{ end }}

<h2>Percona XtraDB Cluster backups</h2>
{{ if .BackupEmpty }}
<p class="meta">No <code>PerconaXtraDBClusterBackup</code> resources under <code>{{ .DumpRoot }}</code> (searched recursively for <code>perconaxtradbclusterbackups.pxc.percona.com.yaml</code>).</p>
{{ else }}
<p class="meta">{{ .BackupMeta }}</p>
<table>
<thead>
<tr>
<th>Name</th>
<th>Cluster</th>
<th>Storage</th>
<th>Destination</th>
<th>Status</th>
<th>Age</th>
<th>Logs</th>
</tr>
</thead>
<tbody>
{{ range .BackupRows }}
<tr>
<td class="pxc-pod-name-cell"><textarea readonly hidden class="backup-manifest-store" id="{{ .BackupManifestModalID }}-full">{{ .BackupManifestEscaped }}</textarea><button type="button" class="pod-name-btn backup-manifest-btn" data-target="{{ .BackupManifestModalID }}" data-backup-display="{{ .Namespace }}/{{ .Name }}">{{ .Name }}</button></td>
<td>{{ .Cluster }}</td>
<td>{{ .Storage }}</td>
<td>{{ .Destination }}</td>
<td>{{ .Status }}</td>
<td>{{ .Age }}</td>
<td class="pxc-pod-log-cell">{{ if .HasPodLog }}<textarea readonly hidden class="pod-log-store" id="{{ .PodLogModalID }}-full">{{ .PodLogEscaped }}</textarea><button type="button" class="pod-log-btn" data-target="{{ .PodLogModalID }}" data-pod-name="{{ .LogPodName }}">View logs</button>{{ else }}<span class="status-muted">—</span>{{ end }}</td>
</tr>
{{ end }}
</tbody>
</table>
{{ end }}

<dialog id="pxc-cfg-dialog" class="pxc-cfg-dialog">
<div class="pxc-cfg-dialog-inner">
<p class="pxc-cfg-dialog-title"><strong>MySQL configuration</strong> (<code>spec.pxc.configuration</code>)</p>
<pre id="pxc-cfg-dialog-pre" class="pxc-cfg-dialog-pre"></pre>
<button type="button" class="pxc-cfg-dialog-close">Close</button>
</div>
</dialog>
<dialog id="pod-log-dialog" class="pxc-cfg-dialog">
<div class="pxc-cfg-dialog-inner">
<p class="pxc-cfg-dialog-title">Pod logs: <strong id="pod-log-dialog-title"></strong></p>
<p class="meta">From dump path <code>&lt;namespace&gt;/&lt;pod&gt;/logs.txt</code> (or <code>log</code>).</p>
<pre id="pod-log-dialog-pre" class="pxc-cfg-dialog-pre"></pre>
<button type="button" class="pod-log-dialog-close">Close</button>
</div>
</dialog>
<dialog id="pod-spec-dialog" class="pxc-cfg-dialog">
<div class="pxc-cfg-dialog-inner">
<p class="pxc-cfg-dialog-title">Containers: <strong id="pod-spec-dialog-title"></strong></p>
<p class="meta">From <code>spec.initContainers</code> and <code>spec.containers</code> in <code>pods.yaml</code> (image and resources per container).</p>
<pre id="pod-spec-dialog-pre" class="pxc-cfg-dialog-pre"></pre>
<button type="button" class="pod-spec-dialog-close">Close</button>
</div>
</dialog>
<dialog id="backup-manifest-dialog" class="pxc-cfg-dialog">
<div class="pxc-cfg-dialog-inner">
<p class="pxc-cfg-dialog-title"><code>PerconaXtraDBClusterBackup</code>: <strong id="backup-manifest-dialog-title"></strong></p>
<p class="meta">YAML for this resource from <code>perconaxtradbclusterbackups.pxc.percona.com.yaml</code> in the dump (one document).</p>
<pre id="backup-manifest-dialog-pre" class="pxc-cfg-dialog-pre"></pre>
<button type="button" class="backup-manifest-dialog-close">Close</button>
</div>
</dialog>
<script>
(function() {
  var cfgDlg = document.getElementById('pxc-cfg-dialog');
  var cfgPre = document.getElementById('pxc-cfg-dialog-pre');
  var logDlg = document.getElementById('pod-log-dialog');
  var logPre = document.getElementById('pod-log-dialog-pre');
  var logTitle = document.getElementById('pod-log-dialog-title');
  var specDlg = document.getElementById('pod-spec-dialog');
  var specPre = document.getElementById('pod-spec-dialog-pre');
  var specTitle = document.getElementById('pod-spec-dialog-title');
  var backupDlg = document.getElementById('backup-manifest-dialog');
  var backupPre = document.getElementById('backup-manifest-dialog-pre');
  var backupTitle = document.getElementById('backup-manifest-dialog-title');
  document.body.addEventListener('click', function(ev) {
    var t = ev.target;
    if (!t || !t.classList) return;
    if (t.classList.contains('pxc-cfg-btn')) {
      if (!cfgDlg || !cfgPre) return;
      var id = t.getAttribute('data-target');
      var src = id ? document.getElementById(id + '-full') : null;
      if (src) { cfgPre.textContent = src.value; cfgDlg.showModal(); }
      return;
    }
    if (t.classList.contains('pxc-cfg-dialog-close')) {
      if (cfgDlg) cfgDlg.close();
      return;
    }
    if (t.classList.contains('pod-log-btn')) {
      if (!logDlg || !logPre) return;
      var lid = t.getAttribute('data-target');
      var lsrc = lid ? document.getElementById(lid + '-full') : null;
      if (lsrc) {
        if (logTitle) logTitle.textContent = t.getAttribute('data-pod-name') || '';
        logPre.textContent = lsrc.value;
        logDlg.showModal();
      }
      return;
    }
    if (t.classList.contains('backup-manifest-btn')) {
      if (!backupDlg || !backupPre) return;
      var bid = t.getAttribute('data-target');
      var bsrc = bid ? document.getElementById(bid + '-full') : null;
      if (bsrc) {
        if (backupTitle) backupTitle.textContent = t.getAttribute('data-backup-display') || '';
        backupPre.textContent = bsrc.value;
        backupDlg.showModal();
      }
      return;
    }
    if (t.classList.contains('pod-name-btn')) {
      if (!specDlg || !specPre) return;
      var sid = t.getAttribute('data-target');
      var ssrc = sid ? document.getElementById(sid + '-full') : null;
      if (ssrc) {
        if (specTitle) specTitle.textContent = t.getAttribute('data-pod-name') || '';
        specPre.textContent = ssrc.value;
        specDlg.showModal();
      }
      return;
    }
    if (t.classList.contains('pod-log-dialog-close')) {
      if (logDlg) logDlg.close();
      return;
    }
    if (t.classList.contains('pod-spec-dialog-close')) {
      if (specDlg) specDlg.close();
      return;
    }
    if (t.classList.contains('backup-manifest-dialog-close')) {
      if (backupDlg) backupDlg.close();
    }
  });
  if (cfgDlg) {
    cfgDlg.addEventListener('click', function(ev) {
      if (ev.target === cfgDlg) cfgDlg.close();
    });
  }
  if (logDlg) {
    logDlg.addEventListener('click', function(ev) {
      if (ev.target === logDlg) logDlg.close();
    });
  }
  if (specDlg) {
    specDlg.addEventListener('click', function(ev) {
      if (ev.target === specDlg) specDlg.close();
    });
  }
  if (backupDlg) {
    backupDlg.addEventListener('click', function(ev) {
      if (ev.target === backupDlg) backupDlg.close();
    });
  }
})();
</script>
</body>
</html>
`

func main() {
	nodesPath := flag.String("nodes", "", "path to nodes.yaml (Node or Node list)")
	dumpRoot := flag.String("dump", ".", "cluster dump root (searched for PXC CRs and pods.yaml)")
	outPath := flag.String("out", "report.html", "output HTML path")
	certImages := flag.Bool("certified-images", true, "fetch Percona certified image list from docs.percona.com (by spec.crVersion) and compare with pod images from pods.yaml")
	flag.Parse()

	now := time.Now().UTC()
	dumpAbs, err := filepath.Abs(*dumpRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dump path: %v\n", err)
		os.Exit(1)
	}

	var nodeRows []nodeRowTmpl
	if strings.TrimSpace(*nodesPath) != "" {
		nodeRows, err = loadNodesFromYAML(*nodesPath, now)
		if err != nil {
			fmt.Fprintf(os.Stderr, "nodes: %v\n", err)
			os.Exit(1)
		}
	}

	podsData, err := loadPodLoader(dumpAbs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pods: %v (PXC pod tables will be empty)\n", err)
		podsData = &podLoader{}
	}

	certCache := newCertifiedImageCache(*certImages)
	pxcRows, pxcFileCount, err := loadPXCRowsFromDump(dumpAbs, now, podsData, certCache)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pxc: %v\n", err)
		os.Exit(1)
	}

	backupRows, backupFileCount, err := loadBackupRowsFromDump(dumpAbs, now, podsData)
	if err != nil {
		fmt.Fprintf(os.Stderr, "backups: %v\n", err)
		os.Exit(1)
	}

	showHX, showPS := false, false
	for i := range pxcRows {
		if pxcRows[i].HAProxyEnabled {
			showHX = true
		}
		if pxcRows[i].ProxySQLEnabled {
			showPS = true
		}
	}

	pxcMeta := ""
	if len(pxcRows) == 0 {
		pxcMeta = ""
	} else {
		pxcMeta = fmt.Sprintf("Found %d cluster(s) in %d YAML file(s).", len(pxcRows), pxcFileCount)
	}

	backupMeta := ""
	if len(backupRows) > 0 {
		backupMeta = fmt.Sprintf("Found %d backup(s) in %d YAML file(s). Dump root: %s", len(backupRows), backupFileCount, dumpAbs)
	}

	page := reportPage{
		GeneratedAt:     now.Format(time.RFC3339),
		NodeCount:       len(nodeRows),
		Nodes:           nodeRows,
		PXCEmpty:        len(pxcRows) == 0,
		PXCMeta:         pxcMeta,
		DumpRoot:        dumpAbs,
		PXCRows:         pxcRows,
		ShowHAProxyCol:  showHX,
		ShowProxySQLCol: showPS,
		PXCMainColspan:  pxcTableColspan,
		BackupEmpty:     len(backupRows) == 0,
		BackupMeta:      backupMeta,
		BackupRows:      backupRows,
	}

	tmpl, err := template.New("page").Funcs(template.FuncMap{}).Parse(htmlTemplate)
	if err != nil {
		fmt.Fprintf(os.Stderr, "template: %v\n", err)
		os.Exit(1)
	}
	_, err = tmpl.New("condCell").Parse(
		`<td><div><span class="{{ .StatusClass }}">{{ .Status }}</span></div><div class="sub {{ .DurationClass }}">Since {{ .SinceTransition }}</div></td>`)
	if err != nil {
		fmt.Fprintf(os.Stderr, "condCell: %v\n", err)
		os.Exit(1)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, page); err != nil {
		fmt.Fprintf(os.Stderr, "execute: %v\n", err)
		os.Exit(1)
	}
	if err := ioutil.WriteFile(*outPath, buf.Bytes(), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "write %s: %v\n", *outPath, err)
		os.Exit(1)
	}
	fmt.Printf("Wrote %s (%d nodes, %d PXC cluster(s) from %d file(s), %d backup(s) from %d file(s))\n", *outPath, len(nodeRows), len(pxcRows), pxcFileCount, len(backupRows), backupFileCount)
}

func loadNodesFromYAML(path string, now time.Time) ([]nodeRowTmpl, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var raw interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	var items []node
	switch doc := raw.(type) {
	case map[string]interface{}:
		if arr, ok := doc["items"].([]interface{}); ok {
			for _, it := range arr {
				b, _ := yaml.Marshal(it)
				var n node
				if yaml.Unmarshal(b, &n) == nil && strings.TrimSpace(n.Metadata.Name) != "" {
					items = append(items, n)
				}
			}
		}
	default:
		return nil, fmt.Errorf("unsupported nodes.yaml shape")
	}
	out := make([]nodeRowTmpl, 0, len(items))
	for i := range items {
		out = append(out, nodeToRowTmpl(&items[i], now))
	}
	return out, nil
}

func nodeToRowTmpl(n *node, now time.Time) nodeRowTmpl {
	host := strings.TrimSpace(n.Metadata.Name)
	if host == "" {
		host = "—"
	}
	role := "worker"
	if n.Metadata.Labels != nil {
		if _, ok := n.Metadata.Labels["node-role.kubernetes.io/control-plane"]; ok {
			role = "control-plane"
		} else if _, ok := n.Metadata.Labels["node-role.kubernetes.io/master"]; ok {
			role = "control-plane"
		}
	}
	ip := nodeInternalIP(n)
	inst := "—"
	if n.Metadata.Labels != nil {
		if v := strings.TrimSpace(n.Metadata.Labels["node.kubernetes.io/instance-type"]); v != "" {
			inst = v
		} else if v := strings.TrimSpace(n.Metadata.Labels["beta.kubernetes.io/instance-type"]); v != "" {
			inst = v
		}
	}
	osStr := strings.TrimSpace(n.Status.NodeInfo.OSImage)
	if osStr == "" {
		osStr = "—"
	}
	kernel := strings.TrimSpace(n.Status.NodeInfo.KernelVersion)
	if kernel == "" {
		kernel = "—"
	}
	kubelet := strings.TrimSpace(n.Status.NodeInfo.KubeletVersion)
	if kubelet == "" {
		kubelet = "—"
	}
	capacity := n.Status.Capacity
	cpu := strings.TrimSpace(capacity["cpu"])
	if cpu == "" {
		cpu = "—"
	}
	mem := formatMemoryQuantity(capacity["memory"])
	if mem == "" {
		mem = "—"
	}
	ephem := ephemeralStorageGiB(capacity["ephemeral-storage"])
	if ephem == "" {
		ephem = "—"
	}
	pods := strings.TrimSpace(capacity["pods"])
	if pods == "" {
		pods = "—"
	}

	ready := nodeConditionCell(n, "Ready", now)
	pid := nodeConditionCell(n, "PIDPressure", now)
	disk := nodeConditionCell(n, "DiskPressure", now)
	memP := nodeConditionCell(n, "MemoryPressure", now)

	rowClass := ""
	if memP.HighlightPressure || disk.HighlightPressure || pid.HighlightPressure {
		rowClass = "pressure-alert"
	}
	return nodeRowTmpl{
		Hostname:         host,
		Role:             role,
		IP:               ip,
		InstanceType:     inst,
		OS:               osStr,
		Kernel:           kernel,
		KubeletVersion:   kubelet,
		CPUCap:           cpu,
		EphemeralGiB:     ephem,
		Memory:           mem,
		Pods:             pods,
		Ready:            condCellFromConditionRow(ready, false),
		PIDPressure:      condCellFromConditionRow(pid, true),
		DiskPressure:     condCellFromConditionRow(disk, true),
		MemoryPressure:   condCellFromConditionRow(memP, true),
		RowPressureClass: rowClass,
	}
}

type conditionRow struct {
	Status            string
	StatusTrue        bool
	SinceTransition   string
	HighlightPressure bool
}

func condCellFromConditionRow(c conditionRow, pressureInverted bool) condCellTmpl {
	st := strings.TrimSpace(c.Status)
	if st == "" {
		st = "—"
	}
	statusClass := "status-muted"
	display := st
	if pressureInverted {
		if strings.EqualFold(st, "false") {
			display = "False"
			statusClass = "pressure-false"
		} else if strings.EqualFold(st, "true") {
			display = "True"
			statusClass = "pressure-true"
		}
	} else {
		if strings.EqualFold(st, "true") {
			statusClass = "status-true"
		} else if strings.EqualFold(st, "false") {
			statusClass = "status-false"
		}
	}
	durClass := "sub-ok"
	if c.HighlightPressure {
		durClass = "sub-bad"
	}
	return condCellTmpl{
		Status:          display,
		SinceTransition: c.SinceTransition,
		StatusClass:     statusClass,
		DurationClass:   durClass,
	}
}

func nodeConditionCell(n *node, condType string, now time.Time) conditionRow {
	out := conditionRow{Status: "—", SinceTransition: "—"}
	for _, c := range n.Status.Conditions {
		if strings.TrimSpace(c.Type) != condType {
			continue
		}
		st := strings.TrimSpace(c.Status)
		out.Status = st
		out.StatusTrue = strings.EqualFold(st, "true")
		if c.LastTransitionTime != "" {
			if t, err := time.Parse(time.RFC3339, strings.TrimSpace(c.LastTransitionTime)); err == nil {
				out.SinceTransition = humanizeDurationInState(t, now)
			} else {
				out.SinceTransition = strings.TrimSpace(c.LastTransitionTime)
			}
		}
		if condType != "Ready" && out.StatusTrue {
			out.HighlightPressure = true
		}
		break
	}
	return out
}

func nodeInternalIP(n *node) string {
	for _, a := range n.Status.Addresses {
		if strings.TrimSpace(a.Type) == "InternalIP" && strings.TrimSpace(a.Address) != "" {
			return strings.TrimSpace(a.Address)
		}
	}
	return "—"
}

func formatMemoryQuantity(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	bytesVal, ok := kubernetesQuantityToBytes(s)
	if !ok {
		return s
	}
	gib := float64(bytesVal) / (1024 * 1024 * 1024)
	return fmt.Sprintf("%.0f", math.Round(gib))
}

func ephemeralStorageGiB(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	bytesVal, ok := kubernetesQuantityToBytes(s)
	if !ok {
		return s
	}
	gib := float64(bytesVal) / (1024 * 1024 * 1024)
	return fmt.Sprintf("%.0f", math.Round(gib))
}

var quantitySuffix = regexp.MustCompile(`^([0-9]+)(Ki|Mi|Gi|Ti)$`)

func kubernetesQuantityToBytes(s string) (int64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	if m := quantitySuffix.FindStringSubmatch(s); m != nil {
		n, err := strconv.ParseInt(m[1], 10, 64)
		if err != nil {
			return 0, false
		}
		switch m[2] {
		case "Ki":
			return n * 1024, true
		case "Mi":
			return n * 1024 * 1024, true
		case "Gi":
			return n * 1024 * 1024 * 1024, true
		case "Ti":
			return n * 1024 * 1024 * 1024 * 1024, true
		}
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return n, true
	}
	return 0, false
}
