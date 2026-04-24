package main

import (
	"flag"
	"fmt"
	htempl "html/template"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"

	"pt-k8s-summary/internal/collector"
	"pt-k8s-summary/internal/dumpctx"
	"pt-k8s-summary/internal/jpreport"

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
	certifiedImages := flag.Bool("certified-images", true, "fetch Percona certified image list (by spec.crVersion) and compare with images from pods.yaml (uses network)")
	flag.Usage = usage
	flag.Parse()

	args := flag.Args()
	if len(args) != 1 || !looksLikeClusterArchive(args[0]) {
		if *dumpPath == "" {
			usage()
			os.Exit(2)
		}
	}

	if err := runMain(args, *dumpPath, *nodesPath, *outPath, *galeraSince, *certifiedImages); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func runMain(args []string, dumpFlag, nodesFlag, outFlag, galeraSince string, certifiedImages bool) error {
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

	podsData, err := jpreport.LoadPodLoader(dumpAbs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pods: %v (PXC pod/backup details may be incomplete)\n", err)
		podsData = &jpreport.PodLoader{}
	}
	var podLogHTML htempl.HTML
	var hasPodLogs bool
	if h, err := collector.GatherPodLogsForReportHTML(dumpAbs, proc.GaleraSince, podsData, now); err != nil {
		fmt.Fprintf(os.Stderr, "pod logs: %v\n", err)
	} else if h != "" {
		podLogHTML = htempl.HTML(h)
		hasPodLogs = true
	}
	certCache := jpreport.NewCertifiedImageCache(certifiedImages)
	pxcRows, pxcFileCount, err := jpreport.LoadPXCRowsFromDump(dumpAbs, now, podsData, certCache)
	if err != nil {
		return fmt.Errorf("pxc resources: %w", err)
	}
	pxcAbsPaths, err := jpreport.ListPXCYAMLFiles(dumpAbs)
	if err != nil {
		return fmt.Errorf("pxc yaml list: %w", err)
	}
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
	backupRows, backupFileCount, err := jpreport.LoadBackupRowsFromDump(dumpAbs, now, podsData)
	if err != nil {
		return fmt.Errorf("pxc backups: %w", err)
	}
	backupMeta := ""
	if len(backupRows) > 0 {
		backupMeta = fmt.Sprintf("Found %d backup(s) in %d YAML file(s).", len(backupRows), backupFileCount)
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
		PXCRows         []jpreport.PXCRowTmpl
		PXCEmpty        bool
		PXCMeta         string
		PXCMainColspan  int
		ShowHAProxyCol  bool
		ShowProxySQLCol bool
		BackupEmpty     bool
		BackupMeta      string
		BackupRows      []jpreport.BackupRowTmpl
		ExtraSections   []collector.Section
		HasPodLogs      bool
		PodLogsHTML     htempl.HTML
	}{
		Proc:            proc,
		GeneratedAt:     now.UTC().Format(time.RFC3339),
		NodeCount:       len(nodeRows),
		Nodes:           toTmplNodes(nodeRows),
		PXCRows:         pxcRows,
		PXCEmpty:        len(pxcRows) == 0,
		PXCMeta:         pxcMeta,
		PXCMainColspan:  8,
		ShowHAProxyCol:  showHAProxyCol,
		ShowProxySQLCol: showProxySQLCol,
		BackupEmpty:     len(backupRows) == 0,
		BackupMeta:      backupMeta,
		BackupRows:      backupRows,
		ExtraSections:   extraSections,
		HasPodLogs:      hasPodLogs,
		PodLogsHTML:     podLogHTML,
	}

	if err := tmpl.Execute(outF, execData); err != nil {
		return fmt.Errorf("render: %w", err)
	}

	fmt.Printf("Wrote %s (%d nodes, %d PXC cluster(s) from %d file(s), %d backup(s) from %d file(s))\n", out, len(nodeRows), len(pxcRows), pxcFileCount, len(backupRows), backupFileCount)
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
